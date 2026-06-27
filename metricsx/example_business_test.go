package metricsx_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/metricsx"
)

// This file demonstrates COMPLETE business scenarios showing how metricsx
// fits into handler → service → repository flow. AI tools should copy these
// patterns when generating new business code.

// ============================================================================
// SCENARIO 1: Bootstrap metrics at startup
// ============================================================================

// Example_businessBootstrap shows the standard startup pattern: create a
// Prometheus-backed Manager, register all metrics, expose /metrics endpoint.
func Example_businessBootstrap() {
	// In real code (cannot run here due to Prometheus registry):
	//   m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)
	//   metricsx.RegisterSystemMetrics(m)
	//   m.NewCounter("skill_create_total", "Total skills created")
	//   m.NewHistogram("skill_query_seconds", "Skill query latency", metricsx.DefaultBuckets...)
	//   m.NewGauge("active_sessions", "Active sessions")
	//   http.Handle("/metrics", metricsx.GetHandler(m))

	m := metricsx.Noop() // stand-in for example
	m.NewCounter("skill_create_total", "Total skills created")
	m.NewHistogram("skill_query_seconds", "Skill query latency", metricsx.DefaultBuckets...)
	m.NewGauge("active_sessions", "Active sessions")
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 2: Counter — count business events
// ============================================================================

// Example_businessCounter shows counting business events with labels.
func Example_businessCounter() {
	m := metricsx.Noop()
	m.NewCounter("skill_create_total", "Total skills created")
	m.NewCounter("skill_delete_total", "Total skills deleted")

	// In handler: count with labels
	m.IncrementCounter(context.Background(), "skill_create_total",
		"tenant", "t_acme",
		"visibility", "public",
	)
	m.IncrementCounter(context.Background(), "skill_delete_total",
		"tenant", "t_acme",
		"reason", "archived",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 3: Histogram — record latency
// ============================================================================

// Example_businessHistogram shows recording request latency.
func Example_businessHistogram() {
	m := metricsx.Noop()
	m.NewHistogram("request_seconds", "Request latency", metricsx.DefaultBuckets...)

	start := time.Now()
	// ... business logic ...
	latency := time.Since(start).Seconds()

	m.RecordHistogram(context.Background(), "request_seconds", latency,
		"operation", "find_skill",
		"status", "200",
		"tenant", "t_acme",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 4: Gauge — current state
// ============================================================================

// Example_businessGauge shows tracking current state (queue depth, active
// sessions, etc.).
func Example_businessGauge() {
	m := metricsx.Noop()
	m.NewGauge("queue_depth", "Current queue depth")

	// Worker reports current queue size periodically
	queueSize := 42
	m.SetGauge("queue_depth", float64(queueSize),
		"queue", "skill_publish",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 5: UpDownCounter — increment/decrement
// ============================================================================

// Example_businessUpDownCounter shows tracking values that go up and down
// (active sessions, in-flight requests).
func Example_businessUpDownCounter() {
	m := metricsx.Noop()
	m.NewUpDownCounter("active_sessions", "Active sessions")

	// Session start: +1
	m.DeltaUpDownCounter(context.Background(), "active_sessions", 1,
		"tenant", "t_acme",
	)
	// Session end: -1
	m.DeltaUpDownCounter(context.Background(), "active_sessions", -1,
		"tenant", "t_acme",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 6: nil Manager in tests
// ============================================================================

// Example_businessNilManagerInTests shows that nil Manager is safe — tests
// can pass nil without nil-checks everywhere.
func Example_businessNilManagerInTests() {
	// Test setup: pass nil Manager
	var m metricsx.Manager // nil
	m = metricsx.NewManager(nil, nil)

	// Business code calls methods without nil-checks:
	m.IncrementCounter(context.Background(), "test_counter")
	m.RecordHistogram(context.Background(), "test_hist", 0.5)
	m.SetGauge("test_gauge", 1.0)

	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 7: System metrics on /metrics scrape
// ============================================================================

// Example_businessSystemMetrics shows that GetHandler auto-records Go runtime
// metrics (goroutines, memory) on each /metrics scrape.
func Example_businessSystemMetrics() {
	m := metricsx.Noop()
	metricsx.RegisterSystemMetrics(m)

	// When Prometheus scrapes /metrics, these are auto-updated:
	//   app_go_goroutines
	//   app_sys_memory_alloc
	//   app_sys_total_alloc
	//   app_go_num_gc
	//   app_go_sys

	handler := metricsx.GetHandler(m)
	fmt.Println(handler != nil)
	// Output: true
}

// ============================================================================
// SCENARIO 8: Multi-tenant metrics with labels
// ============================================================================

// Example_businessMultiTenant shows per-tenant metric segmentation via labels.
func Example_businessMultiTenant() {
	m := metricsx.Noop()
	m.NewCounter("api_requests_total", "Total API requests")
	m.NewHistogram("api_request_seconds", "API request latency", metricsx.DefaultBuckets...)

	// Per-tenant recording
	tenants := []string{"t_acme", "t_globex", "t_initech"}
	for _, tenant := range tenants {
		m.IncrementCounter(context.Background(), "api_requests_total",
			"tenant", tenant,
			"endpoint", "/v1/skills",
		)
		m.RecordHistogram(context.Background(), "api_request_seconds", 0.05,
			"tenant", tenant,
			"endpoint", "/v1/skills",
		)
	}
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 9: Error metrics
// ============================================================================

// Example_businessErrorMetrics shows counting errors by error_code for
// alerting and dashboards.
func Example_businessErrorMetrics() {
	m := metricsx.Noop()
	m.NewCounter("errors_total", "Total errors by code")

	// When business code returns an errorx error:
	//   err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "...")
	//   m.IncrementCounter(ctx, "errors_total",
	//       "error_code", errorx.CodeOf(err).String(),
	//       "http_status", fmt.Sprintf("%d", errorx.HTTPStatusOf(err)),
	//   )

	m.IncrementCounter(context.Background(), "errors_total",
		"error_code", "AIHUB_SKILL_NOT_FOUND",
		"http_status", "404",
	)
	m.IncrementCounter(context.Background(), "errors_total",
		"error_code", "AIHUB_SKILL_QUERY_FAILED",
		"http_status", "500",
	)
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 10: Cardinality warning
// ============================================================================

// Example_businessCardinalityWarning shows that metricsx warns when too many
// labels are passed (potential Prometheus cardinality explosion).
func Example_businessCardinalityWarning() {
	m := metricsx.Noop()
	m.NewCounter("high_cardinality_test", "test")

	// 22 labels — triggers a warning log (but still records)
	m.IncrementCounter(context.Background(), "high_cardinality_test",
		"k1", "v1", "k2", "v2", "k3", "v3", "k4", "v4", "k5", "v5",
		"k6", "v6", "k7", "v7", "k8", "v8", "k9", "v9", "k10", "v10",
		"k11", "v11",
	)
	fmt.Println("ok")
	// Output: ok
}
