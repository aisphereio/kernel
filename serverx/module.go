package serverx

import (
	"context"
	"fmt"

	kernel "github.com/aisphereio/kernel"
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
func WithRuntime(p RuntimeProviders) BuildOption { return func(o *buildOptions) { o.runtime = p } }
func WithServerOptions(opts ...Option) BuildOption {
	return func(o *buildOptions) { o.serverOptions = append(o.serverOptions, opts...) }
}

// BuildService is the backwards-compatible generated-service boot path for
// callers that already have a concrete implementation. Prefer
// BuildServiceFromFactory for new services so Kernel-managed Data/DB/providers
// are injected after config, migration, and provider loading.
func BuildService(ctx context.Context, cfg Config, module ServiceModule, impl any, opts ...BuildOption) (*App, error) {
	return buildService(ctx, cfg, module, func(context.Context, ServiceDeps) (any, error) { return impl, nil }, opts...)
}

// BuildServiceFromFactory is the preferred autoload path:
// config + generated module + provider factory + business factory.
// It assembles Kernel middleware automatically, opens DB, runs/validates SQL
// migrations, constructs generated repositories, creates the concrete service
// implementation, and registers generated HTTP/gRPC routes.
func BuildServiceFromFactory(ctx context.Context, cfg Config, module ServiceModule, factory ServiceFactory, opts ...BuildOption) (*App, error) {
	if factory == nil {
		return nil, fmt.Errorf("serverx: service factory is required")
	}
	return buildService(ctx, cfg, module, factory, opts...)
}

func buildService(ctx context.Context, cfg Config, module ServiceModule, factory ServiceFactory, opts ...BuildOption) (*App, error) {
	if err := module.Validate(); err != nil {
		return nil, err
	}
	if cfg.Name == "" {
		cfg.Name = module.ModuleName()
	}

	bo := &buildOptions{}
	for _, opt := range opts {
		opt(bo)
	}

	var managedDB dbx.DB
	var data any
	if cfg.Database.Enabled {
		if err := validateMigrationRuntime(cfg); err != nil {
			return nil, err
		}
		db, err := dbx.New(cfg.Database.DBX)
		if err != nil {
			return nil, err
		}
		managedDB = db
		if err := migrationx.Apply(ctx, managedDB, cfg.Database.Migration); err != nil {
			_ = managedDB.Close()
			return nil, err
		}
		if module.RegisterData != nil {
			registered, err := module.RegisterData(managedDB)
			if err != nil {
				_ = managedDB.Close()
				return nil, err
			}
			data = registered
		}
	}

	providers := bo.providers
	if isZeroProviders(providers) && bo.providerFactory != nil {
		p, err := bo.providerFactory.BuildAccessProviders(ctx, cfg)
		if err != nil {
			if managedDB != nil {
				_ = managedDB.Close()
			}
			return nil, err
		}
		providers = p
	}

	impl, err := factory(ctx, ServiceDeps{Config: cfg, DB: managedDB, Data: data, Providers: providers})
	if err != nil {
		if managedDB != nil {
			_ = managedDB.Close()
		}
		return nil, err
	}
	if impl == nil {
		if managedDB != nil {
			_ = managedDB.Close()
		}
		return nil, fmt.Errorf("serverx: service factory returned nil implementation")
	}

	runtime := bo.runtime
	if isZeroProviders(runtime.Access) {
		runtime.Access = providers
	}
	if runtime.RequestInfoResolver == nil {
		runtime.RequestInfoResolver = module.RequestInfoResolver
	}
	if runtime.AccessResolver == nil {
		runtime.AccessResolver = module.AccessResolver
	}
	if runtime.CredentialExtractor == nil {
		runtime.CredentialExtractor = bo.credentialExtractor
	}
	if runtime.CredentialExtractor == nil {
		runtime.CredentialExtractor = authnmw.BearerExtractor
	}
	runtime.AllowAnonymous = runtime.AllowAnonymous || bo.allowAnonymous

	sxOpts := []Option{WithRuntimeProviders(runtime)}
	if managedDB != nil {
		sxOpts = append(sxOpts, WithKernelOptions(kernel.AfterStop(func(context.Context) error { return managedDB.Close() })))
	}
	sxOpts = append(sxOpts, bo.serverOptions...)
	app, err := New(ctx, cfg, sxOpts...)
	if err != nil {
		if managedDB != nil {
			_ = managedDB.Close()
		}
		return nil, err
	}
	app.db = managedDB
	app.data = data
	if cfg.GRPC.Enabled && module.RegisterGRPC != nil {
		if err := module.RegisterGRPC(app.GRPC(), impl); err != nil {
			return nil, err
		}
	}
	if cfg.HTTP.Enabled && module.RegisterHTTP != nil {
		if err := module.RegisterHTTP(app.HTTP(), impl); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func validateMigrationRuntime(cfg Config) error {
	mig := cfg.Database.Migration.Normalize()
	if cfg.Deployment.Replicas > 1 && (mig.Mode == migrationx.ModeApply || mig.Mode == migrationx.ModeDevApply) && !mig.AllowConcurrent {
		return fmt.Errorf("serverx: database migration mode %s requires single replica or migration.allow_concurrent=true", mig.Mode)
	}
	return nil
}

func isZeroProviders(p accessx.Providers) bool {
	return p.Authn.Authenticator() == nil && p.Authz.Authorizer() == nil && p.Audit == nil
}

// RegisterServiceGatewayRoutes registers a generated module's manifest into the
// route registry. CI/CD or service startup can call this; business code should
// not handcraft routes.
func RegisterServiceGatewayRoutes(ctx context.Context, registry gatewayx.RouteRegistry, modules ...ServiceModule) error {
	manifests := make([]gatewayx.Manifest, 0, len(modules))
	for _, m := range modules {
		if m.GatewayManifest.Service == "" && len(m.GatewayManifest.Routes) == 0 {
			continue
		}
		manifests = append(manifests, m.GatewayManifest)
	}
	return RegisterGatewayRoutes(ctx, registry, manifests...)
}
