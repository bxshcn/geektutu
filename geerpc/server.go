package geerpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"

	"geektutu/geerpc/codec"
)

const magicNumber = 0x3bef5c

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOption Option = Option{
	MagicNumber: magicNumber,
	CodecType:   codec.GobType,
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
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
	srv.ServeCodec(codecFunc(conn))
}

// 无效请求给回的响应，在请求有问题时返回，注意提前设置Header中的Error参数
var invalidRequestResponse = struct{}{}

func (srv *Server) ServeCodec(cc codec.Codec) {
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
		log.Println("rpc server: get a request: ", req.h.ServiceMethod, req.h.Seq)
		log.Println("rpc server: start handling... ")

		wg.Add(1)
		go srv.handleRequest(cc, req, sending, wg)
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
}

// 从连接的输入流中读取请求并解析，将所得放到request对象中。
func (srv *Server) readRequest(cc codec.Codec) (*request, error) {
	var header codec.Header
	if err := cc.ReadHeader(&header); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			// 发现错误
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}

	req := &request{h: &header}

	// 默认请求对象是字符串，我们用一个字符串来接受请求
	req.argv = reflect.New(reflect.TypeOf(""))
	if err := cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
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

// 暂时只响应固定的信息：geerpc resp num，
func (srv *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("rpc server, the getting request is(header/body) ", *(req.h), "/", req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	srv.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}
