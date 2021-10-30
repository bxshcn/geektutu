package xclient

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

type GeeRegistryDiscovery struct {
	*MultiServersDiscovery        // 底层的最原始的服务发现功能，所有不同的发现服务器都可共用。因此这里使用指针嵌套
	registry               string // 当前registry的地址
	// 指示discovery的客户端，获取服务列表的超时时间，
	// 每超过这个时间，则客户端本地就认为本地服务列表超时，
	// 需要重新从discovery获取新的服务列表。
	timeout    time.Duration
	lastUpdate time.Time
}

const (
	defaultUpdateTimeout = 10 * time.Second // 客户端更新本地服务列表的时间间隔

	geerpcHeader = "X-Geerpc-Server"
)

func NewGeeRegistryDiscovery(registryAddr string, timeout time.Duration) *GeeRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}

	return &GeeRegistryDiscovery{
		MultiServersDiscovery: NewMultiServersDiscovery(make([]string, 0)),
		registry:              registryAddr,
		timeout:               timeout,
	}
}

// Update使用自己本地已知的服务，来更新远端registry中的
func (grd *GeeRegistryDiscovery) Update(servers []string) error {
	grd.mu.Lock()
	defer grd.mu.Unlock()

	grd.servers = servers
	grd.lastUpdate = time.Now()

	return nil
}

// 从registry中获取服务列表，然后更新本地的servers
func (grd *GeeRegistryDiscovery) Refresh() error {
	grd.mu.Lock()
	defer grd.mu.Unlock()

	// 没有超时
	if grd.lastUpdate.Add(grd.timeout).After(time.Now()) {
		return nil
	}

	// 如果超时，则使用http协议，从registry中获取服务列表，然后更新本地列表
	resp, err := http.Get(grd.registry)
	if err != nil {
		return errors.New("refresh failed")
	}

	header := resp.Header.Get(geerpcHeader)
	servers := strings.Split(header, ",")

	// 我们没有考虑从http服务器registry中获取到的服务列表的格式的异常情况
	// 因为我们默认为registry返回的服务器就是正常格式的。
	grd.servers = servers
	grd.lastUpdate = time.Now()
	return nil
}

func (grd *GeeRegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := grd.Refresh(); err != nil {
		return "", err
	}
	return grd.MultiServersDiscovery.Get(mode)
}

func (grd *GeeRegistryDiscovery) GetAll() ([]string, error) {
	if err := grd.Refresh(); err != nil {
		return nil, err
	}
	return grd.MultiServersDiscovery.GetAll()
}
