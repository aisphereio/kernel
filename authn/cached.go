// Package authn cached.go — cache decorator for Authenticator + TokenService.
//
// CachedClient wraps an Authenticator + TokenService pair with a shared
// cachex-backed cache. Both Authenticate and VerifyToken share the same cache
// key for a given token, so a token verified via VerifyToken will be served
// from cache on the next Authenticate call (and vice versa) — important when
// middleware authenticates via Authenticator.Authenticate but business code
// calls TokenService.VerifyToken explicitly.
//
// Cache semantics:
//
//   - Hit:  return cached Principal without calling the inner provider.
//   - Miss: call inner provider, cache result on success.
//   - Invalidate(ctx, token): remove entry; call from RevokeToken, LogoutURL,
//     and any path that invalidates a session.
//
// Cache key: "authn:token:" + sha256_hex(token). We hash the token so cache
// dumps (Redis MONITOR, MEMORY USAGE, etc.) do not leak live bearer tokens.
//
// TTL: min(cfg.ttl, token_exp - now). A token that expires in 30s is cached
// for at most 30s even if cfg.ttl is 5m. This prevents serving a cached
// principal for an already-expired token.
//
// Best-effort: if cache.Get/Set/Del fails, the wrapper falls through to the
// inner provider. A cache outage never blocks authn.
//
// Limitations:
//
//   - The cache does NOT enforce revocation. If a token is revoked at the IdP
//     but Invalidate is not called, the cache will keep returning the cached
//     Principal until the TTL expires. Callers that need revocation semantics
//     must maintain a separate blacklist AND call Invalidate on revoke.
//   - The cache assumes the inner provider's verification is deterministic
//     for a given token within the TTL window. This is true for JWT-based
//     providers (Casdoor, OIDC) but may not be true for introspection-based
//     providers that check a server-side session table. For those, set TTL=0
//     or do not wrap.
package authn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/aisphereio/kernel/cachex"
)

// CachedClient wraps an Authenticator + TokenService + LogoutService with a
// shared cache.
//
// Construction:
//
//	client := casdoor.New(cfg)              // implements all three interfaces
//	cached := authn.NewCachedClient(client, client, client, cache,
//	    authn.WithCachedTTL(5*time.Minute))
//	_ = authn.Authenticator(cached)         // usable as Authenticator
//	_ = authn.TokenService(cached)          // usable as TokenService
//	_ = authn.LogoutService(cached)         // usable as LogoutService
//
// All three of `auth`, `tokens` and `logout` may point to the same object
// (typical for the casdoor Client which implements all three). They are
// accepted separately so callers can mix providers (e.g. JWT verifier for
// Authenticate, introspection endpoint for TokenService) if needed.
type CachedClient struct {
	auth   Authenticator
	tokens TokenService
	logout LogoutService
	cache  cachex.Cache
	ttl    time.Duration
}

// CachedOption configures a CachedClient.
type CachedOption func(*CachedClient)

// WithCachedTTL sets the maximum cache TTL. The effective TTL for a given
// token is min(this, token_exp - now). Default is 5 minutes. Set to 0 to
// effectively disable caching (every call goes to the inner provider).
func WithCachedTTL(ttl time.Duration) CachedOption {
	return func(c *CachedClient) { c.ttl = ttl }
}

