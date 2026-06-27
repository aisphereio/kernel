package metricsx

import (
	"net/http"
	"net/http/pprof"
	"runtime"

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

// GetHandler returns an HTTP handler that serves:
//   - /metrics — Prometheus exposition format
//   - /debug/pprof/* — Go pprof profiling endpoints
//
// It also records system metrics (goroutine count, memory usage, GC count)
// on each /metrics scrape.
func GetHandler(m Manager) http.Handler {
	router := mux.NewRouter()
	router.NewRoute().Methods(http.MethodGet).Path("/metrics").Handler(systemMetricsHandler(m, promhttp.Handler()))

	// pprof endpoints
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.NewRoute().Methods(http.MethodGet).PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

	return router
}

// systemMetricsHandler wraps the Prometheus handler and records Go runtime
// metrics on each scrape.
func systemMetricsHandler(m Manager, next http.Handler) http.Handler {
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
// appear in the metric store before the first /metrics scrape. Call this at
// startup if you want the system metrics to always be present.
func RegisterSystemMetrics(m Manager) {
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
