package gatewayx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
	httpx "github.com/aisphereio/kernel/transportx/http"
)

const DefaultSessionCookie = "AISP_SESSION"

// Session is a browser/BFF login session. Access and refresh tokens are stored
// server-side so the browser only receives an opaque HttpOnly cookie.
type Session struct {
	ID           string
	Principal    authn.Principal
	AccessToken  string
	RefreshToken string
	IDToken      string
	CSRFToken    string
	CreatedAt    time.Time
	LastSeenAt   time.Time
	ExpiresAt    time.Time
	Attributes   map[string]any
}

// Expired reports whether the session is expired at now.
func (s Session) Expired(now time.Time) bool {
	return s.ID == "" || (!s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt))
}

// SessionStore persists sessions.
type SessionStore interface {
	Get(ctx context.Context, id string) (Session, error)
	Set(ctx context.Context, session Session) error
	Delete(ctx context.Context, id string) error
}

// MemorySessionStore is a development and test session store. Production
// gateways should implement SessionStore with Redis.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

// NewMemorySessionStore creates an in-memory store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: make(map[string]Session)}
}

func (s *MemorySessionStore) Get(_ context.Context, id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return Session{}, fmt.Errorf("session not found")
	}
	if sess.Expired(time.Now()) {
		return Session{}, fmt.Errorf("session expired")
	}
	return sess, nil
}

func (s *MemorySessionStore) Set(_ context.Context, session Session) error {
	if session.ID == "" {
		return fmt.Errorf("session id is required")
	}
	now := time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.LastSeenAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *MemorySessionStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

// NewSessionID returns a URL-safe opaque session id.
func NewSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// SessionConfig configures SessionMiddleware.
type SessionConfig struct {
	Store      SessionStore
	CookieName string
	Required   bool
	Touch      bool
}

// SessionMiddleware loads a server-side session and injects authn.Principal into
// request context. Set Required=false for routes that can be anonymous.
func SessionMiddleware(conf SessionConfig) httpx.FilterFunc {
	cookieName := conf.CookieName
	if cookieName == "" {
		cookieName = DefaultSessionCookie
	}
	required := conf.Required
	store := conf.Store
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				http.Error(w, "session store unavailable", http.StatusInternalServerError)
				return
			}
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				if required {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			sess, err := store.Get(r.Context(), cookie.Value)
			if err != nil || !sess.Principal.IsAuthenticated() {
				if required {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if conf.Touch {
				sess.LastSeenAt = time.Now()
				_ = store.Set(r.Context(), sess)
			}
			ctx := authn.ContextWithPrincipal(r.Context(), sess.Principal)
			ctx = contextx.WithAuthnPrincipal(ctx, sess.Principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetSessionCookie writes the browser session cookie.
func SetSessionCookie(w http.ResponseWriter, name string, id string, expires time.Time, secure bool) {
	if name == "" {
		name = DefaultSessionCookie
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie clears the browser session cookie.
func ClearSessionCookie(w http.ResponseWriter, name string, secure bool) {
	if name == "" {
		name = DefaultSessionCookie
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
