package gatewayx

import (
	"net/http"
	"strings"
)

// RouteMatch is the result of matching an external request to a generated route.
type RouteMatch struct {
	Route  GatewayRoute
	Params map[string]string
}

// Matcher resolves HTTP method/path to a GatewayRoute. It intentionally keeps a
// small surface so an etcd route controller can swap snapshots atomically.
type Matcher struct{ routes []GatewayRoute }

func NewMatcher(routes []GatewayRoute) Matcher {
	copied := append([]GatewayRoute(nil), routes...)
	return Matcher{routes: copied}
}

func (m Matcher) Match(method, path string) (RouteMatch, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	for _, rt := range m.routes {
		rtMethod := strings.ToUpper(strings.TrimSpace(rt.Method))
		if rtMethod == "" {
			rtMethod = http.MethodGet
		}
		if rtMethod != method && rtMethod != "*" {
			continue
		}
		params, ok := matchPath(rt.Path, path)
		if ok {
			return RouteMatch{Route: rt, Params: params}, true
		}
	}
	return RouteMatch{}, false
}

func matchPath(pattern, path string) (map[string]string, bool) {
	pp := splitPath(pattern)
	sp := splitPath(path)
	if len(pp) != len(sp) {
		return nil, false
	}
	params := map[string]string{}
	for i := range pp {
		p := pp[i]
		s := sp[i]
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(p, "{"), "}")
			if name == "" || s == "" {
				return nil, false
			}
			params[name] = s
			continue
		}
		if p != s {
			return nil, false
		}
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
