package restx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	httpx "github.com/aisphereio/kernel/transportx/http"
)

const defaultCSRFHeader = "X-CSRF-Token"

// CSRFConfig configures the double-submit/HMAC CSRF filter.
type CSRFConfig struct {
	Secret        []byte
	Header        string
	Cookie        string
	SessionCookie string
	SkipPaths     []string
	MaxAge        time.Duration
}

// CSRFTokenManager signs and verifies stateless CSRF tokens bound to a session
// id. It is intentionally small so Gateway/BFF code can use it before a Redis
// backed session store exists.
type CSRFTokenManager struct {
	secret []byte
	maxAge time.Duration
}

// NewCSRFTokenManager creates a token manager. secret must be non-empty in
// production. maxAge <= 0 disables token age checks.
func NewCSRFTokenManager(secret []byte, maxAge time.Duration) *CSRFTokenManager {
	return &CSRFTokenManager{secret: append([]byte(nil), secret...), maxAge: maxAge}
}

// Generate creates a token for sessionID.
func (m *CSRFTokenManager) Generate(sessionID string, now time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	issued := now.UTC().Format(time.RFC3339Nano)
	payload := sessionID + "\n" + issued + "\n" + base64.RawURLEncoding.EncodeToString(nonce)
	sig := m.sign(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig, nil
}

// Verify reports whether token is valid for sessionID.
func (m *CSRFTokenManager) Verify(sessionID, token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || sessionID == "" || len(m.secret) == 0 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	if !hmac.Equal([]byte(m.sign(payload)), []byte(parts[1])) {
		return false
	}
	fields := strings.Split(payload, "\n")
	if len(fields) != 3 || fields[0] != sessionID {
		return false
	}
	if m.maxAge > 0 {
		issued, err := time.Parse(time.RFC3339Nano, fields[1])
		if err != nil || now.Sub(issued) > m.maxAge || now.Before(issued.Add(-time.Minute)) {
			return false
		}
	}
	return true
}

func (m *CSRFTokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CSRF validates unsafe methods using a token bound to a session cookie. It is
// designed for browser Gateway/BFF endpoints that use HttpOnly cookie sessions.
func CSRF(conf CSRFConfig) httpx.FilterFunc {
	header := conf.Header
	if header == "" {
		header = defaultCSRFHeader
	}
	sessionCookie := conf.SessionCookie
	if sessionCookie == "" {
		sessionCookie = "AISP_SESSION"
	}
	manager := NewCSRFTokenManager(conf.Secret, conf.MaxAge)
	skip := make(map[string]struct{}, len(conf.SkipPaths))
	for _, p := range conf.SkipPaths {
		skip[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if csrfSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := skip[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := r.Cookie(sessionCookie)
			if err != nil || sess.Value == "" {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			token := r.Header.Get(header)
			if token == "" && conf.Cookie != "" {
				if c, err := r.Cookie(conf.Cookie); err == nil {
					token = c.Value
				}
			}
			if !manager.Verify(sess.Value, token, time.Now()) {
				http.Error(w, "invalid csrf token", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
