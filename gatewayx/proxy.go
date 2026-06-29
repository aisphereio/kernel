package gatewayx

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
)

// ProxyConfig configures a lightweight reverse proxy for Gateway routes that
// should remain HTTP-to-HTTP instead of BFF logic -> RPC calls.
type ProxyConfig struct {
	Target      string
	StripPrefix string
	AddPrefix   string
	Audience    string
	JWT         *InternalJWT
	Timeout     time.Duration
}

// NewReverseProxy creates an HTTP reverse proxy with optional internal JWT
// injection. BFF endpoints should usually call RPC clients directly; this is for
// pass-through routes such as file downloads or legacy migration windows.
func NewReverseProxy(conf ProxyConfig) (http.Handler, error) {
	target, err := url.Parse(conf.Target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)
		if conf.StripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, conf.StripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		if conf.AddPrefix != "" {
			req.URL.Path = joinURLPath(conf.AddPrefix, req.URL.Path)
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.Header.Del("Cookie")
		if conf.JWT != nil {
			if principal, ok := authn.PrincipalFromContext(req.Context()); ok {
				token, err := conf.JWT.Sign(conf.Audience, principal, req.Header.Get("X-Request-Id"), req.Header.Get("X-Authz-Decision-Id"), time.Now())
				if err == nil {
					req.Header.Set("Authorization", "Bearer "+token)
				}
			}
		}
	}
	if conf.Timeout > 0 {
		proxy.Transport = &http.Transport{ResponseHeaderTimeout: conf.Timeout}
	}
	return proxy, nil
}

func joinURLPath(prefix, p string) string {
	prefix = strings.TrimRight(prefix, "/")
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return prefix
	}
	return prefix + "/" + p
}
