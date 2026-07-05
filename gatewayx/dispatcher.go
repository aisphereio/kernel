package gatewayx

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/authn"
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
	Registry      RouteRegistry
	Hosts         StaticHosts
	Invoker       UpstreamInvoker
	Authenticator authn.Authenticator
	InternalToken authn.InternalServiceTokenConfig
}

// DispatchOption configures Dispatcher without forcing all existing callers to
// understand Gateway authentication internals.
type DispatchOption func(*Dispatcher)

// WithAuthenticator lets Gateway verify external Casdoor JWTs locally before
// forwarding trusted identity headers to internal services.
func WithAuthenticator(a authn.Authenticator) DispatchOption {
	return func(d *Dispatcher) { d.Authenticator = a }
}

// WithInternalServiceToken configures the shared Gateway -> backend token that
// is injected after external JWT verification. It is stripped from inbound
// client requests before being injected, so clients cannot spoof the boundary.
func WithInternalServiceToken(cfg authn.InternalServiceTokenConfig) DispatchOption {
	return func(d *Dispatcher) { d.InternalToken = cfg.Normalized() }
}

func NewDispatcher(registry RouteRegistry, hosts StaticHosts, invoker UpstreamInvoker, opts ...DispatchOption) Dispatcher {
	d := Dispatcher{Registry: registry, Hosts: hosts, Invoker: invoker}
	for _, opt := range opts {
		if opt != nil {
			opt(&d)
		}
	}
	return d
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
	authn.StripGatewayControlledHeaders(req.Headers, d.InternalToken.Header())
	// The internal token is injected for every proxied route, including public
	// login/callback routes, so backends can optionally enforce "only Gateway may
	// call me" independently from end-user authentication.
	authn.InjectInternalServiceToken(req.Headers, d.InternalToken)
	if requiresAuthorization(match.Route.Gateway) {
		rawToken := bearer(req.Headers)
		if rawToken == "" {
			rawToken = cookie(req.Headers, "aisphere_access_token")
		}
		if rawToken == "" {
			return DispatchResponse{Status: 401}, errorx.Unauthorized("AUTHN_REQUIRED", "login required")
		}
		if d.Authenticator == nil {
			return DispatchResponse{Status: 503}, errorx.Unavailable("AUTHN_BACKEND_NOT_CONFIGURED", "gateway authenticator is not configured")
		}
		principal, err := d.Authenticator.Authenticate(ctx, authn.Credential{Scheme: authn.CredentialBearer, Token: rawToken})
		if err != nil {
			return DispatchResponse{Status: 401}, errorx.Unauthorized("AUTHN_INVALID", err.Error())
		}
		ctx = authn.ContextWithPrincipal(ctx, principal)
		authn.InjectTrustedHeaders(req.Headers, principal)
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

func cookie(headers map[string]string, name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	for k, v := range headers {
		if !strings.EqualFold(k, "cookie") {
			continue
		}
		for _, part := range strings.Split(v, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key, value, ok := strings.Cut(part, "=")
			if ok && strings.TrimSpace(key) == name {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}
