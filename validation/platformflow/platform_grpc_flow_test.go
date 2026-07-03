//go:build platformflow_validation
// +build platformflow_validation

package platformflow

import (
	"context"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/middleware"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/serverx"
)

func TestGatewayIAMSkillServiceRealGRPCFlow(t *testing.T) {
	ctx := context.Background()
	kv := gatewayx.NewMemoryKVStore()
	registry := gatewayx.NewEtcdRegistry(kv, "/aisphere/kernel/routes/test")
	if err := serverx.RegisterServiceGatewayRoutes(ctx, registry, IAMServiceKernelModule(), SkillServiceKernelModule()); err != nil {
		t.Fatal(err)
	}

	authzStore := authz.NewMemoryRelationshipStore()
	if _, err := authzStore.WriteRelationships(ctx, authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"},
		Relation: "get",
		Subject:  authz.SubjectRef{Type: authn.SubjectTypeUser, ID: "alice"},
	}); err != nil {
		t.Fatal(err)
	}
	audit := auditx.NewMemoryStore()
	providers := accessx.Providers{
		Authn: authn.ProviderSet{Runtime: fakeAuthnProvider{tokens: map[string]authn.Principal{
			"alice-token": {SubjectID: "alice", SubjectType: authn.SubjectTypeUser, TenantID: "t1", OrgID: "aisphere", Username: "alice"},
			"bob-token":   {SubjectID: "bob", SubjectType: authn.SubjectTypeUser, TenantID: "t1", OrgID: "aisphere", Username: "bob"},
		}}},
		Authz: authz.ProviderSet{Runtime: authz.NewMemoryAuthorizer(authzStore)},
		Audit: audit,
	}
	skill := &skillService{skills: map[string]Skill{"s1": {ID: "s1", Name: "Kernel Skill"}}}
	skillChain := middleware.Chain(serverx.ServerMiddlewareFromProviders(ctx, serverx.RuntimeProviders{
		Access:              providers,
		RequestInfoResolver: SkillServiceKernelModule().RequestInfoResolver,
		AccessResolver:      SkillServiceKernelModule().AccessResolver,
		CredentialExtractor: authnmw.BearerExtractor,
	})...)
	skillHandler := skillChain(func(ctx context.Context, req any) (any, error) { return skill.GetSkill(ctx, req.(GetSkillRequest)) })

	cc, cleanup, err := newPlatformBufconn(ctx, iamService{}, skillHandler)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	invokers := gatewayx.NewInvokerRegistry()
	if err := RegisterPlatformGatewayGRPCInvokers(invokers, NewIAMServiceClient(cc), NewSkillServiceClient(cc)); err != nil {
		t.Fatal(err)
	}
	gw := gatewayx.NewDispatcher(registry, gatewayx.StaticHosts{"iam-service.aisphere": "bufnet", "skill-service.aisphere": "bufnet"}, invokers)

	login, err := gw.Dispatch(ctx, gatewayx.DispatchRequest{Method: "POST", Path: "/v1/iam/login", Body: LoginRequest{Username: "alice", Password: "pw"}})
	if err != nil {
		t.Fatalf("login failed through grpc gateway path: %v", err)
	}
	if login.Body.(LoginReply).AccessToken != "alice-token" {
		t.Fatalf("unexpected login reply: %#v", login.Body)
	}

	skill.called = 0
	_, err = gw.Dispatch(ctx, gatewayx.DispatchRequest{Method: "GET", Path: "/v1/skills/s1"})
	if err == nil || !strings.Contains(err.Error(), "login required") {
		t.Fatalf("expected gateway authn reject, got %v", err)
	}
	if skill.called != 0 {
		t.Fatalf("service should not run without token, called=%d", skill.called)
	}

	_, err = gw.Dispatch(ctx, gatewayx.DispatchRequest{Method: "GET", Path: "/v1/skills/s1", Headers: map[string]string{"Authorization": "Bearer bob-token"}})
	if err == nil || (!strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "relationship_not_found")) {
		t.Fatalf("expected service authz reject, got %v", err)
	}

	resp, err := gw.Dispatch(ctx, gatewayx.DispatchRequest{Method: "GET", Path: "/v1/skills/s1", Headers: map[string]string{"Authorization": "Bearer alice-token", "X-Request-ID": "req-grpc"}})
	if err != nil {
		t.Fatalf("get skill through grpc failed: %v", err)
	}
	got := resp.Body.(Skill)
	if got.ID != "s1" || got.Name != "Kernel Skill" {
		t.Fatalf("unexpected skill: %#v", got)
	}
	records, err := audit.Query(ctx, auditx.QueryFilter{ActorID: "alice", ResourceType: "skill"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 || records[len(records)-1].Result != auditx.ResultSuccess {
		t.Fatalf("expected success audit, got %#v", records)
	}
}
