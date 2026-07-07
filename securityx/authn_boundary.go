package securityx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/oidcx"
	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/middleware"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
)

const (
	// AuthnModeCasdoorJWT verifies the inbound Casdoor/OIDC JWT locally with
	// OIDC discovery + JWKS. Use it at Gateway and for high-security services
	// that intentionally verify the bearer token again.
	AuthnModeCasdoorJWT = "casdoor_jwt"

	// AuthnModeOIDCJWT is the provider-neutral spelling of casdoor_jwt.
	AuthnModeOIDCJWT = "oidc_jwt"

	// AuthnModePrincipalJWT verifies an IAM-issued Principal assertion JWT from
	// X-Aisphere-Principal-JWT. This is the preferred backend mode when Envoy
	// Gateway ExternalAuth delegates external authentication to IAM.
	AuthnModePrincipalJWT = "principal_jwt"

	// AuthnModeGatewayTrusted trusts Principal headers injected by Gateway, but
	// only after validating the Gateway -> backend internal-service-token.
	AuthnModeGatewayTrusted = "gateway_trusted"

	// AuthnModeHybrid first tries bearer JWT verification, then falls back to
	// Gateway trusted headers. It is useful during migration, not as the final
	// strict production default.
	AuthnModeHybrid = "hybrid"

	// AuthnModeDisabled installs no authn middleware.
	AuthnModeDisabled = "disabled"
)

// AuthnBoundaryConfig is the framework-level AuthN contract shared by Gateway
// and backend services. Business services should expose this structure in their
// config.yaml and then call NewAuthnBoundaryRuntime instead of manually parsing
// JWTs, trusted headers or internal-service-token headers.
type AuthnBoundaryConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Mode    string `json:"mode" yaml:"mode"`

	// Provider is descriptive. For Casdoor deployments use provider=casdoor and
	// mode=casdoor_jwt. The actual JWT verifier is OIDC/JWKS based and does not
	// require Casdoor client_secret.
	Provider string `json:"provider" yaml:"provider"`

	OIDC oidcx.Config `json:"oidc" yaml:"oidc"`

	// PrincipalJWT verifies IAM-issued Gateway -> backend identity assertions.
	// Business services only need mode=principal_jwt plus this shared secret.
	PrincipalJWT authn.PrincipalJWTConfig `json:"principal_jwt" yaml:"principal_jwt"`

	// InternalCall is required in gateway_trusted mode and is injected by
	// Gateway before forwarding requests to backend services.
	InternalCall authn.InternalServiceTokenConfig `json:"internal_call" yaml:"internal_call"`

	// CacheTTL enables optional token-hash -> Principal caching when a cache is
	// supplied. TTL is always capped by the JWT exp claim by authn.CachedAuthenticator.
	CacheTTL time.Duration `json:"cache_ttl_ns" yaml:"cache_ttl_ns"`

	AllowAnonymous bool `json:"allow_anonymous" yaml:"allow_anonymous"`
}

func (c AuthnBoundaryConfig) Normalized() AuthnBoundaryConfig {
	c.Mode = strings.TrimSpace(c.Mode)
	c.Provider = strings.TrimSpace(c.Provider)
	if c.Provider == "" {
		c.Provider = "casdoor"
	}
	if c.Mode == "" {
		if c.Enabled {
			c.Mode = AuthnModeGatewayTrusted
		} else {
			c.Mode = AuthnModeDisabled
		}
	}
	c.Mode = strings.ToLower(c.Mode)
	c.InternalCall = c.InternalCall.Normalized()
	c.PrincipalJWT = c.PrincipalJWT.Normalized()
	c.OIDC = c.OIDC.Normalized()
	return c
}

// AuthnBoundaryRuntime is the auto-wired AuthN result used by server boot.
// Gateway code uses Authenticator directly; backend services normally install
// Middleware so Principal is injected into context before business handlers run.
type AuthnBoundaryRuntime struct {
	Config              AuthnBoundaryConfig
	Authenticator       authn.Authenticator
	CredentialExtractor authnmw.CredentialExtractor
}

