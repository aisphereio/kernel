package server

import (
	"context"

	v1 "github.com/aisphereio/kernel-layout/api/todo/v1"
	"github.com/aisphereio/kernel-layout/internal/conf"
	"github.com/aisphereio/kernel-layout/internal/data"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/requestx"
	"github.com/aisphereio/kernel/securityx"
	"github.com/aisphereio/kernel/serverx"
)

func todoServerMiddlewares(resources *data.Resources, cfg conf.SecurityConfig) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	securityRuntime := mustSecurityRuntime(cfg)
	return serverx.ServerMiddlewareFromProviders(context.Background(), serverx.RuntimeProviders{
		Security:            securityRuntime,
		AccessGuard:         &resources.Access,
		RequestInfoResolver: v1.TodoServiceRequestInfoResolver,
		AccessResolver:      todoAccessResolver,
	})
}

func mustSecurityRuntime(cfg conf.SecurityConfig) *securityx.Runtime {
	runtime, err := securityx.NewRuntime(context.Background(), securityx.Config{
		Authn: securityx.AuthnBoundaryConfig{
			Enabled:        cfg.Authn.Enabled,
			Mode:           cfg.Authn.Mode,
			Provider:       cfg.Authn.Provider,
			OIDC:           cfg.Authn.OIDC,
			InternalCall:   cfg.InternalCall,
			CacheTTL:       cfg.Authn.CacheTTL,
			AllowAnonymous: true,
		},
		InternalCall: cfg.InternalCall,
		Access:       cfg.Access,
	}, nil)
	if err != nil {
		panic(err)
	}
	return runtime
}

func todoAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	check, ok, err := v1.TodoServiceAccessResolver(ctx, operation, req)
	if err != nil || ok {
		return check, ok, err
	}
	return accessx.Check{}, false, nil
}

var _ requestx.Resolver = v1.TodoServiceRequestInfoResolver
var _ mwaccess.Resolver = todoAccessResolver
