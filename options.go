package kernel

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/registry"
	transport "github.com/aisphereio/kernel/transportx"
)

// Option is an application option.
type Option func(o *options)

// options is an application options.
type options struct {
	id        string
	name      string
	version   string
	metadata  map[string]string
	endpoints []*url.URL

	ctx  context.Context
	sigs []os.Signal

	logger            *slog.Logger
	logxLogger        logx.Logger
	metrics           metricsx.Manager
	prometheusMetrics bool
	metricsAddr       string
	metricsPath       string
	metricsPprof      bool
	metricsSystem     bool
	registrar         registry.Registrar
	registrarTimeout  time.Duration
	stopTimeout       time.Duration
	servers           []transport.Server

	// Before and After funcs
	beforeStart []func(context.Context) error
	beforeStop  []func(context.Context) error
	afterStart  []func(context.Context) error
	afterStop   []func(context.Context) error
}

// ID with service id.
func ID(id string) Option {
	return func(o *options) { o.id = id }
}

// Name with service name.
func Name(name string) Option {
	return func(o *options) { o.name = name }
}

// Version with service version.
func Version(version string) Option {
	return func(o *options) { o.version = version }
}

// Metadata with service metadata.
func Metadata(md map[string]string) Option {
	return func(o *options) { o.metadata = md }
}

// Endpoint with service endpoint.
func Endpoint(endpoints ...*url.URL) Option {
	return func(o *options) { o.endpoints = endpoints }
}

// Context with service context.
func Context(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// Logger installs the application's slog logger before any lifecycle hooks or
// transport servers start. It also becomes logx's package default so lower-level
// components initialized by hooks can write to the same sink.
func Logger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// LogxLogger installs the application's logx logger before any lifecycle hooks
// or transport servers start. Prefer this option for new Kernel applications so
// dbx/cachex/authn/authz/objectstorex can share the same logger without
// depending on slog directly.
func LogxLogger(logger logx.Logger) Option {
	return func(o *options) { o.logxLogger = logger }
}

// Metrics installs a shared metrics manager before lifecycle hooks or transport
// servers start. The manager is injected into kernel contexts so hooks can call
// metricsx.FromContext(ctx) and pass the same manager into dbx/cachex/s3/authn/authz.
func Metrics(manager metricsx.Manager) Option {
	return func(o *options) {
		o.metrics = manager
		o.metricsSystem = true
	}
}

// PrometheusMetrics creates a Prometheus-backed metrics manager during
// kernel.New and optionally exposes it on a small admin HTTP server. Pass an
// empty addr to create and inject the manager without starting an extra server.
//
// Typical production setup:
//
//	app := kernel.New(
//	    kernel.Name("aihub"),
//	    kernel.Version(version),
//	    kernel.PrometheusMetrics(":9090"), // GET http://127.0.0.1:9090/metrics
//	)
func PrometheusMetrics(addr string) Option {
	return func(o *options) {
		o.prometheusMetrics = true
		o.metricsAddr = addr
		o.metricsSystem = true
	}
}

// MetricsPath changes the admin metrics path used by PrometheusMetrics. Empty
// or malformed values fall back to /metrics.
func MetricsPath(path string) Option {
	return func(o *options) { o.metricsPath = path }
}

// MetricsPprof controls whether kernel's built-in metrics admin server also
// exposes /debug/pprof/*. It is disabled by default to avoid exposing profiling
// endpoints unintentionally in production.
func MetricsPprof(enabled bool) Option {
	return func(o *options) { o.metricsPprof = enabled }
}

// MetricsSystem controls whether standard Go runtime metrics are registered.
// It defaults to true when Metrics or PrometheusMetrics is used.
func MetricsSystem(enabled bool) Option {
	return func(o *options) { o.metricsSystem = enabled }
}

// Server with transport servers.
func Server(srv ...transport.Server) Option {
	return func(o *options) { o.servers = srv }
}

// Signal with exit signals.
func Signal(sigs ...os.Signal) Option {
	return func(o *options) { o.sigs = sigs }
}

// Registrar with service registry.
func Registrar(r registry.Registrar) Option {
	return func(o *options) { o.registrar = r }
}

// RegistrarTimeout with registrar timeout.
func RegistrarTimeout(t time.Duration) Option {
	return func(o *options) { o.registrarTimeout = t }
}

// StopTimeout with app stop timeout.
func StopTimeout(t time.Duration) Option {
	return func(o *options) { o.stopTimeout = t }
}

// Before and Afters

// BeforeStart run funcs before app starts
func BeforeStart(fn func(context.Context) error) Option {
	return func(o *options) {
		o.beforeStart = append(o.beforeStart, fn)
	}
}

// BeforeStop run funcs before app stops
func BeforeStop(fn func(context.Context) error) Option {
	return func(o *options) {
		o.beforeStop = append(o.beforeStop, fn)
	}
}

// AfterStart run funcs after app starts
func AfterStart(fn func(context.Context) error) Option {
	return func(o *options) {
		o.afterStart = append(o.afterStart, fn)
	}
}

// AfterStop run funcs after app stops
func AfterStop(fn func(context.Context) error) Option {
	return func(o *options) {
		o.afterStop = append(o.afterStop, fn)
	}
}
