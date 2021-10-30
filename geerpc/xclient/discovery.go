package xclient

/*
Discovery是客户端本地维护的远端服务列表
*/

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomMode SelectMode = iota
	RoundRobinMode
)

type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
}

type MultiServersDiscovery struct {
	r       *rand.Rand
	mu      sync.RWMutex
	servers []string
	index   int
}

func NewMultiServersDiscovery(servers []string) *MultiServersDiscovery {
	msd := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	msd.index = msd.r.Intn(math.MaxInt32 - 1)
	return msd
}

// Refresh，从远程的registry中获取服务地址并更新本地的服务列表
// 如果远端rpc server有多个，但每个server注册的服务不同，那么本地怎么能找到提供指定服务的列表呢？
// 如果注册rpc server时，同时将其上面的service列表都注册到discovery本地，
// 才可能依据算法找到适当的rpc server。
// 其结构应该是按照service名称来映射服务列表
// map[string][]string  ->  serviceName/servers
//
// 没有远程的服务端Registry，因此Refresh是没有意义的
func (msd *MultiServersDiscovery) Refresh() error {
	return nil
}

// 注意参数都是直接浅拷贝过的，因此把servers赋值给msd.servers的作用是将新创建的servers安置到msd中。
// 换句话说，Update之前，需要先创建一个独立的servers slice。
func (msd *MultiServersDiscovery) Update(servers []string) error {
	msd.mu.Lock()
	defer msd.mu.Unlock()
	msd.servers = servers
	return nil
}

func (msd *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	// 根据selectmode
	// 如果是随机，则用r生成一个随机数
	// 如果是roundrobin，则用index取下一个
	msd.mu.Lock()
	defer msd.mu.Unlock()
	mod := len(msd.servers)
	if mod == 0 {
		return "", errors.New("no servers")
	}
	switch mode {
	case RandomMode:
		return msd.servers[msd.r.Intn(mod)], nil
	case RoundRobinMode:
		// msd.index的初始值可能很大,servers也可能中间被并行更新
		server := msd.servers[msd.index%mod]
		msd.index = (msd.index + 1) % mod
		return server, nil
	default:
		return "", errors.New("not supported load balance mode")
	}
}

func (msd *MultiServersDiscovery) GetAll() ([]string, error) {
	msd.mu.Lock()
	defer msd.mu.Unlock()
	servers := make([]string, len(msd.servers))
	copy(servers, msd.servers)
	return servers, nil
}

var _ Discovery = (*MultiServersDiscovery)(nil)
