// Package serverx provides Kernel's opinionated service assembly layer.
//
// It is intentionally above low-level transport/http and transport/grpc. Business
// components should ask serverx for managed servers and clients instead of wiring
// raw transports, middleware, system routes, governance validation, and lifecycle
// hooks by hand.
package serverx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	kernel "github.com/aisphereio/kernel"
	"github.com/aisphereio/kernel/bootx"
	"github.com/aisphereio/kernel/clientpolicyx"
	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/middleware/autowire"
	"github.com/aisphereio/kernel/migrationx"
	"github.com/aisphereio/kernel/ratelimitx"
	transportgrpc "github.com/aisphereio/kernel/transportx/grpc"
	transporthttp "github.com/aisphereio/kernel/transportx/http"
)

// Config is the high-level Kernel service contract consumed by serverx.
type Config struct {
	Name       string
	Version    string
	Deployment bootx.Deployment

	HTTP HTTPConfig
	GRPC GRPCConfig

	SystemRoutes SystemRoutesConfig
	Governance   bootx.GovernanceConfig

	// Downstreams are caller-side governance contracts. They are validated at
	// startup and should later be used by Kernel client factories.
	Downstreams []clientpolicyx.DownstreamPolicy

	Security SecurityConfig

	Database DatabaseConfig
}

type HTTPConfig struct {
	Enabled bool
	Address string
	Timeout time.Duration
}

type GRPCConfig struct {
	Enabled bool
	Address string
	Timeout time.Duration
}

// SecurityConfig declares provider names used by service boot factories.
// It is configuration only; concrete Casdoor/SpiceDB construction lives behind
// ProviderFactory implementations, not business handlers.
type SecurityConfig struct {
	Authn ProviderRefConfig
	Authz ProviderRefConfig
	Audit AuditConfig
}

type ProviderRefConfig struct {
	Provider string
	Required bool
}

type AuditConfig struct {
	Enabled  bool
	Provider string
}

// DatabaseConfig declares Kernel-managed database boot. It deliberately keeps
// schema SQL outside proto: migrations/ remains the source of truth, dbx opens
// the GORM-backed pool, and migrationx applies or validates SQL migrations.
type DatabaseConfig struct {
	Enabled   bool
	DBX       dbx.Config
	Migration migrationx.Config
}

// SystemRoutesConfig controls generic server routes owned by Kernel, not by business proto.
type SystemRoutesConfig struct {
	Enabled bool
	Healthz bool
	Readyz  bool
	Version bool
	Metrics bool
}

// App is the assembled service entrypoint.
type App struct {
	config Config
	http   *transporthttp.Server
	grpc   *transportgrpc.Server
	app    *kernel.App
	db     dbx.DB
	data   any
}

type Option func(*options)

type options struct {
	serverMiddleware []middleware.Middleware
	httpOptions      []transporthttp.ServerOption
	grpcOptions      []transportgrpc.ServerOption
	kernelOptions    []kernel.Option
}

// WithServerMiddleware adds already-built Kernel middleware to both HTTP and gRPC servers.
func WithServerMiddleware(m ...middleware.Middleware) Option {
	return func(o *options) { o.serverMiddleware = append(o.serverMiddleware, m...) }
}

func WithHTTPOptions(opts ...transporthttp.ServerOption) Option {
	return func(o *options) { o.httpOptions = append(o.httpOptions, opts...) }
}

func WithGRPCOptions(opts ...transportgrpc.ServerOption) Option {
	return func(o *options) { o.grpcOptions = append(o.grpcOptions, opts...) }
}

func WithKernelOptions(opts ...kernel.Option) Option {
	return func(o *options) { o.kernelOptions = append(o.kernelOptions, opts...) }
}

// New validates governance, creates managed transports, installs system routes,
// and returns a Kernel app. It does not start serving until Run is called.
func New(ctx context.Context, cfg Config, opts ...Option) (*App, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if err := validateGovernance(cfg); err != nil {
		return nil, err
	}

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	servers := make([]kernel.Option, 0, 4)
	out := &App{config: cfg}

	if cfg.HTTP.Enabled {
		httpOpts := []transporthttp.ServerOption{
			transporthttp.Address(cfg.HTTP.Address),
			transporthttp.Timeout(cfg.HTTP.Timeout),
			transporthttp.Middleware(o.serverMiddleware...),
		}
		httpOpts = append(httpOpts, o.httpOptions...)
		out.http = transporthttp.NewServer(httpOpts...)
		installSystemRoutes(out.http, cfg)
		servers = append(servers, kernel.Server(out.http))
	}

	if cfg.GRPC.Enabled {
		grpcOpts := []transportgrpc.ServerOption{
			transportgrpc.Address(cfg.GRPC.Address),
			transportgrpc.Timeout(cfg.GRPC.Timeout),
			transportgrpc.Middleware(o.serverMiddleware...),
		}
		grpcOpts = append(grpcOpts, o.grpcOptions...)
		out.grpc = transportgrpc.NewServer(grpcOpts...)
		servers = append(servers, kernel.Server(out.grpc))
	}

	appOpts := []kernel.Option{kernel.Context(ctx), kernel.Name(cfg.Name), kernel.Version(cfg.Version)}
	appOpts = append(appOpts, servers...)
	appOpts = append(appOpts, o.kernelOptions...)
	out.app = kernel.New(appOpts...)
	return out, nil
}