// NewCachedClient wraps an Authenticator + TokenService + LogoutService with
// a cache.
//
// If cache is nil, the returned CachedClient delegates every call to the
// inner providers without any caching. This lets callers always go through
// CachedClient (no nil checks) even when caching is disabled.
//
// The `logout` parameter is accepted separately from `auth` and `tokens` so
// callers can mix providers. For the typical casdoor Client which implements
// all three, pass the same client for all three parameters.
func NewCachedClient(auth Authenticator, tokens TokenService, logout LogoutService, cache cachex.Cache, opts ...CachedOption) *CachedClient {
	c := &CachedClient{
		auth:   auth,
		tokens: tokens,
		logout: logout,
		cache:  cache,
		ttl:    5 * time.Minute,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// --- Authenticator ---

// Authenticate verifies the credential and returns the cached Principal on
// hit, or delegates to the inner Authenticator on miss.
func (c *CachedClient) Authenticate(ctx context.Context, credential Credential) (Principal, error) {
	if c == nil || c.cache == nil || shouldSkipCache(credential.Token) {
		return c.auth.Authenticate(ctx, credential)
	}
	key := tokenCacheKey(credential.Token)
	if cached, ok := c.lookup(ctx, key); ok {
		return cached, nil
	}
	principal, err := c.auth.Authenticate(ctx, credential)
	if err != nil {
		return Principal{}, err
	}
	c.store(ctx, key, principal)
	return principal, nil
}

// --- TokenService ---

// VerifyToken verifies the token and returns the cached Principal on hit,
// or delegates to the inner TokenService on miss.
func (c *CachedClient) VerifyToken(ctx context.Context, req VerifyTokenRequest) (Principal, error) {
	if c == nil || c.cache == nil || shouldSkipCache(req.Token) {
		return c.tokens.VerifyToken(ctx, req)
	}
	key := tokenCacheKey(req.Token)
	if cached, ok := c.lookup(ctx, key); ok {
		return cached, nil
	}
	principal, err := c.tokens.VerifyToken(ctx, req)
	if err != nil {
		return Principal{}, err
	}
	c.store(ctx, key, principal)
	return principal, nil
}

// ExchangeCode is a one-shot operation — never cached. Delegates directly.
func (c *CachedClient) ExchangeCode(ctx context.Context, req AuthCodeExchangeRequest) (TokenSet, Principal, error) {
	return c.tokens.ExchangeCode(ctx, req)
}

// RefreshToken is a one-shot operation — never cached. Delegates directly.
func (c *CachedClient) RefreshToken(ctx context.Context, req RefreshTokenRequest) (TokenSet, error) {
	return c.tokens.RefreshToken(ctx, req)
}

// RevokeToken delegates to the inner TokenService and, on success, removes
// the token from the cache. On failure, the cache entry is left untouched
// (the inner provider may have rejected the request, in which case there is
// nothing to invalidate; or the inner provider is UNIMPLEMENTED, in which
// case the caller — typically a hub-level repo — should call Invalidate
// explicitly after updating its own blacklist).
func (c *CachedClient) RevokeToken(ctx context.Context, req RevokeTokenRequest) error {
	err := c.tokens.RevokeToken(ctx, req)
	if err == nil {
		_ = c.Invalidate(ctx, req.Token)
	}
	return err
}

// --- LogoutService ---

// BuildLogoutURL delegates to the inner LogoutService without caching.
// Logout URLs are one-shot redirect targets that include per-request state
// and should not be cached.
func (c *CachedClient) BuildLogoutURL(ctx context.Context, req LogoutURLRequest) (LogoutURL, error) {
	return c.logout.BuildLogoutURL(ctx, req)
}

// --- cache management ---

// Invalidate removes a token's cached Principal. Call this from revocation
// flows that bypass RevokeToken (e.g. hub's local-blacklist fallback when
// the IdP's RevokeToken is UNIMPLEMENTED) so the next Authenticate/VerifyToken
// re-queries the inner provider (or, more usefully, the hub blacklist layer
// rejects the call before it reaches the cache).
func (c *CachedClient) Invalidate(ctx context.Context, token string) error {
	if c == nil || c.cache == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	return c.cache.Del(ctx, tokenCacheKey(token))
}

// lookup returns the cached Principal for key if present and not expired.
// On any cache error or expiry, returns (zero, false).
func (c *CachedClient) lookup(ctx context.Context, key string) (Principal, bool) {
	var cached Principal
	if err := c.cache.Get(ctx, key, &cached); err != nil {
		return Principal{}, false
	}
	if cached.Expired(time.Now()) {
		_ = c.cache.Del(ctx, key)
		return Principal{}, false
	}
	return cached, true
}

// store writes principal to cache with TTL = min(c.ttl, principal exp - now).
// Best-effort: cache write errors are silently ignored (the principal is
// still returned to the caller; only future cache hits miss).
func (c *CachedClient) store(ctx context.Context, key string, principal Principal) {
	ttl := c.effectiveTTL(principal)
	if ttl <= 0 {
		return
	}
	_ = c.cache.Set(ctx, key, principal, ttl)
}

func (c *CachedClient) effectiveTTL(p Principal) time.Duration {
	if !p.ExpiresAt.IsZero() {
		remaining := time.Until(p.ExpiresAt)
		if remaining > 0 && remaining < c.ttl {
			return remaining
		}
	}
	return c.ttl
}

func shouldSkipCache(token string) bool {
	return strings.TrimSpace(token) == ""
}

// tokenCacheKey returns the cache key for a token. The token is hashed with
// SHA-256 and hex-encoded so cache keys are URL-safe, fixed-length, and do
// not leak the raw bearer token in cache infrastructure logs.
//
// The "authn:token:" prefix namespaces authn cache entries from other cache
// users (e.g. rate limit counters, session data) within the same cachex
// instance. Combined with cachex's own KeyPrefix config, the full Redis key
// becomes "<service-prefix>:authn:token:<sha256>".
func tokenCacheKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "authn:token:" + hex.EncodeToString(sum[:])
}
