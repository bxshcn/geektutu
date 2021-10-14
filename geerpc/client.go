package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"geektutu/geerpc/codec"
)

type Call struct {
	Seq           uint64
	ServiceMethod string // <service>.<method>
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

// 如果一个call结束，即接收到服务端的正确匹配的响应，则将该call写入到call自身的Done通道，表示调用结束。
// 利用异步调用实现同步，当然可以实现完全意义上的同步调用。
func (c *Call) done() {
	c.Done <- c
}

// Client用于方便用户发起rpc请求
// 其接口包括同步Client.Call和异步Client.Go
// 首先Client应该包含一个编解码器实例，对应一个具体的rpc网络连接
// 基于这个rpc连接，用户可以并发发起多个rpc请求调用，每个rpc请求调用都是独立的。
// 但由于网络连接是共享的，因此同一时刻只能允许发起一个请求
// 获取响应也是如此，需要
type Client struct {
	cc codec.Codec // 使用接口来表示一个具体的编解码实例，在初始化客户端时必须提供一个具体的编解码实例

	// client已经有了cc这个编解码信息，因此不再需要opt，
	// 当然为了方便我们可以将opt放到Client结构中，但目前看不出有什么作用
	// opt *Option
	headMutex sync.Mutex
	header    *codec.Header

	mu       sync.Mutex // 保护下面Client的全局参数
	seq      uint64
	pending  map[uint64]*Call
	closing  bool // 表示client端主动关闭连接
	shutdown bool // 表示服务端或者其他情况（非client端）关闭连接的情况
}

var ErrShutdown error = errors.New("connection is shut down")

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// 我们将新生成的call附加到pending中，即完成注册，后续得到一个响应，
// 就根据请求的seq来匹配，确认是合法的响应。这个seq是client发起请求时维护的一个全局变量
func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.seq++
	c.pending[call.Seq] = call
	return call.Seq, nil
}

// 调用的前提是client有效（未关闭）的情况下
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// 调用的前提是client有效（未关闭）的情况下
// 但这个函数有什么用呢？
func (c *Client) terminateCalls(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.Done <- call
	}
}

// 发送请求
// 先将call注册到pending中，然后
// 根据call构建header，然后发送header，再发送call.args
func (c *Client) send(call *Call) {
	// 首先锁定header
	c.headMutex.Lock()
	defer c.headMutex.Unlock()

	// 将call注册到pending中
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 根据call得到header
	// 为了实施并发控制，必须使用client中的header参数，否则多个并发的请求会扰乱服务端接收到的信息。
	// var header codec.Header
	c.header.ServiceMethod, c.header.Seq = call.ServiceMethod, seq
	log.Println("rpc client: write request(header/body): ", *(c.header), "/", call.Args)
	err = c.cc.Write(c.header, call.Args)
	if err != nil {
		call := c.removeCall(call.Seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// 客户端的接口是通过go语句调用，不论异步还是同步。
func (c *Client) Go(ServiceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}

	call := &Call{
		ServiceMethod: ServiceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

// 客户端与服务端通过call中的Done（带缓冲的通道）来实施同步
func (c *Client) Call(ServiceMethod string, args, reply interface{}) error {
	call := <-c.Go(ServiceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

// receive接收响应信息，我们可以根据接收到的信息头中的seq来明确具体是哪个call。
// 包括codec.Header和replyv（类型为reflect.Value）还有错误信息
// 我们用这些信息更新本地pending中的call状态。对于正确响应的pending call，Error为nil
// Done通道可读取，表示call处理结束。客户端的整体流程：建立好和服务端的连接后，即go receive()
// 启动后台gorouine接收从服务端传来的所有响应。
// client的主go routine则为用户所控制，按照用户的业务流程实施操作，发起请求，然后等待服务端响应
// 如果是异步请求，则使用waitgroup来同步主业务goroutine与请求的gorouine。
// 如果是同步请求，则
// 服务端响应的消息写到连接的缓冲区，客户端必须尽快的处理，所以调用了Call或者Go，就必须立刻
// 区分几种情况，
// 1. call不存在，则直接返回
// 2. 有错误，call.Error非空
// 3. 正确处理，此时需要读取服务端发送的header和body：c.cc.ReadHeader(*Header),c.cc.ReadBody(interface{})
func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil { // 先读响应头
			break
		}
		log.Println("rpc client: receive response header: ", h)
		// 读取响应头成功，表明这个call服务端已经处理，因此需要从本地删除，同时进行后续处理
		call := c.removeCall(h.Seq)

		switch {
		case call == nil:
			// 本地没有对应的call，说明这个响应是针对过时的请求，直接读取body后抛弃
			// 再继续后续call的处理
			err = c.cc.ReadBody(nil)
		case h.Error != "":
			// 服务端处理该请求发生错误，因此body响应也应该无效，我们读取body后抛弃
			// 然后标记该call已经处理完毕，再继续后续call处理
			//call.Error = errors.New(h.Error)
			call.Error = fmt.Errorf(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			err = c.cc.ReadBody(call.Reply)
			log.Println("rpc client: receive response body: ", call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	c.terminateCalls(err)
}

func parseOption(cc ...codec.Type) (*Option, error) {
	if len(cc) > 1 {
		return nil, errors.New("should specify at most 1 codec type")
	}
	if len(cc) == 0 {
		return &DefaultOption, nil
	}

	return &Option{
		MagicNumber: magicNumber,
		CodecType:   cc[0],
	}, nil
}

// 提供用户接口，
func Dial(network, address string, cc ...codec.Type) (*Client, error) {
	// 首先分析opt的有效性，设置合理的Option
	opt, err := parseOption(cc...)
	if err != nil {
		return nil, err
	}
	// 确认参数正确的情况下，尝试和服务端建立网络连接
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	/*defer func() {
		conn.Close()
	}()*/
	// 然后创建Client，最后返回
	return NewClient(conn, opt)
}

func newClientCodec(cc codec.Codec) *Client {
	return &Client{
		seq:     1,
		cc:      cc,
		header:  &codec.Header{},
		pending: make(map[uint64]*Call),
	}
}

// 创建一个新client的实质，是创建一个Client对象，发送协商编解码信息，然后启动一个receive()的goroutine
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// client只负责发送请求，而不发送协商内容。
	// 只有依赖client实际发送了请求，才会有receive的需求。
	client := newClientCodec(f(conn))
	// 发送协商编解码信息
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		return nil, err
	}
	go client.receive()
	return client, nil
}