// NewWithDefaultGovernance is a convenience constructor for demos and early services.
// Production services should pass explicit autowire options once providers are configured.
func NewWithDefaultGovernance(ctx context.Context, cfg Config, opts ...Option) (*App, error) {
	defaultChain := autowire.Server(autowire.WithContextInjection())
	opts = append([]Option{WithServerMiddleware(defaultChain...)}, opts...)
	return New(ctx, cfg, opts...)
}

func (a *App) Run() error                  { return a.app.Run() }
func (a *App) KernelApp() *kernel.App      { return a.app }
func (a *App) HTTP() *transporthttp.Server { return a.http }
func (a *App) GRPC() *transportgrpc.Server { return a.grpc }
func (a *App) DB() dbx.DB                  { return a.db }
func (a *App) Data() any                   { return a.data }
func (a *App) Config() Config              { return a.config }

// ClientMiddlewareForDownstream builds the standard client governance chain for one downstream.
func ClientMiddlewareForDownstream(policy clientpolicyx.DownstreamPolicy, provider ratelimitx.Provider) ([]middleware.Middleware, error) {
	if err := policy.Validate(clientpolicyx.Deployment{Replicas: 1}); err != nil {
		return nil, err
	}
	policy = policy.Normalize()
	opts := []autowire.ClientOption{
		autowire.WithClientMetadataPropagation(),
		autowire.WithClientTimeout(policy.Timeout),
	}
	if policy.RateLimit.Enabled {
		opts = append(opts, autowire.WithClientRateLimitPolicy(policy.RateLimit, provider, nil))
	}
	if policy.CircuitBreaker.Enabled {
		opts = append(opts, autowire.WithCircuitBreaker())
	}
	if policy.Retry.Enabled {
		opts = append(opts, autowire.WithRetry())
	}
	return autowire.Client(opts...), nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Name == "" {
		cfg.Name = "kernel-service"
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.Deployment.Replicas <= 0 {
		cfg.Deployment.Replicas = 1
	}
	if cfg.HTTP.Enabled {
		if cfg.HTTP.Address == "" {
			cfg.HTTP.Address = ":0"
		}
		if cfg.HTTP.Timeout <= 0 {
			cfg.HTTP.Timeout = time.Second
		}
	}
	if cfg.GRPC.Enabled {
		if cfg.GRPC.Address == "" {
			cfg.GRPC.Address = ":0"
		}
		if cfg.GRPC.Timeout <= 0 {
			cfg.GRPC.Timeout = time.Second
		}
	}
	if cfg.SystemRoutes.Enabled {
		if !cfg.SystemRoutes.Healthz && !cfg.SystemRoutes.Readyz && !cfg.SystemRoutes.Version && !cfg.SystemRoutes.Metrics {
			cfg.SystemRoutes.Healthz = true
			cfg.SystemRoutes.Readyz = true
			cfg.SystemRoutes.Version = true
		}
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if !cfg.HTTP.Enabled && !cfg.GRPC.Enabled {
		return fmt.Errorf("serverx requires at least one transport")
	}
	return nil
}

func validateGovernance(cfg Config) error {
	gov := cfg.Governance
	gov.Deployment = cfg.Deployment
	gov.Downstreams = append(gov.Downstreams, cfg.Downstreams...)
	return bootx.ValidateGovernance(gov)
}

func installSystemRoutes(s *transporthttp.Server, cfg Config) {
	if s == nil || !cfg.SystemRoutes.Enabled {
		return
	}
	if cfg.SystemRoutes.Healthz {
		s.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeSystemJSON(w, map[string]any{"status": "ok"}) })
	}
	if cfg.SystemRoutes.Readyz {
		s.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { writeSystemJSON(w, map[string]any{"status": "ready"}) })
	}
	if cfg.SystemRoutes.Version {
		s.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			writeSystemJSON(w, map[string]any{"name": cfg.Name, "version": cfg.Version})
		})
	}
	if cfg.SystemRoutes.Metrics {
		s.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_, _ = w.Write([]byte("# metrics are owned by metricsx/prometheus when enabled\n"))
		})
	}
}

func writeSystemJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logx.DefaultLogger().Error("write system route failed", logx.Err(err))
	}
}
