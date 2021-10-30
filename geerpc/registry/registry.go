package registry

/*
Registry是服务注册和发现功能的服务端，而Discovery为对应的客户端
Registry提供最基本的服务注册和发现功能：通过http协议的POST注册rpc server，通过GET获取所有服务列表。
Registry注册的是rpc server的address，每个rpc server上的service需要register：
就是当你创建一个rpc server时，需要注册其提供的服务。
*/

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type GeeRegistry struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem // map[addr]*ServerItem
}

type ServerItem struct {
	Addr  string
	start time.Time
}

const (
	defaultPath    = "/_geerpc/registry"
	defaultTimeout = 5 * time.Minute

	geerpcHeader = "X-Geerpc-Server"
)

func NewGeeRegistry(timeout time.Duration) *GeeRegistry {
	return &GeeRegistry{
		timeout: timeout,
		servers: make(map[string]*ServerItem)}
}

var DefaultGeeRegistry = NewGeeRegistry(defaultTimeout)

func (r *GeeRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if server, ok := r.servers[addr]; ok {
		server.start = time.Now()
	}
	r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
}

// 返回服务器地址列表，而不是内建的结构slice。
func (r *GeeRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var aliveSvrs []string
	for addr, server := range r.servers {
		if r.timeout == 0 || !time.Now().After(server.start.Add(r.timeout)) {
			aliveSvrs = append(aliveSvrs, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(aliveSvrs)
	return aliveSvrs
}

func (r *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 使用header：X-Geerpc-Servers来获取或者设置新的server
	switch req.Method {
	case "GET":
		log.Println("registry: GET")
		servers := strings.Join(r.aliveServers(), ",")
		log.Println("registry: send server list  back to client: ", servers)
		w.Header().Set(geerpcHeader, servers)
	// If WriteHeader is not called explicitly, the first call to Write
	// will trigger an implicit WriteHeader(http.StatusOK).
	// Thus explicit calls to WriteHeader are mainly used to
	// send error codes.
	//w.WriteHeader(http.StatusOK)
	case "POST":
		serverAddr := req.Header.Get(geerpcHeader)
		log.Println("registry: POST: got server: ", serverAddr)
		if serverAddr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.putServer(serverAddr)
		log.Println("registry: current server list", r.servers)
		//w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *GeeRegistry) HandleHTTP(path string) {
	http.Handle(path, r)
}

func HandleHTTP() {
	DefaultGeeRegistry.HandleHTTP(defaultPath)
}

func Heartbeat(registry, rpcAddr string, duration time.Duration) {
	// 如果给定的duration为0，则表示使用缺省的间隔时间
	if duration == 0 {
		//duration = defaultTimeout - time.Duration(1)*time.Minute
		duration = defaultTimeout - 1*time.Minute
	}

	// 如果我们将ticker放在这里创建，那么在go例程中引用这个ticker会不会有问题呢？
	// 理论上，在另外一个go routine中引用其他goroutine中的变量是没有问题的，只不过
	// 任意时刻引用的是ticker的当前值，如果希望使用调用go语句时刻的值，就需要使用参数
	// 带入到go启动的func中。
	ticker := time.NewTicker(duration)
	err := sendHeartbeat(registry, rpcAddr)
	//var wg sync.WaitGroup
	//wg.Add(1)
	go func() {
		//defer wg.Done()
		//ticker := time.NewTicker(duration)
		for err == nil {
			<-ticker.C
			err = sendHeartbeat(registry, rpcAddr)
		}
	}()
	//wg.Wait()
	//log.Println("registry: Heartbeat failed")
}

func sendHeartbeat(registry, rpcAddr string) error {
	log.Println(rpcAddr, " send heart beat to registry ", registry)
	// 就是使用http协议，将rpcAddr作为头，以post模式发送到register服务中。
	req, err := http.NewRequest("POST", registry, nil)
	if err != nil {
		return err
	}
	req.Header.Set(geerpcHeader, rpcAddr)
	// 我们确切的使用了适当的模式POST，以及正确的请求头，因此唯一的错误
	// 可能就是底层连接方面的问题，我们只需要考虑这种错误即可。
	//_, err = http.DefaultClient.Do(req) // 所有rpc server都使用相同的DefaultClient与registry通讯，是否有问题？
	client := &http.Client{}
	_, err = client.Do(req) // 所有rpc server都使用相同的DefaultClient与registry通讯，是否有问题？
	if err != nil {
		return err
	}
	return nil
}
