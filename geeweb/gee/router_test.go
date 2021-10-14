package gee

import (
	"reflect"
	"testing"
)

func TestParsePattern(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		want  []string
	}{
		{
			desc:  "无参数情况",
			input: "/p/a/b",
			want:  []string{"p", "a", "b"},
		},
		{
			desc:  "带:参数情况",
			input: "/p/:name",
			want:  []string{"p", ":name"},
		},
		{
			desc:  "带*参数情况",
			input: "/p/*",
			want:  []string{"p", "*"},
		},
		{
			desc:  "带多*参数情况",
			input: "/p/*name/*",
			want:  []string{"p", "*name"},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got := parsePattern(tC.input)
			if !reflect.DeepEqual(got, tC.want) {
				t.Errorf("parsePattern(%s)=%v, but we want %v\n", tC.input, got, tC.want)
			}
		})
	}
}

func newTestRouter() *router {
	r := newRouter()
	r.addRoute("GET", "/", nil)
	r.addRoute("GET", "/hello/:name", nil)
	r.addRoute("GET", "/hello/b/c", nil)
	r.addRoute("GET", "/assets/*filepath", nil)
	return r
}

func TestRoute(t *testing.T) {
	// 初始化routers
	r := newTestRouter()

	testCases := []struct {
		desc  string
		input []string
		want1 string
		want2 map[string]string
	}{
		{
			desc:  "单变量参数",
			input: []string{"GET", "/hello/geektutu"},
			want1: "/hello/:name",
			want2: map[string]string{"name": "geektutu"},
		},
		{
			desc:  "任意匹配变量参数",
			input: []string{"GET", "/assets/images/sun.jpg"},
			want1: "/assets/*filepath",
			want2: map[string]string{"filepath": "images/sun.jpg"},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			n, ps := r.getRoute(tC.input[0], tC.input[1])
			//t.Error(n.pattern, ps)
			if n == nil || n.pattern != tC.want1 || !reflect.DeepEqual(ps, tC.want2) {
				t.Errorf("getRoute(%s, %s), we got pattern=%s and params=%v, but we want pattern=%s and params=%v\n",
					tC.input[0], tC.input[1], n.pattern, ps, tC.want1, tC.want2)
			}
		})
	}
}
