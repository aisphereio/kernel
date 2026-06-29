package gatewayx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/gatewayx"
	httpx "github.com/aisphereio/kernel/transportx/http"
)

func TestRegisterServices(t *testing.T) {
	srv := httpx.NewServer()
	root := srv.Route("/")
	err := gatewayx.RegisterServices(root, gatewayx.Service{
		Name:   "skills",
		Prefix: "/api/v1",
		Routes: []gatewayx.Route{{
			Method: http.MethodGet,
			Path:   "/skills/{name}",
			Handler: func(ctx httpx.Context) error {
				return ctx.String(http.StatusOK, ctx.Vars().Get("name"))
			},
		}},
	})
	if err != nil {
		t.Fatalf("RegisterServices: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/skills/demo", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "demo" {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
}

func TestSessionMiddleware(t *testing.T) {
	store := gatewayx.NewMemorySessionStore()
	_ = store.Set(nil, gatewayx.Session{
		ID:        "sid",
		Principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
		ExpiresAt: time.Now().Add(time.Hour),
	})

	mw := gatewayx.SessionMiddleware(gatewayx.SessionConfig{Store: store, Required: true})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.PrincipalFromContext(r.Context())
		if !ok || p.SubjectID != "u1" {
			t.Fatalf("principal missing: %#v", p)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: gatewayx.DefaultSessionCookie, Value: "sid"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestInternalJWT(t *testing.T) {
	jwt := gatewayx.NewInternalJWT("gateway", []byte("secret"), time.Minute)
	token, err := jwt.Sign("skill-service", authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser}, "req1", "dec1", time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	claims, err := jwt.Verify(token, "skill-service", time.Now())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Actor.SubjectID != "u1" || claims.RequestID != "req1" || claims.AuthzDecisionID != "dec1" {
		t.Fatalf("claims=%#v", claims)
	}
}
