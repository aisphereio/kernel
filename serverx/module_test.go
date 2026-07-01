package serverx

import (
	"context"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/bootx"
	accessmw "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/migrationx"
	"github.com/aisphereio/kernel/requestx"
	transportgrpc "github.com/aisphereio/kernel/transportx/grpc"
	transporthttp "github.com/aisphereio/kernel/transportx/http"
)

func TestBuildServiceAutoLoadsModuleAndProviders(t *testing.T) {
	grpcRegistered := false
	httpRegistered := false
	mod := ServiceModule{
		Name: "skill-service",
		RequestInfoResolver: func(context.Context, string, any) (requestx.Info, bool, error) {
			return requestx.Info{Operation: "/svc/Get"}, true, nil
		},
		AccessResolver: func(context.Context, string, any) (accessx.Check, bool, error) { return accessx.Check{}, true, nil },
		RegisterGRPC:   func(*transportgrpc.Server, any) error { grpcRegistered = true; return nil },
		RegisterHTTP:   func(*transporthttp.Server, any) error { httpRegistered = true; return nil },
	}
	cfg := Config{Name: "skill-service", HTTP: HTTPConfig{Enabled: true}, GRPC: GRPCConfig{Enabled: true}}
	app, err := BuildService(context.Background(), cfg, mod, struct{}{}, WithAccessProviders(accessx.Providers{
		Authn: authn.ProviderSet{Runtime: fakeModuleAuthn{}},
		Authz: authz.ProviderSet{Runtime: authz.AllowAllForDevOnly()},
		Audit: auditx.Noop(),
	}))
	if err != nil {
		t.Fatalf("BuildService failed: %v", err)
	}
	if app.HTTP() == nil || app.GRPC() == nil {
		t.Fatalf("expected both transports")
	}
	if !grpcRegistered || !httpRegistered {
		t.Fatalf("expected generated registrars to run grpc=%v http=%v", grpcRegistered, httpRegistered)
	}
}

func TestServiceModuleValidateRequiresGeneratedResolvers(t *testing.T) {
	err := (ServiceModule{Name: "bad"}).Validate()
	if err == nil {
		t.Fatal("expected missing resolver error")
	}
	_ = accessmw.Resolver(nil)
}

type fakeModuleAuthn struct{}

func (fakeModuleAuthn) Authenticate(context.Context, authn.Credential) (authn.Principal, error) {
	return authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser}, nil
}
func (fakeModuleAuthn) ExchangeCode(context.Context, authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return authn.TokenSet{}, authn.Principal{}, nil
}
func (fakeModuleAuthn) RefreshToken(context.Context, authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, nil
}
func (fakeModuleAuthn) VerifyToken(context.Context, authn.VerifyTokenRequest) (authn.Principal, error) {
	return authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser}, nil
}
func (fakeModuleAuthn) RevokeToken(context.Context, authn.RevokeTokenRequest) error { return nil }
func (fakeModuleAuthn) BuildLoginURL(context.Context, authn.LoginURLRequest) (authn.LoginURL, error) {
	return authn.LoginURL{}, nil
}
func (fakeModuleAuthn) HandleCallback(context.Context, authn.CallbackRequest) (authn.CallbackResult, error) {
	return authn.CallbackResult{}, nil
}

var _ authn.Provider = fakeModuleAuthn{}

func TestBuildServiceFromFactoryRejectsConcurrentMigrationApply(t *testing.T) {
	mod := ServiceModule{
		Name: "skill-service",
		RequestInfoResolver: func(context.Context, string, any) (requestx.Info, bool, error) {
			return requestx.Info{Operation: "/svc/Get"}, true, nil
		},
		AccessResolver: func(context.Context, string, any) (accessx.Check, bool, error) { return accessx.Check{}, true, nil },
	}
	cfg := Config{
		Name:       "skill-service",
		HTTP:       HTTPConfig{Enabled: true},
		Deployment: bootx.Deployment{Replicas: 2},
		Database:   DatabaseConfig{Enabled: true, Migration: migrationx.Config{Enabled: true, Mode: migrationx.ModeApply}},
	}
	_, err := BuildServiceFromFactory(context.Background(), cfg, mod, func(context.Context, ServiceDeps) (any, error) {
		return struct{}{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "single replica") {
		t.Fatalf("expected concurrent migration guard, got %v", err)
	}
}
