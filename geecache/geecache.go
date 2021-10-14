package geecache

import (
	"fmt"
	"log"
	"sync"

	pb "geektutu/geecache/geecachepb"
	"geektutu/geecache/singleflight"
)

// 接口函数
// 我们按照该接口的方法，客户化一个loader函数，实现的功能如下：
// 如果缓存命中，则直接返回[]byte，true,
// 如果缓存不中，则从外部加载数据到缓存，并返回加载的数据[]byte, false
// 这样我们只需要定义对应的接口函数，然后使用GetterFunc修饰，即可得到一个Getter对象。
//
// 注意到，cache类型
type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(string) ([]byte, error)

func (gf GetterFunc) Get(key string) ([]byte, error) {
	return gf(key)
}

type Group struct {
	name string
	// 用户定制的回调函数，当缓冲不中时调用
	getter Getter
	// 并发安全的cache
	// TODO
	// 为什么这里没有用指针呢？为什么cache类型中对lru.Cache的引用用的是指针？
	// 我先找一种解释，后续注意观察验证
	// 如果是对不同package中类型对象的引用，那么往往是指针，因为有可能先在别的地方定义，然后再这里引用
	// 如果是同一个package，且只有自己会使用，那么就直接用对象而不是相应的指针。
	mainCache cache
	// 所谓的PeerPicker实质上就是一个HTTPPool，它实现了PeerPicker接口
	// 我们可以HTTPPool视为一个远程缓存的查找对象。
	peer PeerPicker

	sc *singleflight.SingleCall
}

var (
	// 管理groups的
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil getter")
	}
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		sc:        &singleflight.SingleCall{},
	}
	mu.Lock()
	defer mu.Unlock()
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	g := groups[name]
	return g
}

// 我们的缓存，只是用来获取数据的，因此我们没有put，只有get
// 一直永远get，如果不存在（未命中），则从源获取数据，
// 如果Getter获取失败，则返回这个失败信息。
func (g *Group) Get(key string) (ByteView, error) {
	// 为什么要对空key进行特别处理呢？因为我们不允许key为空串！
	// 对于其他任意串，如果存在（不论在不在缓存中）则返回，否则0值和err
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if bv, ok := g.mainCache.get(key); ok {
		log.Printf("[GeeCache] key %s hit in cache \n", key)
		return bv, nil
	}
	// 我们之所以要定义load函数，是因为这个函数的语义复杂度等同于上面的get层面（从缓存中获取）
	// load表示：
	// 从本地源获取，然后构建对象，并将其加载到缓存中，最后返回这个对象。
	log.Printf("[GeeCache] key %s miss in cache\n", key)
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	if g.peer != nil {
		if peer, ok := g.peer.PickPeer(key); ok {
			val, err := g.sc.Do(key, func() (interface{}, error) {
				log.Printf("we try to get from remote server %s\n", (g.peer).(*HTTPPool).peers.Get(key))
				value, err := g.getFromPeer(peer, key)
				if err != nil {
					return value, err
				}
				return value, nil
			})
			return val.(ByteView), err
		}
	}
	log.Printf("we try to get from local\n")
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// Getter.Get会从本地返回一个[]byte对象，这个对象之前不存在，现在加载到了内存中
	// 因此后面可以直接使用这个对象，而不必再拷贝了。
	bs, err := g.getter.Get(key)
	if err != nil {
		// 返回0值和error对象
		return ByteView{}, err
	}
	bv := ByteView{bs}
	g.mainCache.add(key, bv)
	return bv, nil
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	request := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	response := &pb.Response{}
	//bytes, err := peer.Get(g.name, key)
	err := peer.Get(request, response)
	if err != nil {
		log.Println("[GeeCache] Failed to get from peer", err)
		return ByteView{}, err
	}
	//return ByteView{bytes}, nil
	return ByteView{response.Value}, nil
}

func (g *Group) RegisterPeerPicker(p PeerPicker) {
	if g.peer != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peer = p
}
