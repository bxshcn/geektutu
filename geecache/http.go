package geecache

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"geektutu/geecache/consistenthash"

	pb "geektutu/geecache/geecachepb"

	"google.golang.org/protobuf/proto"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// http服务，用socket外加路径表示，HTTPPool表示为指定路径（http://socket/_geecache/）提供的http服务。
type HTTPPool struct {
	self     string
	basePath string

	mu sync.Mutex
	// peers记录所有的服务节点，这是所有HTTPPool共享的基础数据，因此用指针。
	peers *consistenthash.Map
	// 每个物理节点，都有可能访问其他所有物理节点，因此根据节点名称建立好映射。
	// 每个节点名称使用http://ip:port表示
	httpGetters map[string]*httpGetter
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (p *HTTPPool) Log(format string, data ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, data...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)

	// 默认请求path为defaultBasePath/<group>/<key>
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid request path", http.StatusBadRequest)
		return
	}

	groupname, key := parts[0], parts[1]

	group := GetGroup(groupname)
	if group == nil {
		http.Error(w, "invalid group", http.StatusNotFound)
		return
	}
	bv, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNoContent)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: bv.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	// bv.b 与bv.ByteSlice()有什么区别？
	// w.Write(bv.ByteSlice())
	w.Write(body)
}

// 初始化一致性hash环。参数peer为物理节点的url地址，比如http://IP:port
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 初始化一致性hash环
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 向其中添加物理节点名称
	p.peers.Add(peers...)

	// 然后初始化p.httpGetters
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		// p.basePath默认为/_geecache/
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// 根据key找到适当的peerGetter。就是key->
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	peer := p.peers.Get(key)
	if peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	// 如果是自己，则返回nil, false
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

// 用于从其他物理节点获取数据，这个被隐藏在物理节点中，物理节点对外就是HTTPPool的ServeHTTP。
//　为什么我们将其命名为HTTPPool呢？就是因为这个缓存是以HTTP Pool的形式提供的：
// 每个服务节点代表整个pool对外提供数据缓存服务。
type httpGetter struct {
	baseURL string
}

/*func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.PathEscape(group),
		url.PathEscape(key),
	)
	res, err := http.Get(u)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}
	return bytes, nil
} */

func (h *httpGetter) Get(req *pb.Request, resp *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.PathEscape(req.Group),
		url.PathEscape(req.Key),
	)
	res, err := http.Get(u)

	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	if err = proto.Unmarshal(bytes, resp); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

var _ PeerGetter = (*httpGetter)(nil)
