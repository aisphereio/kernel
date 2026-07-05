package authn

import (
	"context"
	"strings"
	"time"

	"github.com/aisphereio/kernel/cachex"
)

// CachedAuthenticator caches verified Principals for bearer JWTs.
//
// It caches the *verification result*, not any decrypted secret. JWTs are not
// encrypted here; they are signed. Cache keys are SHA-256 hashes of the raw
// token and TTL is capped by the token's exp claim, so an expired token is never
// accepted from cache.
type CachedAuthenticator struct {
	inner Authenticator
	cache cachex.Cache
	ttl   time.Duration
}

type CachedAuthenticatorOption func(*CachedAuthenticator)

func WithAuthenticatorCacheTTL(ttl time.Duration) CachedAuthenticatorOption {
	return func(c *CachedAuthenticator) { c.ttl = ttl }
}

func NewCachedAuthenticator(inner Authenticator, cache cachex.Cache, opts ...CachedAuthenticatorOption) *CachedAuthenticator {
	c := &CachedAuthenticator{inner: inner, cache: cache, ttl: 5 * time.Minute}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

func (c *CachedAuthenticator) Authenticate(ctx context.Context, credential Credential) (Principal, error) {
	if c == nil || c.inner == nil {
		return Principal{}, ErrIdentityBackendFailed("authenticator is not configured", nil)
	}
	if c.cache == nil || strings.TrimSpace(credential.Token) == "" || c.ttl <= 0 {
		return c.inner.Authenticate(ctx, credential)
	}
	key := tokenCacheKey(credential.Token)
	var cached Principal
	if err := c.cache.Get(ctx, key, &cached); err == nil && !cached.Expired(time.Now()) && cached.IsAuthenticated() {
		return cached, nil
	}
	principal, err := c.inner.Authenticate(ctx, credential)
	if err != nil {
		return Principal{}, err
	}
	if ttl := c.effectiveTTL(principal); ttl > 0 {
		_ = c.cache.Set(ctx, key, principal, ttl)
	}
	return principal, nil
}

func (c *CachedAuthenticator) effectiveTTL(p Principal) time.Duration {
	ttl := c.ttl
	if !p.ExpiresAt.IsZero() {
		remaining := time.Until(p.ExpiresAt)
		if remaining <= 0 {
			return 0
		}
		if remaining < ttl {
			ttl = remaining
		}
	}
	return ttl
}
