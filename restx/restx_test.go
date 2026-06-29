package restx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aisphereio/kernel/restx"
)

func TestCSRF(t *testing.T) {
	manager := restx.NewCSRFTokenManager([]byte("secret"), time.Hour)
	token, err := manager.Generate("sid", time.Now())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !manager.Verify("sid", token, time.Now()) {
		t.Fatal("token should verify")
	}

	mw := restx.CSRF(restx.CSRFConfig{Secret: []byte("secret")})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "AISP_SESSION", Value: "sid"})
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
}

func TestSecurityAndSanitizeHeaders(t *testing.T) {
	h := restx.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security header missing")
	}

	h = restx.SanitizeHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header["X-Bad"] = []string{"a\rb"}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid header") {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
}

func TestFiltersBuildsDefaultChain(t *testing.T) {
	conf := restx.DefaultMiddlewares()
	filters := restx.Filters(conf)
	if len(filters) == 0 {
		t.Fatal("expected filters")
	}
}
