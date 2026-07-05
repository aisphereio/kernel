// Package authn provides transport middleware for Kernel authentication.
package authn

import (
	"context"
	"strings"

	rootauthn "github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/middleware"
	transport "github.com/aisphereio/kernel/transportx"
)

// CredentialExtractor extracts an authn credential from transport headers.
type CredentialExtractor func(transport.Header) (rootauthn.Credential, bool)

// Option configures Server.
type Option func(*options)

type options struct {
	authenticator  rootauthn.Authenticator
	extractor      CredentialExtractor
	allowAnonymous bool
}

// WithAuthenticator configures the authenticator used by Server.
func WithAuthenticator(a rootauthn.Authenticator) Option {
	return func(o *options) { o.authenticator = a }
}

// WithCredentialExtractor configures how credentials are extracted.
func WithCredentialExtractor(extractor CredentialExtractor) Option {
	return func(o *options) {
		if extractor != nil {
			o.extractor = extractor
		}
	}
}

// WithAllowAnonymous lets unauthenticated requests continue as anonymous.
func WithAllowAnonymous(allow bool) Option {
	return func(o *options) { o.allowAnonymous = allow }
}

// Server authenticates the caller and stores the normalized principal in both
// authn and contextx contexts so old and new Kernel packages see the same
// identity.
func Server(opts ...Option) middleware.Middleware {
	o := &options{
		extractor: BearerExtractor,
	}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, _ := transport.FromServerContext(ctx)
			cred, ok := o.extractor(nil)
			if tr != nil {
				cred, ok = o.extractor(tr.RequestHeader())
			}
			if !ok {
				if o.allowAnonymous {
					return next(withPrincipal(ctx, rootauthn.Anonymous()), req)
				}
				return nil, rootauthn.ErrMissingCredential("")
			}
			if o.authenticator == nil {
				return nil, rootauthn.ErrIdentityBackendFailed("authenticator is not configured", nil)
			}
			principal, err := o.authenticator.Authenticate(ctx, cred)
			if err != nil {
				return nil, err
			}
			return next(withPrincipal(ctx, principal), req)
		}
	}
}

// BearerExtractor extracts an Authorization: Bearer <token> credential.
func BearerExtractor(h transport.Header) (rootauthn.Credential, bool) {
	if h == nil {
		return rootauthn.Credential{}, false
	}
	return rootauthn.BearerCredential(h.Get("Authorization"))
}

// HeaderExtractor extracts a token directly from a named header. It is useful
// for dev-token, API-key and mTLS adapter tests where the upstream identity has
// already been verified.
func HeaderExtractor(headerName, scheme string) CredentialExtractor {
	return func(h transport.Header) (rootauthn.Credential, bool) {
		if h == nil {
			return rootauthn.Credential{}, false
		}
		token := strings.TrimSpace(h.Get(headerName))
		if token == "" {
			return rootauthn.Credential{}, false
		}
		return rootauthn.Credential{Scheme: scheme, Token: token}, true
	}
}

// TrustedHeaderExtractor extracts Gateway-verified identity headers. It is for
// internal services running in gateway_trusted mode; external traffic must never
// be allowed to set these headers directly.
func TrustedHeaderExtractor(h transport.Header) (rootauthn.Credential, bool) {
	return GatewayTrustedExtractor(rootauthn.InternalServiceTokenHeader)(h)
}

// GatewayTrustedExtractor extracts both Gateway-injected Principal headers and
// the optional Gateway-to-backend internal token header. The authenticator
// decides whether the internal token is required.
func GatewayTrustedExtractor(internalTokenHeader string) CredentialExtractor {
	return func(h transport.Header) (rootauthn.Credential, bool) {
		if h == nil {
			return rootauthn.Credential{}, false
		}
		metadata := map[string]string{}
		for _, key := range rootauthn.TrustedHeaderNames() {
			if value := strings.TrimSpace(h.Get(key)); value != "" {
				metadata[key] = value
			}
		}
		if header := strings.TrimSpace(internalTokenHeader); header != "" {
			if value := strings.TrimSpace(h.Get(header)); value != "" {
				metadata[header] = value
			}
		}
		if value := strings.TrimSpace(h.Get(rootauthn.InternalServiceTokenHeader)); value != "" {
			metadata[rootauthn.InternalServiceTokenHeader] = value
		}
		if _, ok := rootauthn.PrincipalFromTrustedHeaders(metadata); !ok {
			return rootauthn.Credential{}, false
		}
		return rootauthn.Credential{Scheme: rootauthn.CredentialGatewayTrusted, Token: metadata[rootauthn.TrustedHeaderSubject], Metadata: metadata}, true
	}
}

// HybridBearerOrGatewayTrustedExtractor extracts a bearer JWT when present;
// otherwise it falls back to Gateway trusted Principal headers. This is intended
// for migration/testing. Prefer a strict single mode in production.
func HybridBearerOrGatewayTrustedExtractor(internalTokenHeader string) CredentialExtractor {
	trusted := GatewayTrustedExtractor(internalTokenHeader)
	return func(h transport.Header) (rootauthn.Credential, bool) {
		if cred, ok := BearerExtractor(h); ok {
			return cred, true
		}
		return trusted(h)
	}
}

func withPrincipal(ctx context.Context, p rootauthn.Principal) context.Context {
	p = p.Normalize()
	ctx = rootauthn.ContextWithPrincipal(ctx, p)
	if p.IsAuthenticated() {
		ctx = contextx.WithAuthnPrincipal(ctx, p)
		if p.TenantID != "" {
			ctx = contextx.WithTenant(ctx, p.TenantID)
		}
	}
	return ctx
}
