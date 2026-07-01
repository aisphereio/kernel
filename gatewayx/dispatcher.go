package gatewayx

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/errorx"
)

// DispatchRequest is a transport-neutral request used by validation tests and
// future Gateway proxy implementations. Real HTTP handlers should adapt to this
// shape after route matching.
type DispatchRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    any
}

// DispatchResponse is a transport-neutral response from an upstream service.
type DispatchResponse struct {
	Status  int
	Headers map[string]string
	Body    any
}

// UpstreamInvoker invokes the selected upstream. HTTP reverse proxy, gRPC
// transcoding and in-memory validation dispatchers can all implement it.
type UpstreamInvoker interface {
	Invoke(ctx context.Context, match RouteMatch, req DispatchRequest, target string) (DispatchResponse, error)
}

// Dispatcher combines route registry, matcher snapshot, boundary authn policy
// and upstream invocation.
type Dispatcher struct {
	Registry RouteRegistry
	Hosts    StaticHosts
	Invoker  UpstreamInvoker
}

func NewDispatcher(registry RouteRegistry, hosts StaticHosts, invoker UpstreamInvoker) Dispatcher {
	return Dispatcher{Registry: registry, Hosts: hosts, Invoker: invoker}
}

func (d Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (DispatchResponse, error) {
	if d.Registry == nil {
		return DispatchResponse{}, fmt.Errorf("gatewayx: route registry is not configured")
	}
	matcher := NewMatcher(d.Registry.ListRoutes())
	match, ok := matcher.Match(req.Method, req.Path)
	if !ok {
		return DispatchResponse{Status: 404}, errorx.NotFound("ROUTE_NOT_FOUND", "gateway route not found")
	}
	if match.Route.Gateway.Exposure.String() == "INTERNAL" {
		return DispatchResponse{Status: 404}, errorx.NotFound("ROUTE_NOT_FOUND", "gateway route not found")
	}
	if requiresAuthorization(match.Route.Gateway) && bearer(req.Headers) == "" {
		return DispatchResponse{Status: 401}, errorx.Unauthorized("AUTHN_REQUIRED", "login required")
	}
	target, err := d.Hosts.Resolve(match.Route.Upstream)
	if err != nil {
		return DispatchResponse{Status: 503}, errorx.Unavailable("UPSTREAM_UNAVAILABLE", err.Error())
	}
	if d.Invoker == nil {
		return DispatchResponse{}, fmt.Errorf("gatewayx: upstream invoker is not configured")
	}
	return d.Invoker.Invoke(ctx, match, req, target)
}

func requiresAuthorization(p GatewayPolicy) bool {
	switch p.EffectiveAuthnMode() {
	case AuthnModeNone:
		return false
	default:
		return true
	}
}

func bearer(headers map[string]string) string {
	for k, v := range headers {
		if strings.EqualFold(k, "authorization") {
			v = strings.TrimSpace(v)
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				return strings.TrimSpace(v[len("Bearer "):])
			}
			return v
		}
	}
	return ""
}
