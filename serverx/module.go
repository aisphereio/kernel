package serverx

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/migrationx"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/gatewayx"
	accessmw "github.com/aisphereio/kernel/middleware/access"
	authnmw "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/requestx"
	transportgrpc "github.com/aisphereio/kernel/transportx/grpc"
	transporthttp "github.com/aisphereio/kernel/transportx/http"
)

// ServiceModule is the generated service contract consumed by serverx.
//
// protoc-gen-go-kernel / protoc-gen-go-gateway should generate one module per
// proto service. The module carries generated metadata and registration hooks so
// service boot code does not manually wire authn/authz/audit, Gateway manifests,
// HTTP routes, or gRPC servers.
type ServiceModule struct {
	Name string

	// GatewayManifest is generated from google.api.http + aisphere.access.v1.policy.
	GatewayManifest gatewayx.Manifest

	// RequestInfoResolver and AccessResolver are generated from proto service/RPC
	// metadata and access policy. Runtime code must not re-derive this from raw
	// path/method strings.
	RequestInfoResolver requestx.Resolver
	AccessResolver      accessmw.Resolver

	// RegisterGRPC registers the generated gRPC server against Kernel's managed
	// gRPC transport. The service implementation is passed as any because
	// generated code owns the concrete interface assertion.
	RegisterGRPC func(*transportgrpc.Server, any) error

	// RegisterHTTP registers generated HTTP/JSON routes against Kernel's managed
	// HTTP transport. Optional for services that only expose gRPC internally.
	RegisterHTTP func(*transporthttp.Server, any) error

	// RegisterGatewayInvokers registers generated Gateway -> gRPC operation
	// invokers. Gateways use this when linking compiled service clients.
	RegisterGatewayInvokers func(*gatewayx.InvokerRegistry, any) error

	// RegisterData is optional. Generated or hand-written data modules can use it
	// to construct repositories from Kernel-managed dbx.DB. Migrations remain SQL
	// files under migrations/; this hook is only for safe repo construction.
	RegisterData func(dbx.DB) (any, error)
}

func (m ServiceModule) Validate() error {
	if m.Name == "" && m.GatewayManifest.Service == "" {
		return fmt.Errorf("serverx: service module name is required")
	}
	if m.RequestInfoResolver == nil {
		return fmt.Errorf("serverx: service module %s missing request info resolver", m.ModuleName())
	}
	if m.AccessResolver == nil {
		return fmt.Errorf("serverx: service module %s missing access resolver", m.ModuleName())
	}
	return nil
}

func (m ServiceModule) ModuleName() string {
	if m.Name != "" {
		return m.Name
	}
	return m.GatewayManifest.Service
}

// RegisterServiceGatewayRoutes registers generated module manifests into the
// route registry without filtering. Prefer RegisterServiceGatewayRoutesWithFilter
// for public/internal/ops gateway profiles so internal routes do not leak into
// the wrong registry.
func RegisterServiceGatewayRoutes(ctx context.Context, registry gatewayx.RouteRegistry, modules ...ServiceModule) error {
	return RegisterServiceGatewayRoutesWithFilter(ctx, registry, gatewayx.RouteFilter{}, modules...)
}

// RegisterServiceGatewayRoutesWithFilter registers generated module manifests
// after applying a profile-aware route filter. This is the recommended path for
// production gateways: public gateways should use gatewayx.PublicRouteFilter(),
// internal gateways should use gatewayx.InternalRouteFilter(), and ops gateways
// should use an explicit allowlist.
func RegisterServiceGatewayRoutesWithFilter(ctx context.Context, registry gatewayx.RouteRegistry, filter gatewayx.RouteFilter, modules ...ServiceModule) error {
	_ = ctx
	if registry == nil {
		return fmt.Errorf("serverx: gateway route registry is nil")
	}
	for _, module := range modules {
		manifest := module.GatewayManifest
		if manifest.Service == "" && len(manifest.Routes) == 0 {
			continue
		}
		manifest = gatewayx.FilterManifest(manifest, filter)
		if manifest.Service == "" && len(manifest.Routes) == 0 {
			continue
		}
		if len(manifest.Routes) == 0 {
			continue
		}
		if err := registry.RegisterManifest(manifest); err != nil {
			return err
		}
	}
	return nil
}

// ProviderFactory creates provider-neutral Kernel providers from Config. Concrete
// factories may live in contrib/authn/casdoor, contrib/authz/spicedb, or platform
// boot packages. Business services should not instantiate provider SDKs directly.
type ProviderFactory interface {
	BuildAccessProviders(ctx context.Context, cfg Config) (accessx.Providers, error)
}

