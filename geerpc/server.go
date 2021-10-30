package geerpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"geektutu/geerpc/codec"
)

const magicNumber = 0x3bef5c

type Option struct {
	MagicNumber    int
	CodecType      codec.Type
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

var DefaultOption Option = Option{
	MagicNumber:    magicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: 10 * time.Second,
}

type Server struct {
	// server内含一个可访问的服务映射表：服务名/服务实例
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

// 将一个具体的用于提供服务的对象实例添加到Server的sync.Map中
// 后续Server会从中加载对应对象实例并向客户端提供服务。
func (srv *Server) Register(service interface{}) error {
	svc, err := newService(service)
	if err != nil {
		return err
	}
	// 将服务实例store到map中。
	if _, dup := srv.serviceMap.LoadOrStore(svc.name, svc); dup {
		// 重复注册并不是个严重的问题，在日志中提示即可
		log.Printf("rpc server: service %s has already been registered\n", svc.name)
	}
	log.Printf("rpc server: service %s register successfully\n", svc.name)
	return nil
}

// 根据请求头Header.ServiceMethod字符串，返回装载的对象实例，以及具体方法对象
func (srv *Server) findServiceMethod(ServiceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(ServiceMethod, ".")
	if dot < 0 {
		err = fmt.Errorf("rpc server: %s is ill-formed format. should be <service.method>", ServiceMethod)
		return
	}
	serviceName := ServiceMethod[:dot]
	methodName := ServiceMethod[dot+1:]

	// 从Server中加载Service，得到*service
	svci, ok := srv.serviceMap.Load(serviceName)
	if !ok {
		err = fmt.Errorf("rpc server: service %s doesn't exist", serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.methods[methodName]
	if mtype == nil {
		err = fmt.Errorf("rpc server: method %s doesn't exist", methodName)
	}
	return
}

func Register(service interface{}) error {
	return DefaultServer.Register(service)
}

var DefaultServer = NewServer()

func (srv *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		log.Println("rpc server: accept one request, start serving...")
		go srv.ServeConn(conn)
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func (srv *Server) ServeConn(conn io.ReadWriteCloser) {
	// 因为这里涉及到了编解码和逻辑判断，因此有可能失败，所以要在这里添加defer确保各种情况下都能关闭连接
	defer func() {
		_ = conn.Close()
	}()

	log.Println("rpc server: start check option of codec...")
	// 首先解析客户端请求，并用json解码option
	var opt Option
	// json.NewDecoder()会引入自己的buffer然后可能读取超过比请求的JSON值更多的字节数，
	// 这样的话，会不会导致后面进行编解码值异常？从json.Decoder.Decode(v interface{})的实现看，
	// 它只会从流中读取单个json编码的go值。
	jsonDec := json.NewDecoder(conn)
	if err := jsonDec.Decode(&opt); err != nil {
		log.Println("option decode error:", err)
		return
	}
	if opt.MagicNumber != magicNumber {
		log.Println("invalid rpc request")
		return
	}
	// 然后根据option中的字段获取具体的编解码器实例
	codecFunc := codec.NewCodecFuncMap[opt.CodecType]
	if codecFunc == nil {
		log.Println("invalid codec type")
		return
	}
	log.Println("rpc server: codec is ", opt.CodecType)
	log.Println("rpc server: preparing for getting request...")
	// 最后使用编解码器解析请求并给出响应
	srv.ServeCodec(codecFunc(conn), opt.HandleTimeout)
}

// 无效请求给回的响应，在请求有问题时返回，注意提前设置Header中的Error参数
var invalidRequestResponse = struct{}{}

func (srv *Server) ServeCodec(cc codec.Codec, timeout time.Duration) {
	c := cc.(*codec.GobCodec).Conn.(net.Conn)
	log.Printf("rpc server %s: servercodec: timeout is %s\n", c.LocalAddr().String(), timeout)
	// codec的具体实现有一个conn字段用于表示连接，我们只需要根据接口来读取Header，body，并写响应信息
	// Header包含了ServiceMethod，body中包含了请求的参数，因此我们会根据这两类信息，在本地调用
	// 执行命令，然后将结果编码后写入返回。
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup) // 等待所有requests都被处理，每个request包含一个header，一个body

	for {
		req, err := srv.readRequest(cc)
		if err != nil { // 读取单个请求（包括头-service.method和body参数）发现错误，直接
			if req != nil {
				req.h.Error = err.Error()
				srv.sendResponse(cc, req.h, invalidRequestResponse, sending)
			}
			break
		}
		log.Printf("rpc server %s: get a request(%s(%v)) from %s: ", c.LocalAddr().String(), req.h.ServiceMethod, req.argv, c.RemoteAddr().String())
		log.Println("rpc server: start handling... ")

		wg.Add(1)
		go srv.handleRequest(cc, req, sending, wg, timeout)
	}
	wg.Wait()
	cc.Close()

}

// 读取的Header（包含Service.Method）,以及输入参数和输出参数。
// 这个request仅仅用于server端，其实不定义这个也可以。net/rpc就没有定义这个对象，而是直接使用多个返回值：
// func (server *Server) readRequest(codec ServerCodec) (service *service, mtype *methodType, req *Request, argv, replyv reflect.Value, keepReading bool, err error)
// 其中service抽象具体的服务实例，methodType抽象表示具体的方法
// type service struct {
//	name   string                 // name of service
//	rcvr   reflect.Value          // receiver of methods for the service
//	typ    reflect.Type           // type of the receiver
//	method map[string]*methodType // registered methods
//}
type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	svc          *service
	method       *methodType
}

// 从连接的输入流中读取请求，关键是请求的
// service.method（含有arg，reply的类型信息）
// arg的具体取值
// 拿到这三个信息就足够了。然后再根据这些信息，在本地调用服务方法，
// 此时会将结果写入到reply对象中，最后将这个对象连同请求头编码后写入到流中，响应给客户端。
// 其实reply的类型及值也可以从前面三个信息中获取，但太麻烦
// 所以直接在解析method，得到arg的type时，也同时保存reply的type，并返回。
func (srv *Server) readRequest(cc codec.Codec) (req *request, err error) {
	var header codec.Header
	if err = cc.ReadHeader(&header); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			// 发现错误
			log.Println("rpc server: read header error:", err)
		}
		return
	}

	req = &request{}
	req.h = &header
	req.svc, req.method, err = srv.findServiceMethod(header.ServiceMethod)
	if err != nil {
		// 没有找到对应的服务.方法，说明请求有误，因为请求头已经读取成功，
		// 我们尝试读取body参数后，返回这个错误，以便继续服务下一个请求
		cc.ReadBody(nil)
		return
	}
	req.argv = req.method.newArgValue()
	req.replyv = req.method.newReplyValue()

	// readbody的参数必须是一个指针对象，且这个所指的对象必须已经分配了内存。
	// 但实际方法的定义上，argv即可能是指针，也可能不是指针。
	// newArgValue()根据情况创建一个具体对象，或者一个已经分配了内存的指针对象
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		// 常规情况下我们传输的是一个普通的对象作为参数，所以我们要将其转换为一个指针对象
		argvi = req.argv.Addr().Interface()
	}

	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read argv err:", err)
		return
	}
	return req, nil
}

