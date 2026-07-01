package iamgateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aisphereio/kernel/admissionx"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/clientpolicyx"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/middleware/autowire"
	"github.com/aisphereio/kernel/ratelimitx"
	"github.com/aisphereio/kernel/requestx"
)

type gatewayRouteRequest struct {
	Tenant string
	UserID string
	Path   string
}

type iamCheckRequest struct {
	Tenant   string
	UserID   string
	Resource string
	Action   string
}

type iamCheckReply struct{ Allowed bool }

type fakeIAM struct {
	calls int
}

func (s *fakeIAM) Check(ctx context.Context, req iamCheckRequest) (iamCheckReply, error) {
	s.calls++
	info, ok := requestx.FromContext(ctx)
	if !ok {
		return iamCheckReply{}, errors.New("iam missing requestx.Info")
	}
	if info.Direction != requestx.DirectionClient || info.TargetService != "iam" || info.CallerService != "gateway" {
		return iamCheckReply{}, errors.New("iam received incomplete downstream request info")
	}
	if info.RequestID == "" || info.TraceID == "" {
		return iamCheckReply{}, errors.New("iam missing propagated request/trace metadata")
	}
	if req.Tenant == "" {
		return iamCheckReply{}, errorx.BadRequest("TENANT_REQUIRED", "tenant required")
	}
	return iamCheckReply{Allowed: true}, nil
}

type gateway struct {
	iam       *fakeIAM
	iamClient middleware.Handler
}

func (g *gateway) Route(ctx context.Context, req gatewayRouteRequest) (string, error) {
	_, err := g.iamClient(ctx, iamCheckRequest{Tenant: req.Tenant, UserID: req.UserID, Resource: req.Path, Action: "route"})
	if err != nil {
		return "", err
	}
	return "gateway routed", nil
}

