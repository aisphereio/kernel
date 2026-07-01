package platformflow

import (
	"context"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/middleware"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/serverx"
)

type LoginRequest struct{ Username, Password string }
type LoginReply struct{ AccessToken string }
type GetSkillRequest struct{ ID string }
type Skill struct{ ID, Name string }

type iamService struct{}

func (iamService) Login(ctx context.Context, req LoginRequest) (LoginReply, error) {
	if req.Username != "alice" || req.Password == "" {
		return LoginReply{}, authn.ErrUnauthenticated("invalid demo login")
	}
	return LoginReply{AccessToken: "alice-token"}, nil
}

type skillService struct {
	called int
	skills map[string]Skill
}

func (s *skillService) GetSkill(ctx context.Context, req GetSkillRequest) (Skill, error) {
	s.called++
	skill, ok := s.skills[req.ID]
	if !ok {
		return Skill{}, errorx.NotFound("SKILL_NOT_FOUND", "skill not found")
	}
	return skill, nil
}

func TestGatewayIAMSkillServiceKernelDrivenFlow(t *testing.T) {
	// This is the end-to-end validation shape we want from real services:
	// kernel new -> proto access policy -> make api -> generated route manifest
	// and generated request/access resolvers. The test uses hand-written
	// generated-equivalent functions to avoid requiring online buf deps locally.
	kv := gatewayx.NewMemoryKVStore()
	registry := gatewayx.NewEtcdRegistry(kv, "/aisphere/kernel/routes/test")
	if err := serverx.RegisterServiceGatewayRoutes(context.Background(), registry, IAMServiceKernelModule(), SkillServiceKernelModule()); err != nil {
		t.Fatal(err)
	}

	authzStore := authz.NewMemoryRelationshipStore()
	if _, err := authzStore.WriteRelationships(context.Background(), authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"},
		Relation: "get",
		Subject:  authz.SubjectRef{Type: authn.SubjectTypeUser, ID: "alice"},
	}); err != nil {
		t.Fatal(err)
	}
	audit := auditx.NewMemoryStore()
	authnProvider := fakeAuthnProvider{tokens: map[string]authn.Principal{
		"alice-token": {SubjectID: "alice", SubjectType: authn.SubjectTypeUser, TenantID: "t1", OrgID: "aisphere", Username: "alice"},
		"bob-token":   {SubjectID: "bob", SubjectType: authn.SubjectTypeUser, TenantID: "t1", OrgID: "aisphere", Username: "bob"},
	}}
	providers := accessx.Providers{
		Authn: authn.ProviderSet{Runtime: authnProvider},
		Authz: authz.ProviderSet{Runtime: authz.NewMemoryAuthorizer(authzStore)},
		Audit: audit,
	}

	skill := &skillService{skills: map[string]Skill{"s1": {ID: "s1", Name: "Kernel Skill"}}}
	skillChain := middleware.Chain(serverx.ServerMiddlewareFromProviders(context.Background(), serverx.RuntimeProviders{
		Access:              providers,
		RequestInfoResolver: SkillServiceKernelModule().RequestInfoResolver,
		AccessResolver:      SkillServiceKernelModule().AccessResolver,
		CredentialExtractor: authnmw.BearerExtractor,
	})...)
	skillHandler := skillChain(func(ctx context.Context, req any) (any, error) {
		return skill.GetSkill(ctx, req.(GetSkillRequest))
	})

	invokers := gatewayx.NewInvokerRegistry()
	if err := RegisterPlatformGatewayInvokers(invokers, iamService{}, skill, skillHandler); err != nil {
		t.Fatal(err)
	}
	gw := gatewayx.NewDispatcher(registry, gatewayx.StaticHosts{
		"iam-service.aisphere":   "iam.local",
		"skill-service.aisphere": "skill.local",
	}, invokers)

	t.Run("public login reaches iam service without authorization", func(t *testing.T) {
		resp, err := gw.Dispatch(context.Background(), gatewayx.DispatchRequest{
			Method: "POST", Path: "/v1/iam/login", Body: LoginRequest{Username: "alice", Password: "pw"},
			Headers: map[string]string{"X-Request-ID": "req-login", "X-Trace-ID": "trace-login"},
		})
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
		token := resp.Body.(LoginReply).AccessToken
		if token != "alice-token" {
			t.Fatalf("unexpected token %q", token)
		}
	})

	t.Run("protected route without token is rejected by gateway before skill service", func(t *testing.T) {
		skill.called = 0
		_, err := gw.Dispatch(context.Background(), gatewayx.DispatchRequest{Method: "GET", Path: "/v1/skills/s1", Headers: map[string]string{"X-Request-ID": "req-no-token"}})
		if err == nil || !strings.Contains(err.Error(), "login required") {
			t.Fatalf("expected gateway 401, got %v", err)
		}
		if skill.called != 0 {
			t.Fatalf("skill service should not be called, called=%d", skill.called)
		}
	})

	t.Run("authenticated but unauthorized is denied by skill service authz", func(t *testing.T) {
		skill.called = 0
		_, err := gw.Dispatch(context.Background(), gatewayx.DispatchRequest{
			Method: "GET", Path: "/v1/skills/s1",
			Headers: map[string]string{"Authorization": "Bearer bob-token", "X-Request-ID": "req-bob", "X-Trace-ID": "trace-bob"},
		})
		if err == nil || (!strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "relationship_not_found")) {
			t.Fatalf("expected service authz denial, got %v", err)
		}
		if skill.called != 0 {
			t.Fatalf("business handler should not run when authz denies, called=%d", skill.called)
		}
	})

	t.Run("authenticated and authorized reaches skill service", func(t *testing.T) {
		resp, err := gw.Dispatch(context.Background(), gatewayx.DispatchRequest{
			Method: "GET", Path: "/v1/skills/s1",
			Headers: map[string]string{"Authorization": "Bearer alice-token", "X-Request-ID": "req-alice", "X-Trace-ID": "trace-alice"},
		})
		if err != nil {
			t.Fatalf("get skill failed: %v", err)
		}
		got := resp.Body.(Skill)
		if got.ID != "s1" || got.Name != "Kernel Skill" {
			t.Fatalf("unexpected skill: %#v", got)
		}
		records, err := audit.Query(context.Background(), auditx.QueryFilter{ActorID: "alice", ResourceType: "skill"})
		if err != nil {
			t.Fatal(err)
		}
		if len(records) == 0 || records[len(records)-1].Result != auditx.ResultSuccess {
			t.Fatalf("expected success audit, got %#v", records)
		}
	})
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
	return authn.TokenSet{}, authn.Principal{}, authn.ErrIdentityBackendFailed("not implemented", nil)
}
func (p fakeAuthnProvider) RefreshToken(context.Context, authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, authn.ErrIdentityBackendFailed("not implemented", nil)
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

var _ authn.Provider = fakeAuthnProvider{}
