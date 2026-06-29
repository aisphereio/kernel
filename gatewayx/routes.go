// Package gatewayx provides Gateway/BFF building blocks on top of Kernel's
// existing transportx/http package.
//
// The route registry is a small go-zero-inspired target for generated code: a
// future kernel api generator can emit []Route and []Service values instead of
// making business services register routes manually in main.go.
package gatewayx

import (
	"fmt"
	"net/http"
	"strings"

	httpx "github.com/aisphereio/kernel/transportx/http"
)

// HandlerFunc is the Kernel HTTP business handler.
type HandlerFunc = httpx.HandlerFunc

// Route describes a single external Gateway/BFF route.
type Route struct {
	Method  string
	Path    string
	Handler HandlerFunc
	Filters []httpx.FilterFunc
	Name    string
}

// Service groups routes under a common prefix and filter chain. It is the
// runtime shape that a go-zero-like .api generator should target.
type Service struct {
	Name    string
	Prefix  string
	Filters []httpx.FilterFunc
	Routes  []Route
}

// RegisterServices registers route groups under parent. It keeps transportx/http
// untouched while giving services a generated-code-friendly entry point.
func RegisterServices(parent *httpx.Router, services ...Service) error {
	if parent == nil {
		return fmt.Errorf("gatewayx: nil router")
	}
	for _, svc := range services {
		router := parent.Group(svc.Prefix, svc.Filters...)
		if err := RegisterRoutes(router, svc.Routes...); err != nil {
			return fmt.Errorf("register service %q: %w", svc.Name, err)
		}
	}
	return nil
}

// RegisterRoutes registers routes on router.
func RegisterRoutes(router *httpx.Router, routes ...Route) error {
	if router == nil {
		return fmt.Errorf("gatewayx: nil router")
	}
	for _, rt := range routes {
		if rt.Handler == nil {
			return fmt.Errorf("gatewayx: route %s %s has nil handler", rt.Method, rt.Path)
		}
		method := strings.ToUpper(strings.TrimSpace(rt.Method))
		if method == "" {
			method = http.MethodGet
		}
		switch method {
		case http.MethodGet:
			router.GET(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodHead:
			router.HEAD(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodPost:
			router.POST(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodPut:
			router.PUT(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodPatch:
			router.PATCH(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodDelete:
			router.DELETE(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodOptions:
			router.OPTIONS(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodConnect:
			router.CONNECT(rt.Path, rt.Handler, rt.Filters...)
		case http.MethodTrace:
			router.TRACE(rt.Path, rt.Handler, rt.Filters...)
		case "*":
			router.Handle("*", rt.Path, rt.Handler, rt.Filters...)
		default:
			return fmt.Errorf("gatewayx: unsupported method %q for %s", rt.Method, rt.Path)
		}
	}
	return nil
}