// ServiceDeps are Kernel-managed dependencies produced during service boot.
// Generated services should accept these through a ServiceFactory so data repos,
// provider sets, and managed DB are injected after config/migration/provider
// loading, instead of being hand-wired in main.go.
type ServiceDeps struct {
	Config    Config
	DB        dbx.DB
	Data      any
	Providers accessx.Providers
}

// ServiceFactory creates the concrete generated service implementation from
// Kernel-managed dependencies. This is the preferred autoload path.
type ServiceFactory func(ctx context.Context, deps ServiceDeps) (any, error)

type BuildOption func(*buildOptions)

type buildOptions struct {
	providers           accessx.Providers
	providerFactory     ProviderFactory
	runtime             RuntimeProviders
	credentialExtractor authnmw.CredentialExtractor
	allowAnonymous      bool
	serverOptions       []Option
}

func WithAccessProviders(p accessx.Providers) BuildOption {
	return func(o *buildOptions) { o.providers = p }
}
func WithProviderFactory(f ProviderFactory) BuildOption {
	return func(o *buildOptions) { o.providerFactory = f }
}
func WithCredentialExtractor(e authnmw.CredentialExtractor) BuildOption {
	return func(o *buildOptions) { o.credentialExtractor = e }
}
func WithAllowAnonymous(allow bool) BuildOption {
	return func(o *buildOptions) { o.allowAnonymous = allow }
}

// BuildService assembles a generated service module with a concrete service
// implementation and Kernel-managed transports/middleware.
func BuildService(ctx context.Context, cfg Config, module ServiceModule, svc any, opts ...BuildOption) (*App, error) {
	return BuildServiceFromFactory(ctx, cfg, module, func(context.Context, ServiceDeps) (any, error) {
		return svc, nil
	}, opts...)
}

// BuildServiceFromFactory assembles a generated service module after loading
// Kernel-managed dependencies. Provider SDKs and database construction stay in
// boot/provider layers; generated service handlers receive only ServiceDeps.
func BuildServiceFromFactory(ctx context.Context, cfg Config, module ServiceModule, factory ServiceFactory, opts ...BuildOption) (*App, error) {
	if err := module.Validate(); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("serverx: service factory is nil")
	}

	bo := &buildOptions{}
	for _, opt := range opts {
		opt(bo)
	}

	if err := validateMigrationConcurrency(cfg); err != nil {
		return nil, err
	}

	providers := bo.providers
	if bo.providerFactory != nil {
		built, err := bo.providerFactory.BuildAccessProviders(ctx, cfg)
		if err != nil {
			return nil, err
		}
		providers = built
	}

	runtime := bo.runtime
	runtime.Access = providers
	runtime.RequestInfoResolver = module.RequestInfoResolver
	runtime.AccessResolver = module.AccessResolver
	runtime.CredentialExtractor = bo.credentialExtractor
	runtime.AllowAnonymous = bo.allowAnonymous

	serverOpts := append([]Option{WithRuntimeProviders(runtime)}, bo.serverOptions...)
	app, err := New(ctx, cfg, serverOpts...)
	if err != nil {
		return nil, err
	}

	if cfg.Database.Enabled {
		db, err := dbx.New(cfg.Database.DBX)
		if err != nil {
			return nil, err
		}
		app.db = db
		if cfg.Database.Migration.Enabled {
			if err := migrationx.Apply(ctx, db, cfg.Database.Migration); err != nil {
				_ = db.Close()
				return nil, err
			}
		}
		if module.RegisterData != nil {
			data, err := module.RegisterData(db)
			if err != nil {
				_ = db.Close()
				return nil, err
			}
			app.data = data
		}
	}

	svc, err := factory(ctx, ServiceDeps{
		Config:    cfg,
		DB:        app.db,
		Data:      app.data,
		Providers: providers,
	})
	if err != nil {
		if app.db != nil {
			_ = app.db.Close()
		}
		return nil, err
	}

	if app.GRPC() != nil && module.RegisterGRPC != nil {
		if err := module.RegisterGRPC(app.GRPC(), svc); err != nil {
			return nil, err
		}
	}
	if app.HTTP() != nil && module.RegisterHTTP != nil {
		if err := module.RegisterHTTP(app.HTTP(), svc); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func validateMigrationConcurrency(cfg Config) error {
	migration := cfg.Database.Migration.Normalize()
	if !cfg.Database.Enabled || !cfg.Database.Migration.Enabled {
		return nil
	}
	switch migration.Mode {
	case migrationx.ModeApply, migrationx.ModeDevApply:
	default:
		return nil
	}
	if cfg.Deployment.Replicas > 1 && !migration.AllowConcurrent {
		return fmt.Errorf("serverx: migration apply requires a single replica or migration.allow_concurrent=true")
	}
	return nil
}
