package gee

import (
	"fmt"
	"net/http"
	"strings"
)

// router
// roots key eg, roots['GET'] roots['POST']
// handlers key eg, handlers['GET-/p/:lang/doc'], handlers['POST-/p/book']
type router struct {
	// root for each method(get, post, ...)
	roots map[string]*node
	//
	handlers map[string]HandlerFunc
}

func newRouter() *router {
	return &router{
		roots:    map[string]*node{},
		handlers: map[string]HandlerFunc{},
	}
}

func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	fmt.Println("add route:", method, pattern)
	parts := parsePattern(pattern)

	key := method + "-" + pattern
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{}
	}

	r.roots[method].insert(pattern, parts, 0)
	r.handlers[key] = handler
}

func (r *router) getRoute(method string, path string) (*node, map[string]string) {
	searchParts := parsePattern(path)
	params := make(map[string]string)

	root, ok := r.roots[method]
	if !ok {
		return nil, nil
	}

	n := root.search(searchParts, 0)

	if n != nil {
		parts := parsePattern(n.pattern)
		for index, part := range parts {
			if part[0] == ':' {
				params[part[1:]] = searchParts[index]
			}
			if part[0] == '*' && len(part) > 1 {
				params[part[1:]] = strings.Join(searchParts[index:], "/")
				break
			}
		}
		return n, params
	}
	return nil, nil
}

func (r *router) handle(c *Context) {
	defer Recover()
	// handlers的key是pattern，不是path，我们需要根据path解析得到pattern
	//key := c.Method + "-" + c.Path
	n, params := r.getRoute(c.Method, c.Path)
	//fmt.Println("get router:", n.pattern, params)
	if n != nil {
		key := c.Method + "-" + n.pattern
		c.Params = params
		c.handlers = append(c.handlers, r.handlers[key])
		//r.handlers[key](c)
	} else {
		c.handlers = append(c.handlers, func(c *Context) {
			c.Stringf(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
		})
	}
	c.Next()
}

func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")

	var parts []string
	for _, item := range vs {
		if item != "" {
			parts = append(parts, item)
			if item[0] == '*' {
				break
			}
		}
	}
	return parts
}
