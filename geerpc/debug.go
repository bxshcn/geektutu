package geerpc

import (
	"fmt"
	"html/template"
	"net/http"
)

const debugText = `<html>
	<body>
	<title>GeeRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

var debug = template.Must(template.New("PRC debug").Parse(debugText))

// 嵌套指针类型
type debugHTTP struct {
	*Server
}

type debugService struct {
	Name   string
	Method map[string]*methodType
}

func (server debugHTTP) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	var services []debugService

	// 将原来server中的服务实例转储到debug的临时对象中。
	// 除了用Range，我们还可以用什么方法遍历sync.Map呢？就是用Range好了：
	// 查看sync.Map类的方法集，就是Range.
	server.serviceMap.Range(func(namei, svci interface{}) bool {
		svc := svci.(*service)
		services = append(services, debugService{
			Name:   namei.(string),
			Method: svc.methods,
		})
		return true
	})

	err := debug.Execute(resp, services)
	if err != nil {
		fmt.Fprintln(resp, "rpc:error handling template", err.Error())
	}
}
