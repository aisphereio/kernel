package metricsx_test

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/metricsx"
)

// ============================================================================
// Manager bootstrap EXAMPLES
// ============================================================================

func ExampleNewManager() {
	// In real code: meter := metricsx.NewPrometheusMeter("aihub", "v1.0.0")
	// Here we pass nil meter to get a no-op Manager (safe for examples).
	m := metricsx.NewManager(nil, nil)
	fmt.Println(m == nil)
	// Output: false
}

func ExampleNoop() {
	// Noop returns a Manager that discards all operations.
	// Useful in tests and disabled features.
	m := metricsx.Noop()
	m.NewCounter("test", "test counter")
	m.IncrementCounter(context.Background(), "test")
	fmt.Println("ok")
	// Output: ok
}

func ExampleNewPrometheusManager() {
	// NewPrometheusManager creates a Prometheus-backed Manager in one call.
	// (Not run in example to avoid Prometheus registry conflicts.)
	_ = metricsx.NewPrometheusManager
	fmt.Println("see signature")
	// Output: see signature
}

// ============================================================================
// Registration EXAMPLES
// ============================================================================

func ExampleManager_NewCounter() {
	m := metricsx.Noop()
	m.NewCounter("skill_create_total", "Total skills created")
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_NewUpDownCounter() {
	m := metricsx.Noop()
	m.NewUpDownCounter("active_sessions", "Active sessions")
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_NewHistogram() {
	m := metricsx.Noop()
	// Buckets are optional; if provided, override SDK defaults.
	m.NewHistogram("request_seconds", "Request latency",
		0.005, 0.01, 0.05, 0.1, 0.5, 1, 5,
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_NewHistogram_defaultBuckets() {
	m := metricsx.Noop()
	// Use DefaultBuckets for standard latency histograms.
	m.NewHistogram("request_seconds", "Request latency",
		metricsx.DefaultBuckets...,
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_NewGauge() {
	m := metricsx.Noop()
	m.NewGauge("memory_usage_bytes", "Current memory usage in bytes")
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// Recording EXAMPLES
// ============================================================================

func ExampleManager_IncrementCounter() {
	m := metricsx.Noop()
	m.NewCounter("requests_total", "Total requests")

	// Without labels
	m.IncrementCounter(context.Background(), "requests_total")

	// With labels (key-value pairs)
	m.IncrementCounter(context.Background(), "requests_total",
		"method", "GET",
		"status", "200",
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_DeltaUpDownCounter() {
	m := metricsx.Noop()
	m.NewUpDownCounter("active_sessions", "Active sessions")

	// Increment
	m.DeltaUpDownCounter(context.Background(), "active_sessions", 1,
		"tenant", "t_acme",
	)
	// Decrement (negative value)
	m.DeltaUpDownCounter(context.Background(), "active_sessions", -1,
		"tenant", "t_acme",
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_RecordHistogram() {
	m := metricsx.Noop()
	m.NewHistogram("request_seconds", "Request latency", metricsx.DefaultBuckets...)

	// Record a latency observation
	m.RecordHistogram(context.Background(), "request_seconds", 0.042,
		"operation", "find_skill",
		"status", "200",
	)
	fmt.Println("ok")
	// Output: ok
}

func ExampleManager_SetGauge() {
	m := metricsx.Noop()
	m.NewGauge("active_sessions", "Active sessions")

	// Set current value (replaces previous)
	m.SetGauge("active_sessions", 42,
		"tenant", "t_acme",
	)
	m.SetGauge("active_sessions", 38,
		"tenant", "t_acme",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// nil safety EXAMPLES
// ============================================================================

func ExampleManager_nilSafety() {
	// A nil Manager is a valid no-op — methods don't panic.
	// This happens when NewManager(nil, nil) is called.
	m := metricsx.NewManager(nil, nil)

	m.NewCounter("test", "test")                            // no-op
	m.IncrementCounter(context.Background(), "test")        // no-op
	m.DeltaUpDownCounter(context.Background(), "test", 1.0) // no-op
	m.RecordHistogram(context.Background(), "test", 0.5)    // no-op
	m.SetGauge("test", 1.0)                                 // no-op

	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// System metrics EXAMPLES
// ============================================================================

func ExampleRegisterSystemMetrics() {
	m := metricsx.Noop()
	metricsx.RegisterSystemMetrics(m)
	// Now app_go_goroutines, app_sys_memory_alloc, etc. are registered.
	fmt.Println("ok")
	// Output: ok
}

func ExampleGetHandler() {
	m := metricsx.Noop()
	handler := metricsx.GetHandler(m)
	if handler != nil {
		fmt.Println("ok")
	}
	// Output: ok
}

// ============================================================================
// DefaultBuckets EXAMPLE
// ============================================================================

func ExampleDefaultBuckets() {
	// DefaultBuckets for latency histograms in seconds.
	fmt.Println(len(metricsx.DefaultBuckets) > 0)
	// Output: true
}

// ============================================================================
// Errors EXAMPLES
// ============================================================================

func ExampleErrNotRegistered() {
	err := &metricsx.ErrNotRegistered{Name: "missing_metric"}
	fmt.Println(err)
	// Output: metricsx: metric "missing_metric" not registered
}

func ExampleErrAlreadyRegistered() {
	err := &metricsx.ErrAlreadyRegistered{Name: "duplicate_metric"}
	fmt.Println(err)
	// Output: metricsx: metric "duplicate_metric" already registered
}
