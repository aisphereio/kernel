package metricsx

import (
	"net/http"
	"net/http/pprof"
	"runtime"
	"strings"

	"github.com/gorilla/mux"
	promotel "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
)

// NewPrometheusMeter creates an otel Meter backed by a Prometheus exporter.
// Use this with NewManager to get a Manager that exposes /metrics via GetHandler.
//
//	meter := metricsx.NewPrometheusMeter("aihub", "v1.0.0")
//	m := metricsx.NewManager(meter, logger)
//	http.Handle("/metrics", metricsx.GetHandler(m))
func NewPrometheusMeter(appName, appVersion string) metric.Meter {
	exporter, err := promotel.New(
		promotel.WithoutTargetInfo(),
		promotel.WithTranslationStrategy(otlptranslator.NoTranslation),
	)
	if err != nil {
		return nil
	}
	mp := metricsdk.NewMeterProvider(
		metricsdk.WithReader(exporter),
		metricsdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
			semconv.ServiceVersionKey.String(appVersion),
		)),
	)
	return mp.Meter(appName, metric.WithInstrumentationVersion(appVersion))
}

// NewPrometheusManager is a convenience that creates a Prometheus-backed Meter
// and wraps it in a Manager in one call.
//
//	m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)
//	http.Handle("/metrics", metricsx.GetHandler(m))
func NewPrometheusManager(appName, appVersion string, logger Logger) Manager {
	meter := NewPrometheusMeter(appName, appVersion)
	return NewManager(meter, logger)
}

type handlerConfig struct {
	metricsPath string
	pprof       bool
}

// HandlerOption customizes the admin handler returned by GetHandler.
type HandlerOption func(*handlerConfig)

// WithMetricsPath changes the Prometheus exposition path. Empty or malformed
// values fall back to /metrics.
func WithMetricsPath(path string) HandlerOption {
	return func(cfg *handlerConfig) { cfg.metricsPath = cleanMetricsPath(path) }
}

// WithPprof controls whether /debug/pprof/* endpoints are added to the returned
// handler. GetHandler defaults to true for compatibility; kernel's built-in
// metrics server disables pprof unless explicitly requested.
func WithPprof(enabled bool) HandlerOption {
	return func(cfg *handlerConfig) { cfg.pprof = enabled }
}

// GetHandler returns an HTTP handler that serves:
//   - /metrics by default, or a custom path via WithMetricsPath
//   - /debug/pprof/* when WithPprof(true) is used
//
// It also records system metrics (goroutine count, memory usage, GC count)
// on each metrics scrape. A nil Manager is treated as a no-op Manager.
func GetHandler(m Manager, opts ...HandlerOption) http.Handler {
	cfg := handlerConfig{metricsPath: "/metrics", pprof: true}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	cfg.metricsPath = cleanMetricsPath(cfg.metricsPath)
	m = ensureManager(m)

	router := mux.NewRouter()
	router.NewRoute().Methods(http.MethodGet).Path(cfg.metricsPath).Handler(systemMetricsHandler(m, promhttp.Handler()))

	if cfg.pprof {
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
		router.NewRoute().Methods(http.MethodGet).PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	}

	return router
}

// GetMetricsHandler returns a metrics-only handler suitable for mounting on an
// existing admin server. It does not expose pprof endpoints.
func GetMetricsHandler(m Manager) http.Handler {
	return systemMetricsHandler(ensureManager(m), promhttp.Handler())
}

// systemMetricsHandler wraps the Prometheus handler and records Go runtime
// metrics on each scrape.
func systemMetricsHandler(m Manager, next http.Handler) http.Handler {
	m = ensureManager(m)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)

		m.SetGauge("app_go_goroutines", float64(runtime.NumGoroutine()))
		m.SetGauge("app_sys_memory_alloc", float64(stats.Alloc))
		m.SetGauge("app_sys_total_alloc", float64(stats.TotalAlloc))
		m.SetGauge("app_go_num_gc", float64(stats.NumGC))
		m.SetGauge("app_go_sys", float64(stats.Sys))

		next.ServeHTTP(w, r)
	})
}

// RegisterSystemMetrics registers the standard system metric names so they
// appear in the metric store before the first metrics scrape. It is safe to call
// multiple times with the same Manager.
func RegisterSystemMetrics(m Manager) {
	m = ensureManager(m)
	m.NewGauge("app_go_goroutines", "Number of goroutines")
	m.NewGauge("app_sys_memory_alloc", "Allocated memory in bytes")
	m.NewGauge("app_sys_total_alloc", "Total allocated memory in bytes")
	m.NewGauge("app_go_num_gc", "Number of GC cycles")
	m.NewGauge("app_go_sys", "Memory obtained from OS in bytes")
}

// DefaultBuckets provides sensible histogram bucket boundaries for request
// latency in seconds. Use with NewHistogram:
//
//	m.NewHistogram("request_seconds", "Request latency",
//	    metricsx.DefaultBuckets...)
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// PromCounter returns a Prometheus counter (for direct use with promhttp
// instrumentation, bypassing the otel layer). Advanced use only — prefer
// Manager.NewCounter for business code.
func PromCounter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
}

func cleanMetricsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/metrics"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	if path == "" || path == "/" {
		return "/metrics"
	}
	return path
}
