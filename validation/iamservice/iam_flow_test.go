package iamservice

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aisphereio/kernel/accessx"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/middleware"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/requestx"
	"github.com/aisphereio/kernel/serverx"
	transport "github.com/aisphereio/kernel/transportx"
)

type GetUserRequest struct{ ID string }
type LoginRequest struct{ Username, Password string }
type User struct{ ID, DisplayName, OrgID string }

type IAMService struct{ users map[string]User }

func (s IAMService) GetUser(ctx context.Context, req GetUserRequest) (User, error) {
	user, ok := s.users[req.ID]
	if !ok {
		return User{}, authz.ErrPermissionDenied("demo user not visible")
	}
	return user, nil
}

func TestIAMServiceKernelFlow(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	_, err := store.WriteRelationships(context.Background(), authz.Relationship{
		Resource: authz.ObjectRef{Type: "iam:user", ID: "u1"},
		Relation: "get",
		Subject:  authz.SubjectRef{Type: authn.SubjectTypeUser, ID: "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	audit := auditx.NewMemoryStore()
	providers := accessx.Providers{
		Authn: authn.ProviderSet{Runtime: fakeAuthnProvider{tokens: map[string]authn.Principal{
			"alice-token": {SubjectID: "alice", SubjectType: authn.SubjectTypeUser, OrgID: "aisphere", Username: "alice"},
			"bob-token":   {SubjectID: "bob", SubjectType: authn.SubjectTypeUser, OrgID: "aisphere", Username: "bob"},
		}}},
		Authz: authz.ProviderSet{Runtime: authz.NewMemoryAuthorizer(store)},
		Audit: audit,
	}
	chain := middleware.Chain(serverx.ServerMiddlewareFromProviders(context.Background(), serverx.RuntimeProviders{
		Access:              providers,
		RequestInfoResolver: iamRequestInfoResolver,
		AccessResolver:      iamAccessResolver,
		CredentialExtractor: authnmw.BearerExtractor,
	})...)
	svc := IAMService{users: map[string]User{"u1": {ID: "u1", DisplayName: "Alice", OrgID: "aisphere"}}}

	t.Run("missing token is rejected before business handler", func(t *testing.T) {
		called := false
		handler := chain(func(ctx context.Context, req any) (any, error) {
			called = true
			return nil, nil
		})
		_, err := handler(serverCtx(""), GetUserRequest{ID: "u1"})
		if err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("expected missing credential error, got %v", err)
		}
		if called {
			t.Fatal("business handler should not run without token")
		}
	})

	t.Run("authenticated but unauthorized returns 403-like authz error", func(t *testing.T) {
		called := false
		handler := chain(func(ctx context.Context, req any) (any, error) {
			called = true
			return svc.GetUser(ctx, req.(GetUserRequest))
		})
		_, err := handler(serverCtx("Bearer bob-token"), GetUserRequest{ID: "u1"})
		if err == nil || (!strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "relationship_not_found")) {
			t.Fatalf("expected permission denied, got %v", err)
		}
		if called {
			t.Fatal("business handler should not run when accessx denies")
		}
	})

	t.Run("authenticated and authorized reaches handler", func(t *testing.T) {
		handler := chain(func(ctx context.Context, req any) (any, error) {
			return svc.GetUser(ctx, req.(GetUserRequest))
		})
		got, err := handler(serverCtx("Bearer alice-token"), GetUserRequest{ID: "u1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		user := got.(User)
		if user.ID != "u1" || user.DisplayName != "Alice" {
			t.Fatalf("unexpected user: %#v", user)
		}
		records, err := audit.Query(context.Background(), auditx.QueryFilter{ActorID: "alice", ResourceType: "iam:user"})
		if err != nil {
			t.Fatal(err)
		}
		if len(records) == 0 || records[len(records)-1].Result != auditx.ResultSuccess {
			t.Fatalf("expected success audit record, got %#v", records)
		}
	})
}

func iamRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	switch req.(type) {
	case LoginRequest:
		return requestx.Info{Operation: "/aisphere.iam.v1.IAMService/Login", Service: "aisphere.iam.v1.IAMService", Method: "Login", Exposure: accessv1.Exposure_PUBLIC, Action: "iam.login"}, true, nil
	case GetUserRequest:
		return requestx.Info{Operation: "/aisphere.iam.v1.IAMService/GetUser", Service: "aisphere.iam.v1.IAMService", Method: "GetUser", Exposure: accessv1.Exposure_AUTHORIZED, Action: "iam.user.get"}, true, nil
	default:
		return requestx.Info{}, false, nil
	}
}

func iamAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	switch r := req.(type) {
	case GetUserRequest:
		return accessx.Check{Permission: "get", Resource: authz.ObjectRef{Type: "iam:user", ID: r.ID}, AuditAction: "iam.user.get"}, true, nil
	default:
		return accessx.Check{}, false, nil
	}
}

type fakeAuthnProvider struct{ tokens map[string]authn.Principal }

func (p fakeAuthnProvider) Authenticate(ctx context.Context, cred authn.Credential) (authn.Principal, error) {
	principal, ok := p.tokens[cred.Token]
	if !ok {
		return authn.Principal{}, authn.ErrUnauthenticated("invalid demo token")
	}
	principal.Provider = "casdoor-demo"
	principal.AuthMethod = authn.AuthMethodJWT
	return principal, nil
}

func (p fakeAuthnProvider) ExchangeCode(context.Context, authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return authn.TokenSet{}, authn.Principal{}, authn.ErrIdentityBackendFailed("not implemented in demo", nil)
}
func (p fakeAuthnProvider) RefreshToken(context.Context, authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, authn.ErrIdentityBackendFailed("not implemented in demo", nil)
}
func (p fakeAuthnProvider) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return p.Authenticate(ctx, authn.Credential{Scheme: authn.CredentialBearer, Token: req.Token})
}
func (p fakeAuthnProvider) RevokeToken(context.Context, authn.RevokeTokenRequest) error { return nil }
func (p fakeAuthnProvider) BuildLoginURL(context.Context, authn.LoginURLRequest) (authn.LoginURL, error) {
	return authn.LoginURL{URL: "https://casdoor.example.com/login", Provider: "casdoor"}, nil
}
func (p fakeAuthnProvider) HandleCallback(context.Context, authn.CallbackRequest) (authn.CallbackResult, error) {
	return authn.CallbackResult{Provider: "casdoor"}, nil
}

type fakeTransport struct{ header fakeHeader }

func serverCtx(authorization string) context.Context {
	h := fakeHeader{}
	if authorization != "" {
		h.Set("Authorization", authorization)
	}
	return transport.NewServerContext(context.Background(), fakeTransport{header: h})
}

func (fakeTransport) Kind() transport.Kind              { return transport.KindGRPC }
func (fakeTransport) Endpoint() string                  { return "bufconn://iam-service" }
func (fakeTransport) Operation() string                 { return "/aisphere.iam.v1.IAMService/GetUser" }
func (t fakeTransport) RequestHeader() transport.Header { return t.header }
func (fakeTransport) ReplyHeader() transport.Header     { return fakeHeader{} }

type fakeHeader map[string][]string

func (h fakeHeader) Get(key string) string {
	values := h.Values(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func (h fakeHeader) Set(key, value string) { h[key] = []string{value} }
func (h fakeHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h fakeHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h fakeHeader) Values(key string) []string {
	if v, ok := h[key]; ok {
		return v
	}
	for k, v := range h {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return nil
}

var _ authn.Provider = fakeAuthnProvider{}
var _ transport.Transporter = fakeTransport{}
var _ = time.Second
