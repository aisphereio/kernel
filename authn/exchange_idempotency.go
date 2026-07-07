package authn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/aisphereio/kernel/errorx"
)

const (
	CodeAuthorizationCodeUsed  = errorx.Code("AUTHN_AUTHORIZATION_CODE_USED")
	CodeAuthExchangeInProgress = errorx.Code("AUTHN_AUTH_EXCHANGE_IN_PROGRESS")
)

// ErrAuthorizationCodeUsed reports that an OAuth authorization code has already
// been consumed by the identity provider. Authorization codes are one-time
// credentials; callers must restart the login flow instead of retrying the same
// code unless an idempotency cache returns the first successful result.
func ErrAuthorizationCodeUsed(message string, cause error) error {
	if message == "" {
		message = "authorization code has already been used"
	}
	if cause == nil {
		return errorx.Conflict(CodeAuthorizationCodeUsed, message)
	}
	return errorx.Wrap(cause, CodeAuthorizationCodeUsed, errorx.WithMessage(message))
}

// ErrAuthExchangeInProgress reports that another request is already exchanging
// the same OAuth authorization code and the caller should wait or retry without
// reusing a different code.
func ErrAuthExchangeInProgress(message string) error {
	if message == "" {
		message = "authorization code exchange is already in progress"
	}
	return errorx.Conflict(CodeAuthExchangeInProgress, message)
}