func (srv *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error", err)
	}
	log.Println("rpc server: write response(header/body) ", h, "/", body)
}

// 使用编解码器cc，读取请求并写回响应req。多个go routine之间要共享同一rpc通道，因此需要使用sending来互斥
// 如果主go routine因特别原因跳出for循环，必须等待所有子go routine结束后，才能结束，因此需要使用wg。
func (srv *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	log.Printf(
		"rpc server: handle request: (*%s) %s(%s, *%s); request arg:%v\n",
		req.svc.name,
		req.method.method.Name,
		req.argv.Type().Name(),
		req.replyv.Elem().Type().Name(),
		reflect.Indirect(req.argv).Interface(),
	)
	// 使用通道来表示完成的状态
	called := make(chan struct{})
	// 启用goroutine调用后端服务
	go func() {
		err := req.svc.call(req.method, req.argv, req.replyv)
		if err != nil {
			req.h.Error = err.Error()
			log.Printf("rpc server: call back service error: %s\n", req.h.Error)
			called <- struct{}{}
			return
		}
		called <- struct{}{}
	}()
	if timeout == 0 {
		<-called
		if req.h.Error != "" {
			srv.sendResponse(cc, req.h, invalidRequestResponse, sending)
		}
		srv.sendResponse(cc, req.h, req.replyv.Elem().Interface(), sending)
		return
	}
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("call back service timeout in %s seconds", timeout)
		log.Printf("rpc server: call back service error: %s\n", req.h.Error)
		srv.sendResponse(cc, req.h, invalidRequestResponse, sending)
	case <-called:
		if req.h.Error != "" {
			srv.sendResponse(cc, req.h, invalidRequestResponse, sending)
			return
		}
		srv.sendResponse(cc, req.h, req.replyv.Elem().Interface(), sending)
	}
}

const (
	// Status-Code Reason-Phrase
	codeReason          = "202 Connected to Gee RPC accepted"
	defaultRPCPath      = "/_geerpc"
	defaultRPCDebugPath = "/debug/geerpc"
)

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//log.Println("http server: start serving")
	// 读取响应，如果是到指定路径的CONNECT连接，则劫持后，将请求的tcp连接交给后端创建服务
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("http request must use CONNECT first"))
		return
	}
	// 劫持底层的tcp连接，用于rpc协议数据包处理
	// 相当于将rpc包封装在http隧道中。
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(
			fmt.Sprintf("failed to hijack remote client %s connection: %s", req.RemoteAddr, err.Error()),
		))
		log.Printf("failed to hijack remote client %s connection: %s", req.RemoteAddr, err.Error())
		return
	}

	// 拿到连接后，将其交给后端的rpc进行处理。注意此时，服务端只发了一个响应的status_line
	// 而没有发送其他任何头和内容。
	// 其预设前提是，客户端会基于这个连接发送rpc请求
	// 即基于rpc协议发送请求（发送json编解码协商包，然后是header+args_body|header+args_body
	log.Println("http server: send back http response")
	io.WriteString(conn, "HTTP/1.0 "+codeReason+"\n\n")
	s.ServeConn(conn)
}

func (s *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, s)
	log.Println("http: rpc server path:", defaultRPCPath)
	http.Handle(defaultRPCDebugPath, debugHTTP{s})
	log.Println("http: rpc server debug path:", defaultRPCDebugPath)
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
