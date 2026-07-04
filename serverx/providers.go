package serverx

import (
	"context"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/admissionx"
	"github.com/aisphereio/kernel/middleware"
	accessmw "github.com/aisphereio/kernel/middleware/access"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/middleware/autowire"
	"github.com/aisphereio/kernel/requestx"
)

// RuntimeProviders is the provider-neutral server wiring contract used by
// Kernel services. Concrete engines such as Casdoor and SpiceDB must be adapted
// into accessx.Providers before they enter serverx. Business services should not
// import provider SDKs directly.
type RuntimeProviders struct {
	Access accessx.Providers

	// AccessResolver is normally generated from proto access policy. It maps a
	// request operation to an accessx.Check consumed by accessx.Guard.
	AccessResolver accessmw.Resolver

	// SkipPolicyResolver is an optional config-driven skip policy resolver.
	// When set, it is checked before the AccessResolver — operations matching
	// public or skip policies are short-circuited without calling the resolver
	// or Guard.Require.
	//
	// This is typically created from accessx.NewSkipPolicyResolver(cfg) using
	// the service's security.access configuration.
	SkipPolicyResolver accessmw.SkipPolicyResolver

	// RequestInfoResolver is normally generated from proto metadata. It fills
	// requestx.Info before authn/authz/audit/metrics run.
	RequestInfoResolver requestx.Resolver

	// Admission contains Kubernetes-apiserver-style mutating/validating hooks.
	Admission admissionx.Chain

	// CredentialExtractor allows gateways and services to choose bearer,
	// token, API key, or other extraction strategies.
	CredentialExtractor authnmw.CredentialExtractor
	AllowAnonymous      bool
}

// ServerMiddleware builds the opinionated Kernel middleware chain from runtime
// providers. This is the preferred path for generated services.
func (p RuntimeProviders) ServerMiddleware() []middleware.Middleware {
	opts := []autowire.ServerOption{autowire.WithContextInjection()}
	if p.RequestInfoResolver != nil {
		opts = append(opts, autowire.WithRequestInfoResolver(p.RequestInfoResolver))
	}
	if authenticator := p.Access.Authn.Authenticator(); authenticator != nil {
		opts = append(opts, autowire.WithAuthn(authenticator), autowire.WithAllowAnonymous(p.AllowAnonymous))
		if p.CredentialExtractor != nil {
			opts = append(opts, autowire.WithCredentialExtractor(p.CredentialExtractor))
		}
	}
if p.AccessResolver != nil {
			opts = append(opts, autowire.WithAccess(p.Access.Guard(), p.AccessResolver, p.SkipPolicyResolver))
		}
	if !p.Admission.Empty() {
		opts = append(opts, autowire.WithAdmission(p.Admission))
	}
	return autowire.Server(opts...)
}

// WithRuntimeProviders installs provider-neutral authn/authz/audit/admission
// middleware into serverx.New. It exists so a service created by `kernel new`
// can be wired by providers instead of raw middleware.
func WithRuntimeProviders(p RuntimeProviders) Option {
	return WithServerMiddleware(p.ServerMiddleware()...)
}

// ServerMiddlewareFromProviders is a functional helper for tests and custom
// bootstrap code that do not need a full serverx.App yet.
func ServerMiddlewareFromProviders(_ context.Context, p RuntimeProviders) []middleware.Middleware {
	return p.ServerMiddleware()
}