func TestGatewayIAMDemoFollowsKernelGovernance(t *testing.T) {
	iam := &fakeIAM{}
	downstream := clientpolicyx.DownstreamPolicy{
		Name: "iam", Protocol: clientpolicyx.ProtocolGRPC, Target: "discovery:///iam", Timeout: time.Second,
		RateLimit:   ratelimitx.Policy{Enabled: true, Name: "iam.client", Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 10, Burst: 10},
		Retry:       clientpolicyx.RetryPolicy{Enabled: true, MaxAttempts: 2},
		ServiceAuth: clientpolicyx.ServiceAuthPolicy{Enabled: true, Mode: "jwt"},
	}
	if err := downstream.Validate(clientpolicyx.Deployment{Replicas: 2}); err != nil {
		t.Fatal(err)
	}
	iamClientChain := middleware.Chain(autowire.Client(
		autowire.WithClientRequestInfoResolver(iamRequestInfoResolver),
		autowire.WithClientMetadataPropagation(),
		autowire.WithClientTimeout(downstream.Timeout),
		autowire.WithClientRateLimitPolicy(downstream.RateLimit, ratelimitx.NewMemoryProvider(), nil),
		autowire.WithRetry(),
	)...)
	gw := &gateway{iam: iam}
	gw.iamClient = iamClientChain(func(ctx context.Context, req any) (any, error) {
		return iam.Check(ctx, req.(iamCheckRequest))
	})

	serverChain := middleware.Chain(autowire.Server(
		autowire.WithRequestInfoResolver(gatewayRequestInfoResolver),
		autowire.WithAdmission(gatewayAdmission()),
	)...)
	handler := serverChain(func(ctx context.Context, req any) (any, error) {
		return gw.Route(ctx, req.(gatewayRouteRequest))
	})

	ctx := contextx.WithRequestID(context.Background(), "req-demo")
	ctx = contextx.WithTraceID(ctx, "trace-demo")
	out, err := handler(ctx, gatewayRouteRequest{UserID: "u1", Path: "/v1/skills/s1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.(string) != "gateway routed" {
		t.Fatalf("unexpected response %v", out)
	}
	if iam.calls != 1 {
		t.Fatalf("expected one IAM call, got %d", iam.calls)
	}
}

func TestGatewayClientRateLimitProtectsIAM(t *testing.T) {
	iam := &fakeIAM{}
	policy := ratelimitx.Policy{Enabled: true, Name: "iam.client", Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 1, Burst: 1}
	chain := middleware.Chain(autowire.Client(
		autowire.WithClientRequestInfoResolver(iamRequestInfoResolver),
		autowire.WithClientRateLimitPolicy(policy, ratelimitx.NewMemoryProvider(), nil),
	)...)
	client := chain(func(ctx context.Context, req any) (any, error) { return iam.Check(ctx, req.(iamCheckRequest)) })
	ctx := contextx.WithRequestID(context.Background(), "req-limit")
	ctx = contextx.WithTraceID(ctx, "trace-limit")
	if _, err := client(ctx, iamCheckRequest{Tenant: "t1", UserID: "u1", Resource: "r", Action: "read"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client(ctx, iamCheckRequest{Tenant: "t1", UserID: "u1", Resource: "r", Action: "read"}); err == nil {
		t.Fatal("expected client-side rate limit")
	}
	if iam.calls != 1 {
		t.Fatalf("second request should be rejected before IAM, calls=%d", iam.calls)
	}
}

func TestGatewayAdmissionDefaultsAndValidatesTenant(t *testing.T) {
	chain := gatewayAdmission()
	ctx := requestx.NewContext(context.Background(), requestx.Info{Operation: "/gateway.v1.GatewayService/Route"})
	got, err := chain.Admit(ctx, gatewayRouteRequest{UserID: "u1", Path: "/v1/skills/s1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.(gatewayRouteRequest).Tenant != "default" {
		t.Fatalf("tenant not defaulted: %#v", got)
	}
	_, err = chain.Admit(ctx, gatewayRouteRequest{UserID: "u1", Path: "/forbidden"})
	if err == nil {
		t.Fatal("expected admission rejection")
	}
}

func gatewayRequestInfoResolver(ctx context.Context, _ string, req any) (requestx.Info, bool, error) {
	r, ok := req.(gatewayRouteRequest)
	if !ok {
		return requestx.Info{}, false, nil
	}
	return requestx.Info{
		Direction:     requestx.DirectionServer,
		Protocol:      requestx.ProtocolHTTP,
		Service:       "gateway.v1.GatewayService",
		Method:        "Route",
		Operation:     "/gateway.v1.GatewayService/Route",
		Exposure:      accessv1.Exposure_PUBLIC,
		Action:        "gateway.route",
		Resource:      r.Path,
		UserID:        r.UserID,
		TargetService: "gateway",
	}, true, nil
}

func iamRequestInfoResolver(ctx context.Context, _ string, req any) (requestx.Info, bool, error) {
	r, ok := req.(iamCheckRequest)
	if !ok {
		return requestx.Info{}, false, nil
	}
	return requestx.Info{
		Direction:     requestx.DirectionClient,
		Protocol:      requestx.ProtocolGRPC,
		Service:       "iam.v1.IAMService",
		Method:        "CheckPermission",
		Operation:     "/iam.v1.IAMService/CheckPermission",
		Exposure:      accessv1.Exposure_INTERNAL,
		Action:        r.Action,
		Resource:      r.Resource,
		TenantID:      r.Tenant,
		UserID:        r.UserID,
		CallerService: "gateway",
		TargetService: "iam",
	}, true, nil
}

func gatewayAdmission() admissionx.Chain {
	return admissionx.New([]admissionx.MutatingPlugin{
		admissionx.MutatingPluginFunc{PluginName: "gateway.default-tenant", Fn: func(ctx context.Context, a admissionx.Attributes) (any, error) {
			req := a.Object.(gatewayRouteRequest)
			if req.Tenant == "" {
				req.Tenant = "default"
			}
			return req, nil
		}},
	}, []admissionx.ValidatingPlugin{
		admissionx.ValidatingPluginFunc{PluginName: "gateway.path-policy", Fn: func(ctx context.Context, a admissionx.Attributes) error {
			req := a.Object.(gatewayRouteRequest)
			if req.Path == "/forbidden" {
				return errors.New("path denied by gateway admission")
			}
			return nil
		}},
	})
}
