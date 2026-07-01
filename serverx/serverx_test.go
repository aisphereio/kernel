package serverx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aisphereio/kernel/bootx"
	"github.com/aisphereio/kernel/clientpolicyx"
	"github.com/aisphereio/kernel/gatewayx"
	"github.com/aisphereio/kernel/ratelimitx"
)

func TestNewInstallsSystemRoutes(t *testing.T) {
	app, err := New(context.Background(), Config{
		Name: "demo", Version: "v1",
		HTTP:         HTTPConfig{Enabled: true},
		SystemRoutes: SystemRoutesConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	app.HTTP().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "ok") {
		t.Fatalf("unexpected healthz: code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestNewRejectsInvalidGovernance(t *testing.T) {
	_, err := New(context.Background(), Config{
		Name:       "bad",
		Deployment: bootx.Deployment{Replicas: 3},
		HTTP:       HTTPConfig{Enabled: true},
		Governance: bootx.GovernanceConfig{
			ServerLimits: []ratelimitx.Policy{{Enabled: true, Name: "global", Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeGlobalCluster, QPS: 10, Burst: 10}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid governance error")
	}
}

func TestClientMiddlewareForDownstream(t *testing.T) {
	chain, err := ClientMiddlewareForDownstream(clientpolicyx.DownstreamPolicy{
		Name: "iam", Target: "discovery:///iam", Timeout: time.Second,
		RateLimit: ratelimitx.Policy{Enabled: true, Name: "iam.client", Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 10, Burst: 10},
		Retry:     clientpolicyx.RetryPolicy{Enabled: true, MaxAttempts: 2},
	}, ratelimitx.NewMemoryProvider())
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) == 0 {
		t.Fatal("expected middleware")
	}
}

func TestNewCreatesBothTransports(t *testing.T) {
	app, err := New(context.Background(), Config{
		Name: "both",
		HTTP: HTTPConfig{Enabled: true},
		GRPC: GRPCConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if app.HTTP() == nil || app.GRPC() == nil {
		t.Fatalf("expected both transports, http=%v grpc=%v", app.HTTP(), app.GRPC())
	}
}

func TestRegisterGatewayRoutes(t *testing.T) {
	registry := gatewayx.NewMemoryRegistry()
	err := RegisterGatewayRoutes(context.Background(), registry, gatewayx.Manifest{Service: "iam-service", Namespace: "aisphere", Routes: []gatewayx.GatewayRoute{{ID: "iam.login", Method: http.MethodPost, Path: "/v1/iam/login"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.ListRoutes(); len(got) != 1 || got[0].ID != "iam.login" {
		t.Fatalf("routes=%#v", got)
	}
}
