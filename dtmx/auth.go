package dtmx

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/aisphereio/kernel/errorx"
)

const (
	// BranchAuthHeader is the default header used to authenticate DTM -> service
	// branch callbacks. dtmx/dtm sends this header when Config.BranchSecret is
	// configured, and BranchAuthMiddleware validates it on callback handlers.
	BranchAuthHeader = "X-Kernel-DTM-Branch-Token"
)

var ErrBranchUnauthorized = errorx.Unauthorized(CodeBranchUnauthorized, "dtm branch callback is unauthorized")

// ValidateBranchToken validates a received DTM branch token using constant-time
// comparison. Empty expected means branch authentication is disabled; this is
// allowed for local development, but production services should normally set a
// BranchSecret or protect branch endpoints with mTLS/private networking.
func ValidateBranchToken(expected, got string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	got = strings.TrimSpace(got)
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		return ErrBranchUnauthorized
	}
	return nil
}

// ValidateBranchRequest validates the default branch-auth header on an HTTP
// request. It is intentionally net/http only so kernel transport adapters and
// business services can use it without importing the DTM SDK.
func ValidateBranchRequest(r *http.Request, secret string) error {
	if r == nil {
		return ErrBranchUnauthorized
	}
	return ValidateBranchToken(secret, r.Header.Get(BranchAuthHeader))
}

// BranchAuthMiddleware wraps internal DTM branch endpoints with shared-secret
// validation. It writes 401 and does not call next when validation fails.
func BranchAuthMiddleware(secret string, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := ValidateBranchRequest(r, secret); err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