func (r AuthnBoundaryRuntime) Enabled() bool {
	mode := strings.ToLower(strings.TrimSpace(r.Config.Mode))
	return r.Config.Enabled && mode != "" && mode != AuthnModeDisabled
}

// ServerMiddleware returns the complete AuthN middleware for backend services.
// In principal_jwt mode it verifies IAM-issued Principal JWTs. In
// gateway_trusted mode it validates internal-service-token and restores Principal
// from trusted headers. In casdoor_jwt mode it verifies bearer/cookie-forwarded
// JWTs with OIDC/JWKS.
func (r AuthnBoundaryRuntime) ServerMiddleware() []middleware.Middleware {
	if !r.Enabled() || r.Authenticator == nil {
		return nil
	}
	opts := []authnmw.Option{
		authnmw.WithAuthenticator(r.Authenticator),
		authnmw.WithAllowAnonymous(r.Config.AllowAnonymous),
	}
	if r.CredentialExtractor != nil {
		opts = append(opts, authnmw.WithCredentialExtractor(r.CredentialExtractor))
	}
	return []middleware.Middleware{authnmw.Server(opts...)}
}

// NewAuthnBoundaryRuntime builds the framework AuthN runtime from config. cache
// is optional; pass nil to use only the built-in JWKS cache.
func NewAuthnBoundaryRuntime(ctx context.Context, cfg AuthnBoundaryConfig, cache cachex.Cache) (*AuthnBoundaryRuntime, error) {
	_ = ctx
	cfg = cfg.Normalized()
	if !cfg.Enabled || cfg.Mode == AuthnModeDisabled {
		return &AuthnBoundaryRuntime{Config: cfg}, nil
	}

	rt := &AuthnBoundaryRuntime{Config: cfg}
	switch cfg.Mode {
	case AuthnModeCasdoorJWT, AuthnModeOIDCJWT:
		a, err := newOIDCAuthenticator(cfg, cache)
		if err != nil {
			return nil, err
		}
		rt.Authenticator = a
		rt.CredentialExtractor = authnmw.BearerExtractor
	case AuthnModePrincipalJWT:
		rt.Authenticator = authn.PrincipalJWTAuthenticator{Config: cfg.PrincipalJWT}
		rt.CredentialExtractor = authnmw.HeaderExtractor(cfg.PrincipalJWT.Header, authn.CredentialPrincipalJWT)
	case AuthnModeGatewayTrusted:
		rt.Authenticator = authn.TrustedHeaderAuthenticator{InternalToken: cfg.InternalCall}
		rt.CredentialExtractor = authnmw.GatewayTrustedExtractor(cfg.InternalCall.Header())
	case AuthnModeHybrid:
		a, err := newOIDCAuthenticator(cfg, cache)
		if err != nil {
			return nil, err
		}
		rt.Authenticator = authn.CompositeAuthenticator{
			Primary:   a,
			Secondary: authn.TrustedHeaderAuthenticator{InternalToken: cfg.InternalCall},
		}
		rt.CredentialExtractor = authnmw.HybridBearerOrGatewayTrustedExtractor(cfg.InternalCall.Header())
	default:
		return nil, fmt.Errorf("securityx: unsupported authn.mode %q", cfg.Mode)
	}
	return rt, nil
}

func newOIDCAuthenticator(cfg AuthnBoundaryConfig, cache cachex.Cache) (authn.Authenticator, error) {
	verifier, err := oidcx.New(cfg.OIDC)
	if err != nil {
		return nil, fmt.Errorf("securityx: oidc verifier: %w", err)
	}
	var a authn.Authenticator = verifier
	if cache != nil && cfg.CacheTTL > 0 {
		a = authn.NewCachedAuthenticator(a, cache, authn.WithAuthenticatorCacheTTL(cfg.CacheTTL))
	}
	return a, nil
}
