package callback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aisphereio/kernel/authn"
)

type fakeLoginService struct {
	last authn.CallbackRequest
}

func (f *fakeLoginService) BuildLoginURL(ctx context.Context, req authn.LoginURLRequest) (authn.LoginURL, error) {
	return authn.LoginURL{URL: "http://login.example", RedirectURI: req.RedirectURI, State: req.State}, nil
}

func (f *fakeLoginService) HandleCallback(ctx context.Context, req authn.CallbackRequest) (authn.CallbackResult, error) {
	f.last = req
	return authn.CallbackResult{
		Tokens:    authn.TokenSet{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)},
		Principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser, Username: "alice", OrgID: req.OrgID}.Normalize(),
		State:     req.State,
		Provider:  "fake",
	}, nil
}

func TestHandlerSuccess(t *testing.T) {
	login := &fakeLoginService{}
	var called bool
	h := &Handler{
		Login:         login,
		RedirectURI:   "http://localhost:3000/callback",
		ExpectedState: "s1",
		OrgID:         "org1",
		OnSuccess: func(ctx context.Context, result authn.CallbackResult) error {
			called = true
			if result.Principal.Username != "alice" {
				t.Fatalf("username=%q", result.Principal.Username)
			}
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/callback?code=c1&state=s1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("success hook was not called")
	}
	if login.last.Code != "c1" || login.last.ExpectedState != "s1" || login.last.RedirectURI == "" || login.last.OrgID != "org1" {
		t.Fatalf("callback request not propagated: %+v", login.last)
	}
}

func TestHandlerRejectsProviderError(t *testing.T) {
	h := &Handler{Login: &fakeLoginService{}}
	req := httptest.NewRequest(http.MethodGet, "/callback?error=access_denied", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRejectsMissingCode(t *testing.T) {
	h := &Handler{Login: &fakeLoginService{}}
	req := httptest.NewRequest(http.MethodGet, "/callback?state=s1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandlerRedirectsMissingCodeWhenLoginURLIsSet(t *testing.T) {
	h := &Handler{Login: &fakeLoginService{}, LoginURL: "http://login.example/start"}
	req := httptest.NewRequest(http.MethodGet, "/callback?state=s1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "http://login.example/start" {
		t.Fatalf("Location=%q", got)
	}
}
