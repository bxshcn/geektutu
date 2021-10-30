package geerpc

import (
	"errors"
	"fmt"
	"go/token"
	"log"
	"reflect"
	"sync"
)

// methodType表示service的一个method, 其实这个命名有点问题，直接命名method更合适，它就是表示一个方法
// 我们当然可以直接使用reflect.Method，但为了使用的方便，我们实际是在这个类型基础上
// ***做了封装***，加入了ArgType,ReplyType两个方便的字段。
// 一个method包括名称，入参和出参三个部分，另外还包括receive对象，其核心是Type
// 知道了Type，就能复原整个方法:
// 首先是入参，reflect.New(argType).Interface()得到具体的对象实例，然后从连接流中获取字节并初始化。
// 然后是出参，reflect.New(replyType).Elem()，Set()后，在通过Interface()转换为具体的对象，最后写到连接流中。
// service本身也需要被首相，
type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	mu        sync.Mutex
	numCalls  uint64
}

func (m *methodType) NumCalls() uint64 {
	m.mu.Lock()
	n := m.numCalls
	m.mu.Unlock()
	return n
}

// 根据类型创建一个具体的对象，用于接收请求中的body作为参数
func (m *methodType) newArgValue() reflect.Value {
	if m.ArgType.Kind() == reflect.Ptr { // 如果argType是指针类型，则返回一个指向对应类型的指针对象
		return reflect.New(m.ArgType.Elem())
	}
	return reflect.New(m.ArgType).Elem()
}

// 根据类型创建一个具体的对象，用于创建一个本地的对象然后返回给客户端。
func (m *methodType) newReplyValue() reflect.Value {
	// m.ReplyType必然是一个指针类型：作为返回值
	value := reflect.New(m.ReplyType.Elem())
	// 对于map和slice而言，reflect.New返回的是未初始化的对象，因此需要进行初始化
	// 下面只做了最基本的初始化，因为slice和map都是可自扩展管理的。
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		value.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
		// return reflect.Indirect(value)
	case reflect.Slice:
		value.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return value
}

// service是服务的内部抽象表示
// name字段用于根据注册映射表得到对应的服务对象
// typ表示服务的类型
// val表示服务对象本身，也是方法的receiver对象。
// method表示方法集
//
// 在go中，我们可以动态创建一个结构体，但无法动态创建结构体的方法，因此这些方法需要在注册时设定，
// 并保存在service对象中。与之类似的是val，它其实可以根据typ间接得到，但因为太常用，所以单独作为一个子域
type service struct {
	name    string
	typ     reflect.Type
	val     reflect.Value
	methods map[string]*methodType
}

// 首先我们根据一个具体的服务对象创建一个service对象
// 源服务对象可能是一个指针指向的对象实例，因此这里用到了reflect.Indirect()
func newService(s interface{}) (svc *service, err error) {
	svc = new(service)
	// 根据反射可以得到
	svc.typ = reflect.TypeOf(s)
	svc.val = reflect.ValueOf(s)
	//name := typ.Name()
	// 只有defined type的Name()才返回具体的类型名
	svc.name = reflect.Indirect(svc.val).Type().Name()
	// TODO: 有很多服务名的检查条件，这里只检查服务名是导出的
	if !token.IsExported(svc.name) {
		err = fmt.Errorf("rpc server: service %s is not exported", svc.name)
		return nil, err
	}
	svc.methods = suitableMethods(svc.typ)
	return
}

// 装配参数svc，找到service的有效方法
// 有效的方法必须满足如下条件：
// 两个导出或内置类型的入参, 注意反射包中看到的是三个！第1个为服务对象自身
// 一个出参，类型为error
// 第二个参数必须是指针
func suitableMethods(serviceType reflect.Type) map[string]*methodType {
	methods := make(map[string]*methodType)

	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			log.Printf("rpc server: register service %s: method %s is not suilted for rpc request, ommited\n", serviceType.Name(), method.PkgPath+method.Name)
			continue
		}
		// 出参必须是error类型
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			log.Printf("rpc server: register service %s: method %s is not suilted for rpc request, ommited\n", serviceType.Name(), method.PkgPath+method.Name)
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		// 返回值必须是指针类型
		if replyType.Kind() != reflect.Ptr {
			log.Printf("rpc server: register service %s: method %s is not suilted for rpc request, ommited\n", serviceType.Name(), method.PkgPath+method.Name)
			continue
		}
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			log.Printf("rpc server: register service %s: method %s is not suilted for rpc request, ommited\n", serviceType.Name(), method.PkgPath+method.Name)
			continue
		}
		methods[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register service %s: adding method %s\n", serviceType.Name(), method.PkgPath+method.Name)
	}
	return methods
}

func isExportedOrBuiltinType(typ reflect.Type) bool {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return token.IsExported(typ.Name()) || typ.PkgPath() == ""
}

// 每个请求都是在单独的goroutine中处理的，也就是说这里的call是在单独的goroutine中被调用的，
// 因此这里就是单纯的同步调用场景
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	// 添加调用计数
	m.mu.Lock()
	m.numCalls++
	m.mu.Unlock()
	// 给定的参数的类型必须与方法参数类型一致
	if m.ArgType != argv.Type() || m.ReplyType != replyv.Type() {
		// TODO, 完善错误信息
		return errors.New("rpc server: call method error")
	}
	// 调用
	resultValues := m.method.Func.Call([]reflect.Value{s.val, argv, replyv})
	if resulti := resultValues[0].Interface(); resulti != nil {
		return resulti.(error)
	}
	return nil
}
