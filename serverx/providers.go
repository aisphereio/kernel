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
	"github.com/aisphereio/kernel/securityx"
)

// RuntimeProviders is the provider-neutral server wiring contract used by
// Kernel services. Concrete engines such as Casdoor and SpiceDB must be adapted
// into provider-neutral Kernel interfaces before they enter serverx. Business
// services should not import provider SDKs directly.
type RuntimeProviders struct {
	// Security is the preferred config-derived runtime produced by
	// securityx.NewRuntime. It supplies AuthnBoundary and SkipPolicyResolver while
	// leaving middleware order to serverx/autowire.
	Security *securityx.Runtime

	Access accessx.Providers

	// AccessGuard allows existing services that already own an accessx.Guard to
	// join the unified serverx/autowire path without rebuilding Providers.
	// Prefer Access for new code.
	AccessGuard *accessx.Guard

	// AccessResolver is normally generated from proto access policy. It maps a
	// request operation to an accessx.Check consumed by accessx.Guard.
	AccessResolver accessmw.Resolver

	// SkipPolicyResolver is an optional config-driven skip policy resolver.
	// securityx.Runtime normally supplies this from security.access. This field
	// is kept for tests and advanced overrides.
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

	// AuthnBoundary is the explicit automatic authn wiring object. Prefer setting
	// Security to the result of securityx.NewRuntime so authn and access skip
	// policy come from the same config subtree.
	AuthnBoundary *securityx.AuthnBoundaryRuntime
}

// ServerMiddleware builds the opinionated Kernel middleware chain from runtime
// providers. This is the preferred path for generated services.
func (p RuntimeProviders) ServerMiddleware() []middleware.Middleware {
	authnBoundary := p.AuthnBoundary
	skipPolicyResolver := p.SkipPolicyResolver
	if p.Security != nil {
		if authnBoundary == nil {
			authnBoundary = p.Security.AuthnBoundary
		}
		if skipPolicyResolver == nil && p.Security.SkipPolicyResolver != nil {
			skipPolicyResolver = accessmw.SkipPolicyResolver(p.Security.SkipPolicyResolver)
		}
	}

	opts := []autowire.ServerOption{autowire.WithContextInjection()}
	if p.RequestInfoResolver != nil {
		opts = append(opts, autowire.WithRequestInfoResolver(p.RequestInfoResolver))
	}
	if authnBoundary != nil && authnBoundary.Enabled() {
		opts = append(opts, autowire.WithAuthn(authnBoundary.Authenticator), autowire.WithAllowAnonymous(authnBoundary.Config.AllowAnonymous))
		if authnBoundary.CredentialExtractor != nil {
			opts = append(opts, autowire.WithCredentialExtractor(authnBoundary.CredentialExtractor))
		}
	} else if authenticator := p.Access.Authn.Authenticator(); authenticator != nil {
		opts = append(opts, autowire.WithAuthn(authenticator), autowire.WithAllowAnonymous(p.AllowAnonymous))
		if p.CredentialExtractor != nil {
			opts = append(opts, autowire.WithCredentialExtractor(p.CredentialExtractor))
		}
	}
	if p.AccessResolver != nil {
		guard := p.Access.Guard()
		if p.AccessGuard != nil {
			guard = *p.AccessGuard
		}
		opts = append(opts, autowire.WithAccess(guard, p.AccessResolver, skipPolicyResolver))
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
