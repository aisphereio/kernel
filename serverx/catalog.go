package serverx

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/gatewayx"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/requestx"
	transportgrpc "github.com/aisphereio/kernel/transportx/grpc"
	transporthttp "github.com/aisphereio/kernel/transportx/http"
)

// ServiceCatalog is the Kernel-owned composition boundary for generated service
// modules.
//
// Business services should declare their generated modules once and then use the
// catalog for Gateway route registration, request-info resolution and access
// resolution. This prevents each service from maintaining separate hand-written
// module lists for routes and middleware.
type ServiceCatalog struct {
	modules []ServiceModule
}

// ServiceBinding pairs one generated service module with its concrete service
// implementation. Kernel uses the generated module registration hooks to bind
// the implementation to managed HTTP/RPC transports.
type ServiceBinding struct {
	Module         ServiceModule
	Implementation any
}

// NewServiceCatalog validates and stores generated service modules.
func NewServiceCatalog(modules ...ServiceModule) (ServiceCatalog, error) {
	seen := map[string]struct{}{}
	out := make([]ServiceModule, 0, len(modules))
	for _, module := range modules {
		if err := module.Validate(); err != nil {
			return ServiceCatalog{}, err
		}
		name := module.ModuleName()
		if _, ok := seen[name]; ok {
			return ServiceCatalog{}, fmt.Errorf("serverx: duplicate service module %s", name)
		}
		seen[name] = struct{}{}
		out = append(out, module)
	}
	return ServiceCatalog{modules: out}, nil
}

// MustServiceCatalog is for package-level generated catalogs.
func MustServiceCatalog(modules ...ServiceModule) ServiceCatalog {
	catalog, err := NewServiceCatalog(modules...)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c ServiceCatalog) Modules() []ServiceModule {
	out := make([]ServiceModule, len(c.modules))
	copy(out, c.modules)
	return out
}

func (c ServiceCatalog) RequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	for _, module := range c.modules {
		resolver := module.RequestInfoResolver
		if resolver == nil {
			continue
		}
		info, ok, err := resolver(ctx, operation, req)
		if err != nil || ok {
			return info, ok, err
		}
	}
	return requestx.Info{}, false, nil
}

func (c ServiceCatalog) AccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	for _, module := range c.modules {
		resolver := module.AccessResolver
		if resolver == nil {
			continue
		}
		check, ok, err := resolver(ctx, operation, req)
		if err != nil || ok {
			return check, ok, err
		}
	}
	return accessx.Check{}, false, nil
}

func (c ServiceCatalog) RuntimeProviders(base RuntimeProviders) RuntimeProviders {
	base.RequestInfoResolver = c.RequestInfoResolver
	base.AccessResolver = c.AccessResolver
	return base
}

func (c ServiceCatalog) RegisterGatewayRoutes(ctx context.Context, registry gatewayx.RouteRegistry) error {
	return c.RegisterGatewayRoutesWithFilter(ctx, registry, gatewayx.RouteFilter{})
}

func (c ServiceCatalog) RegisterGatewayRoutesWithFilter(ctx context.Context, registry gatewayx.RouteRegistry, filter gatewayx.RouteFilter) error {
	return RegisterServiceGatewayRoutesWithFilter(ctx, registry, filter, c.modules...)
}

func RegisterHTTPServices(srv *transporthttp.Server, bindings ...ServiceBinding) error {
	if srv == nil {
		return fmt.Errorf("serverx: http server is nil")
	}
	for _, binding := range bindings {
		if err := binding.validate(); err != nil {
			return err
		}
		if binding.Module.RegisterHTTP == nil {
			return fmt.Errorf("serverx: service module %s missing generated http registration hook", binding.Module.ModuleName())
		}
		if err := binding.Module.RegisterHTTP(srv, binding.Implementation); err != nil {
			return err
		}
	}
	return nil
}

func RegisterGRPCServices(srv *transportgrpc.Server, bindings ...ServiceBinding) error {
	if srv == nil {
		return fmt.Errorf("serverx: grpc server is nil")
	}
	for _, binding := range bindings {
		if err := binding.validate(); err != nil {
			return err
		}
		if binding.Module.RegisterGRPC == nil {
			return fmt.Errorf("serverx: service module %s missing generated grpc registration hook", binding.Module.ModuleName())
		}
		if err := binding.Module.RegisterGRPC(srv, binding.Implementation); err != nil {
			return err
		}
	}
	return nil
}

func (b ServiceBinding) validate() error {
	if err := b.Module.Validate(); err != nil {
		return err
	}
	if b.Implementation == nil {
		return fmt.Errorf("serverx: service module %s implementation is nil", b.Module.ModuleName())
	}
	return nil
}

var _ requestx.Resolver = (ServiceCatalog{}).RequestInfoResolver
var _ mwaccess.Resolver = (ServiceCatalog{}).AccessResolver
