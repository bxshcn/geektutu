package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// 这是一个通用的object定义，可以用于方便的构建一个对象
type Obj map[string]interface{}

type Context struct {
	Req    *http.Request
	Writer http.ResponseWriter

	// 处理之后的request info，比如请求路径，方法，和相关参数（路径中的变量），
	// 当然后续还会添加中间件或者其他渠道置入的参数
	// 注意这些信息都可以从原始的Req中得到，但为了方便，我们将这些重要信息直接解析出来放到这里，方便使用
	Path   string
	Method string
	Params map[string]string

	StatusCode int

	// 记录当前中间件（待施加的处理函数列表）列表，包括路由处理函数
	handlers []HandlerFunc
	index    int

	// 增加到engine的访问，获取其中的htmlTemplates
	engine *Engine
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Req:    req,
		Writer: w,
		Path:   req.URL.Path,
		Method: req.Method,
		index:  -1,
	}
}

func (c *Context) Param(key string) string {
	value, _ := c.Params[key]
	return value
}

func (c *Context) PostForm(key string) string {
	return c.Req.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Req.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) Stringf(code int, format string, values ...interface{}) {
	c.SetHeader("Content-Type", "text/plain")
	c.Status(code)
	// TODO， handle error
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	// TODO, handle error
	if err := encoder.Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.Writer.Write(data)
}

/*func (c *Context) HTML(code int, html string) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	c.Writer.Write([]byte(html))
}*/

func (c *Context) HTML(code int, name string, data interface{}) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	//根据pattern生成template，然后用c中的参数实例化后，写到c.Writer中
	if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.Fail(500, err.Error())
	}
}

func (c *Context) Fail(code int, err string) {
	c.Data(500, []byte(err))
}

func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		c.handlers[c.index](c)
	}
}
