// Package oidcx provides provider-neutral OIDC/JWKS JWT verification.
//
// It is intentionally independent from Casdoor's SDK. Gateway and backend
// services can use this package to verify Casdoor, Keycloak, Logto or any
// OIDC provider that exposes discovery + JWKS. Provider-specific behavior is
// handled by configurable claim names and a PrincipalMapper.
package oidcx

import (
	"net/http"
	"strings"
	"time"

	"github.com/aisphereio/kernel/logx"
)

// Config controls OIDC/JWKS token verification.
type Config struct {
	// Provider is copied to authn.Principal.Provider. Use "casdoor" for
	// Casdoor-backed deployments. Defaults to "oidc".
	Provider string `json:"provider" yaml:"provider"`

	// Issuer is the trusted issuer expected in the JWT iss claim. When
	// DiscoveryURL is set and Issuer is empty, the discovery issuer is used.
	Issuer string `json:"issuer" yaml:"issuer"`

	// DiscoveryURL is the OIDC discovery endpoint, for example
	// https://casdoor.example.com/.well-known/openid-configuration.
	DiscoveryURL string `json:"discovery_url" yaml:"discovery_url"`

	// JWKSURL is the public key endpoint. It can be provided directly or read
	// from discovery_url.jwks_uri.
	JWKSURL string `json:"jwks_url" yaml:"jwks_url"`

	// Audience lists accepted aud values. At least one must match when non-empty.
	Audience []string `json:"audience" yaml:"audience"`

	// AllowedAlgs restricts accepted JWT signing algorithms.
	// Defaults to RS256, RS512, ES256, ES512.
	AllowedAlgs []string `json:"allowed_algs" yaml:"allowed_algs"`

	// AllowedOwners optionally restricts an owner/organization claim. Casdoor
	// uses owner. Other providers can map this via OwnerClaim.
	AllowedOwners []string `json:"allowed_owners" yaml:"allowed_owners"`

	// Claim mapping. Defaults are Casdoor-compatible while remaining generic.
	SubjectIDClaim string `json:"subject_id_claim" yaml:"subject_id_claim"`
	UsernameClaim  string `json:"username_claim" yaml:"username_claim"`
	NameClaim      string `json:"name_claim" yaml:"name_claim"`
	EmailClaim     string `json:"email_claim" yaml:"email_claim"`
	OwnerClaim     string `json:"owner_claim" yaml:"owner_claim"`
	GroupsClaim    string `json:"groups_claim" yaml:"groups_claim"`
	RolesClaim     string `json:"roles_claim" yaml:"roles_claim"`
	ScopesClaim    string `json:"scopes_claim" yaml:"scopes_claim"`

	// CacheTTL controls in-process JWKS cache lifetime.
	JWKSCacheTTL time.Duration `json:"jwks_cache_ttl_ns" yaml:"jwks_cache_ttl_ns"`

	// ClockSkew is the accepted leeway for exp/nbf/iat validation.
	ClockSkew time.Duration `json:"clock_skew_ns" yaml:"clock_skew_ns"`

	HTTPClient *http.Client `json:"-" yaml:"-"`
	Logger     logx.Logger  `json:"-" yaml:"-"`
}

func (c Config) Normalized() Config {
	c.Provider = strings.TrimSpace(c.Provider)
	if c.Provider == "" {
		c.Provider = "oidc"
	}
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.DiscoveryURL = strings.TrimSpace(c.DiscoveryURL)
	c.JWKSURL = strings.TrimSpace(c.JWKSURL)
	c.Audience = normalizeStrings(c.Audience)
	c.AllowedAlgs = normalizeStrings(c.AllowedAlgs)
	if len(c.AllowedAlgs) == 0 {
		c.AllowedAlgs = []string{"RS256", "RS512", "ES256", "ES512"}
	}
	c.AllowedOwners = normalizeStrings(c.AllowedOwners)
	if c.SubjectIDClaim == "" {
		c.SubjectIDClaim = "sub"
	}
	if c.UsernameClaim == "" {
		c.UsernameClaim = "name"
	}
	if c.NameClaim == "" {
		c.NameClaim = "displayName"
	}
	if c.EmailClaim == "" {
		c.EmailClaim = "email"
	}
	if c.OwnerClaim == "" {
		c.OwnerClaim = "owner"
	}
	if c.GroupsClaim == "" {
		c.GroupsClaim = "groups"
	}
	if c.RolesClaim == "" {
		c.RolesClaim = "roles"
	}
	if c.ScopesClaim == "" {
		c.ScopesClaim = "scope"
	}
	if c.JWKSCacheTTL <= 0 {
		c.JWKSCacheTTL = 10 * time.Minute
	}
	if c.ClockSkew <= 0 {
		c.ClockSkew = 60 * time.Second
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	if c.Logger == nil {
		c.Logger = logx.DefaultLogger()
	}
	return c
}

func normalizeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
