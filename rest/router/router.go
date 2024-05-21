package router

import (
	"attribute-db/logging"
	"errors"
	"fmt"
	"net/http"
)

type Method string
type Scheme string

const (
	GET    Method = "GET"
	POST   Method = "POST"
	UPDATE Method = "UPDATE"
	DELETE Method = "DELETE"
	PUT    Method = "PUT"
)

const (
	HTTP  Scheme = "http"
	HTTPS Scheme = "https"
)

type Router struct {
	scheme Scheme
	port   string
	prefix string //default '/'
	router *map[string]*router
}

type router struct {
	handler map[Method]http.HandlerFunc
}

func NewRouter(prefix string) *Router {
	route := make(map[string]*router)
	return &Router{prefix: prefix, router: &route}
}

func (r *Router) NewSubRouter(prefix string) *Router {
	if prefix[:1] != "/" {
		prefix = fmt.Sprint("/", prefix)
	}

	newPrefix := ""
	if r.prefix == "/" {
		newPrefix = prefix
	} else {
		newPrefix = fmt.Sprint(r.prefix, prefix)
	}

	newRouter := &Router{prefix: newPrefix, router: r.router}
	return newRouter
}

func (r *Router) SetPort(port string) *Router {
	r.port = port
	return r
}

func (r *Router) SetScheme(scheme Scheme) *Router {
	r.scheme = scheme
	return r
}

func (r *Router) SetHandler(method Method, handler http.HandlerFunc) *Router {
	prefix := r.prefix
	var targetRoute *router
	if srcRouter, ok := (*r.router)[prefix]; !ok {
		targetRoute = &router{make(map[Method]http.HandlerFunc)}
		(*r.router)[prefix] = targetRoute
	} else {
		if srcRouter == nil {
			srcRouter = &router{make(map[Method]http.HandlerFunc)}
			(*r.router)[prefix] = srcRouter
		} else {
			(*r.router)[prefix] = srcRouter
			targetRoute = srcRouter
		}
	}

	targetRoute.handler[method] = handler
	return r
}

func (r *Router) SetErrorPage(handler http.HandlerFunc) *Router {
	prefix := ""
	if r.prefix == "/" {
		prefix = "/error"
	} else {
		prefix = fmt.Sprint(r.prefix, "/error")
	}

	var targetRoute *router
	if srcRouter, ok := (*r.router)[prefix]; !ok {
		targetRoute = &router{make(map[Method]http.HandlerFunc)}
		(*r.router)[prefix] = targetRoute
	} else {
		if srcRouter == nil {
			srcRouter = &router{make(map[Method]http.HandlerFunc)}
			(*r.router)[prefix] = srcRouter
		} else {
			targetRoute = srcRouter
		}
	}

	targetRoute.handler[GET] = handler

	return r
}

// TODO: middleware 에서 로깅
func middleware(router Router) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		uri := r.RequestURI
		logging.PrintINFO(fmt.Sprintf("HTTP Recv Method : %s URI : %s", method, uri))

		if route, ok := (*router.router)[uri]; ok {
			if handler, ok := route.handler[Method(method)]; ok {
				handler(w, r)
			}
		}
	})
}

// TODO: Run에서 error channel 반환 필요
func (r *Router) Run() error {
	middle := middleware(*r)

	switch r.scheme {
	case "http":
		go func() {
			fmt.Println(http.ListenAndServe(r.port, middle))
		}()
		return nil
	case "https":
		return nil
	default:
		return errors.New("unkwon scheme")
	}
}
