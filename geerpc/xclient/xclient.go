package xclient

import (
	"context"
	. "geektutu/geerpc"
	"log"
	"reflect"
	"sync"
)

// XClient是一个更高级的客户端，它支持各种协议，并且将创建的客户端调用保存起来，方便共享底层的tcp连接。
// XClient只提供接口，其内部字段都是私有数据，因为我们只需要使用XClient的API接口即可
type XClient struct {
	d       Discovery  // 有一个内部的注册中心发现接口
	mode    SelectMode // 有一个特定的负载均衡模式
	opt     *Option    // 有一个特定的编解码参数配置
	mu      sync.Mutex
	clients map[string]*Client //本地缓存的已经创建的各个Client
}

func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	for key, client := range xc.clients {
		client.Close()
		delete(xc.clients, key)
	}

	return nil
}

// 根据目标地址建立连接
func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	// 首先判断访问这个地址的客户端是否存在，如果存在就直接返回
	// 否则先调用Client的方法创建一个新的Client，并将其添加到xc.clients中
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client, ok := xc.clients[rpcAddr]
	if ok && !client.IsAlive() { //死连接，需要从xc中剔除
		log.Println("死连接", rpcAddr, "，关闭然后重新建立连接")
		client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil { // 没有找到合适的连接
		log.Println("新建连接", rpcAddr)
		client, err := XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
		return client, nil
	}
	// 找到适当的连接，则直接返回对应的客户端
	log.Println("复用现有连接", rpcAddr)
	return client, nil
}

// 内部调用方法的实现
// 关键是参数，需要根据方法，上下文，还有地址实施调用，即在指定地址的服务上，调用其中的方法
func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, argv, replyv interface{}) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, argv, replyv)
}

func (xc *XClient) Call(ctx context.Context, serviceMethod string, argv, replyv interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	log.Printf("XClient.Call(). %s|%s|%v", rpcAddr, serviceMethod, argv)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, argv, replyv)
}

// Broadcast给所有服务发送方法请求
// 如果有任一服务器上执行失败，则整体失败
// 所有服务器都执行成功，则返回最后一个执行的方法结果（通过replyv）
// 一旦某个服务上的调用执行失败，则相关客户端上的所有calls都会被cancel掉。
// 整个情况是：xclient中有一个clients列表，每个client都访问一个特定的server，并有多个并发calls
// 对于一个特定的服务调用，在所有服务端执行，每个服务端都有一个特定的client对应建立的连接。
// 所谓Broadcast，就是建立len(servers)个clients，建立连接并并发调用对应服务，直到所有服务调用都返回
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, argv, replyv interface{}) error {
	servers, err := xc.d.GetAll()
	log.Printf("servers=%v\n", servers)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // protect e & replyDone
	var e error
	replyDone := replyv == nil

	ctx, cancel := context.WithCancel(ctx)

	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			// 如果已经得到一个响应，则不再并发调起服务。
			mu.Lock()
			if replyDone {
				return
			}
			mu.Unlock()
			var cloneReplyv interface{}
			if replyv != nil {
				cloneReplyv = reflect.New(reflect.ValueOf(replyv).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, argv, cloneReplyv)
			log.Printf("xc.call(%s, %v)=%v: error=%s\n", rpcAddr, argv, cloneReplyv, err)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				// 如果某个客户端连接的服务上执行方法失败，则cancel掉改client和该服务端的所有calls
				// 注意对外部来说，xclient.Broadcast并没有结束：我们没有得到适当的响应
				//replyv = nil
				//replyDone = true
				cancel()
			}
			if err == nil && !replyDone {
				//reflect.ValueOf(replyv).Elem().Set(reflect.ValueOf(cloneReplyv).Elem())
				replyv = cloneReplyv
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
