package gee

import (
	"html/template"
	"net/http"
	"path"
	"strings"
)

type HandlerFunc func(c *Context)

type RouterGroup struct {
	prefix      string        // 表示一个RouteGroup
	middlewares []HandlerFunc // support middleware, middleware就是一个输入为Context的处理函数，处理结果更新Context
	parent      *RouterGroup  // support nesting
	// 整个框架的所有资源(包括router）都是由Engine统一协调，为了访问router的能力，内嵌一个指向Engine的指针来获取router
	engine *Engine // all groups share a Engine instance
}

// Group 在当前group下创建新的group，即嵌套创建
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	engine := group.engine

	newGroup := &RouterGroup{
		prefix:      group.prefix + prefix,
		middlewares: group.middlewares,
		parent:      group,
		engine:      engine}

	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

/*func (e *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	e.router.addRoute(method, pattern, handler)
}*/
func (group *RouterGroup) addRoute(method string, comp string, handler HandlerFunc) {
	pattern := group.prefix + comp
	group.engine.router.addRoute(method, pattern, handler)
}

// GET defines the method to add GET request
/*func (engine *Engine) GET(pattern string, handler HandlerFunc) {
	engine.addRoute("GET", pattern, handler)
}

// POST defines the method to add POST request
func (engine *Engine) POST(pattern string, handler HandlerFunc) {
	engine.addRoute("POST", pattern, handler)
} */

func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}

// 只是将中间件添加到group的middlewares域中，真正起作用是根据group的middleware和对应router的handler，
// 具体策略是先在ServerHTTP中将middlewares填充到context，然后以Ccontext来调用针对router注册的handler
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

// group.Static("/assets", "/usr/geektutu/blog/static")
// relativePath指定访问URL中静态资源的路径的根路径，root指定文件在本地磁盘所在的路径
func (group *RouterGroup) Static(relativePath string, root string) {
	// http.Dir(root)是强制类型的转换，赋予root这个string FileSystem的能力
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	// 为relativePath指定路径下的文件添加路由
	urlPattern := path.Join(relativePath, "/*filepath")
	group.GET(urlPattern, handler)
}

// 这个就是创建一个http.FileServer
// 其本质是一个HandlerFunc类型的对象，每个HandlerFunc对象
// urlRelativePath/*filepath，你要访问的实际是后面的filepath所代表的文件，它们被置于fs的根目录下，比如/usr/web/blog/目录
func (group *RouterGroup) createStaticHandler(urlRelativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := path.Join(group.prefix, urlRelativePath)
	// 剥离urlRelativePath, 获取静态文件的相对路径filepath, 然后与后面的/usr/web/blog拼接得到文件在本地的真正路径。
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	// HandlerFunc的作用就是根据context写c.Writer这个对象
	// 在其他地方，一些简单的场景（尤其是api的json等）我们调用Writer接口相关自己实现读写，
	// 但对于文件服务器，我们要直接使用文件服务器提供的函数ServeHTTP。
	// 至于http.Handler接口类型，我们不用考虑其设计细节（比如封装等）。因为这里我们只考虑写数据。
	// 或者简单说，Handler.ServeHTTP(c.Writer, c.Req)等同于我们的HandlerFunc(c)，都是实现往客户端写响应数据
	return func(c *Context) {
		file := c.Param("filepath")
		// just 测试
		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		//c.Status(http.StatusOK)
		fileServer.ServeHTTP(c.Writer, c.Req)
	}
}

// Engine implement the interface of ServeHTTP
// Engine最核心的能力就是路由，router，然后加上groups域是为了支持分组功能。
type Engine struct {
	// Engine作为最顶层的分组，也就是说Engine拥有RouterGroup所有的能力
	*RouterGroup
	router *router
	groups []*RouterGroup // store all groups
	// for模板支持
	htmlTemplates *template.Template
	funcMap       template.FuncMap
}

// New is the constructor of gee.Engine
func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

// Run defines the method to start a http server
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := newContext(w, req)

	var middlewares []HandlerFunc
	for _, group := range engine.groups {
		if strings.HasPrefix(c.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}

	c.engine = engine

	c.handlers = middlewares
	engine.router.handle(c)
}

func (engine *Engine) SetFuncMap(fm template.FuncMap) {
	engine.funcMap = fm
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}
