// Package metricsx provides business-facing metrics for Aisphere Kernel.
//
// metricsx is the recommended package for registering and recording metrics
// (counters, up-down counters, histograms, gauges) in business code. It wraps
// go.opentelemetry.io/otel/metric with a simpler API and built-in safety:
//
//   - Nil-safe: all methods on a nil Manager are no-ops
//   - Low-cardinality label validation (warns on > 20 labels)
//   - Standard system metrics registration helpers (goroutines, memory)
//   - Prometheus exporter one-liner via NewPrometheusManager
//   - Logger type reuses logx.Logger
//
// # Design principle
//
// metricsx only DEFINES metric registration and recording. It does NOT do
// logging (use logx), NOT do tracing (use contrib/otel/tracing), NOT do
// transport middleware (use contrib/otel/metrics for that). It only manages
// metric instruments.
//
// metricsx depends on logx, go.opentelemetry.io/otel/metric, the Prometheus
// exporter, and the Go standard library. It does NOT import errorx or transport.
//
// # 30-second quickstart
//
//	// Bootstrap: create a Prometheus-backed manager
//	m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)
//
//	// Register metrics at startup
//	m.NewCounter("skill_create_total", "Total skills created")
//	m.NewHistogram("skill_query_seconds", "Skill query latency", 0.01, 0.1, 1, 10)
//	m.NewGauge("active_sessions", "Active sessions")
//
//	// Record in business code
//	m.IncrementCounter(ctx, "skill_create_total", "tenant", "t_acme")
//	m.RecordHistogram(ctx, "skill_query_seconds", 0.042, "operation", "find")
//	m.SetGauge("active_sessions", 42, "tenant", "t_acme")
//
//	// Expose /metrics endpoint
//	http.Handle("/metrics", metricsx.GetHandler(m))
//
// # Instrument types
//
//	m.NewCounter(name, desc)                        // monotonic, int64
//	m.NewUpDownCounter(name, desc)                  // can go up or down, float64
//	m.NewHistogram(name, desc, buckets...)          // distribution, float64
//	m.NewGauge(name, desc)                          // current value, float64
//
// # Recording
//
//	m.IncrementCounter(ctx, name, labels...)        // +1
//	m.DeltaUpDownCounter(ctx, name, value, labels...) // +/- value
//	m.RecordHistogram(ctx, name, value, labels...)  // observe value
//	m.SetGauge(name, value, labels...)              // set current value
//
// Labels are key-value pairs: "key1", "value1", "key2", "value2".
//
// # nil safety
//
// All methods on a nil Manager are no-ops. This means you can pass a nil
// Manager to test code or disabled features without nil-checks everywhere:
//
//	var m metricsx.Manager  // nil
//	m.IncrementCounter(ctx, "x")  // no-op, no panic
//
// # Further reading
//
// See metricsx/README.md for the single-source-of-truth user guide, and
// docs/ai/metricsx.md for the AI coding recipe.
package metricsx