// IsAuthorizationCodeUsed returns true for Kernel-normalized and common OAuth2
// provider errors indicating that an authorization code was already consumed.
func IsAuthorizationCodeUsed(err error) bool {
	if err == nil {
		return false
	}
	if errorx.CodeOf(err) == CodeAuthorizationCodeUsed {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid_grant") && strings.Contains(msg, "code") && (strings.Contains(msg, "used") || strings.Contains(msg, "consumed"))
}

// IdempotentExchangeConfig controls the in-memory OAuth authorization-code
// exchange protection used by NewIdempotentTokenService. It intentionally keeps
// only very short-lived results because exchanged token sets are sensitive.
type IdempotentExchangeConfig struct {
	// Enabled disables the wrapper when explicitly false only if all other fields
	// are zero. The zero config is enabled with safe short defaults.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// SuccessTTL caches a successful exchange result for duplicate callback calls,
	// for example React StrictMode double effects, browser refresh, or network
	// retry after the first exchange succeeded at the identity provider.
	SuccessTTL time.Duration `json:"success_ttl_ns" yaml:"success_ttl_ns"`

	// MaxEntries bounds the in-memory result table. When exceeded, expired entries
	// are removed first and then an arbitrary oldest-scan subset is evicted.
	MaxEntries int `json:"max_entries" yaml:"max_entries"`
}

func (c IdempotentExchangeConfig) Normalized() IdempotentExchangeConfig {
	if !c.Enabled && c.SuccessTTL == 0 && c.MaxEntries == 0 {
		c.Enabled = true
	}
	if c.SuccessTTL <= 0 {
		c.SuccessTTL = 2 * time.Minute
	}
	if c.MaxEntries <= 0 {
		c.MaxEntries = 2048
	}
	return c
}

// NewIdempotentTokenService wraps a TokenService so repeated ExchangeCode calls
// for the same code/state/redirect tuple are coalesced and short-term successful
// results are replayed instead of reusing the one-time authorization code at the
// identity provider.
func NewIdempotentTokenService(next TokenService, cfg IdempotentExchangeConfig) TokenService {
	if next == nil {
		return nil
	}
	cfg = cfg.Normalized()
	if !cfg.Enabled {
		return next
	}
	return &idempotentTokenService{next: next, cfg: cfg, entries: map[string]*exchangeEntry{}}
}

type idempotentTokenService struct {
	next    TokenService
	cfg     IdempotentExchangeConfig
	mu      sync.Mutex
	entries map[string]*exchangeEntry
}

type exchangeEntry struct {
	done      chan struct{}
	complete  bool
	expiresAt time.Time
	tokens    TokenSet
	principal Principal
	err       error
}

func (s *idempotentTokenService) ExchangeCode(ctx context.Context, req AuthCodeExchangeRequest) (TokenSet, Principal, error) {
	key := exchangeCodeKey(req)
	if key == "" {
		return s.next.ExchangeCode(ctx, req)
	}
	now := time.Now()

	s.mu.Lock()
	s.evictExpiredLocked(now)
	if entry, ok := s.entries[key]; ok {
		done := entry.done
		if entry.complete {
			if entry.err == nil && now.Before(entry.expiresAt) {
				tokens, principal := cloneExchangeResult(entry.tokens, entry.principal)
				s.mu.Unlock()
				return tokens, principal, nil
			}
			delete(s.entries, key)
		} else {
			s.mu.Unlock()
			select {
			case <-done:
				return s.ExchangeCode(ctx, req)
			case <-ctx.Done():
				return TokenSet{}, Principal{}, ctx.Err()
			}
		}
	}
	entry := &exchangeEntry{done: make(chan struct{})}
	s.entries[key] = entry
	s.enforceMaxEntriesLocked(now)
	s.mu.Unlock()

	tokens, principal, err := s.next.ExchangeCode(ctx, req)
	if IsAuthorizationCodeUsed(err) {
		err = ErrAuthorizationCodeUsed("authorization code has already been used", err)
	}

	s.mu.Lock()
	entry.complete = true
	entry.tokens, entry.principal = cloneExchangeResult(tokens, principal)
	entry.err = err
	if err == nil {
		entry.expiresAt = time.Now().Add(s.cfg.SuccessTTL)
	} else {
		delete(s.entries, key)
	}
	close(entry.done)
	s.mu.Unlock()
	return tokens, principal, err
}

func (s *idempotentTokenService) RefreshToken(ctx context.Context, req RefreshTokenRequest) (TokenSet, error) {
	return s.next.RefreshToken(ctx, req)
}

func (s *idempotentTokenService) VerifyToken(ctx context.Context, req VerifyTokenRequest) (Principal, error) {
	return s.next.VerifyToken(ctx, req)
}

func (s *idempotentTokenService) RevokeToken(ctx context.Context, req RevokeTokenRequest) error {
	return s.next.RevokeToken(ctx, req)
}

func exchangeCodeKey(req AuthCodeExchangeRequest) string {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return ""
	}
	parts := []string{
		code,
		strings.TrimSpace(req.State),
		strings.TrimSpace(req.RedirectURI),
		strings.TrimSpace(req.OrgID),
		strings.TrimSpace(req.AppID),
	}
	if req.Metadata != nil {
		parts = append(parts, strings.TrimSpace(req.Metadata["idempotency_key"]))
		parts = append(parts, strings.TrimSpace(req.Metadata["code_verifier"]))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (s *idempotentTokenService) evictExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if entry == nil {
			delete(s.entries, key)
			continue
		}
		if entry.complete && !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}

func (s *idempotentTokenService) enforceMaxEntriesLocked(now time.Time) {
	if len(s.entries) <= s.cfg.MaxEntries {
		return
	}
	s.evictExpiredLocked(now)
	for key, entry := range s.entries {
		if len(s.entries) <= s.cfg.MaxEntries {
			return
		}
		if entry != nil && !entry.complete {
			continue
		}
		delete(s.entries, key)
	}
}

func cloneExchangeResult(tokens TokenSet, principal Principal) (TokenSet, Principal) {
	tokens.Raw = cloneAttributeSet(tokens.Raw)
	principal.Audience = append([]string(nil), principal.Audience...)
	principal.Roles = append([]string(nil), principal.Roles...)
	principal.Groups = append([]string(nil), principal.Groups...)
	principal.Scopes = append([]string(nil), principal.Scopes...)
	principal.Attributes = cloneAttributeSet(principal.Attributes)
	return tokens, principal
}

func cloneAttributeSet(in AttributeSet) AttributeSet {
	if in == nil {
		return nil
	}
	out := make(AttributeSet, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

var _ TokenService = (*idempotentTokenService)(nil)
