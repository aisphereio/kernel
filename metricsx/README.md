# metricsx

`metricsx` is the Aisphere Kernel metrics package. It is the **recommended**
package for registering and recording metrics (counters, up-down counters,
histograms, gauges) in business code. It wraps `go.opentelemetry.io/otel/metric`
with a simpler API and built-in safety: nil-safe, low-cardinality validation,
automatic system metrics, Prometheus one-liner.

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 metricsx

Before metricsx, business code used otel/metric directly:

- Verbose registration (`meter.Int64Counter(name, metric.WithDescription(desc))`)
- No nil safety — passing nil meter panicked
- No cardinality validation — easy to cause Prometheus label explosion
- No system metrics — had to manually register goroutine/memory gauges
- No Prometheus one-liner — had to wire exporter + meter provider + handler

metricsx provides a `Manager` interface with clean methods, nil safety,
cardinality warnings, and a `NewPrometheusManager` one-liner.

```text
metricsx (only defines metric registration/recording)
  ↓ stable contract (Manager interface)
handler   → IncrementCounter(ctx, "requests_total", "method", "GET")
service   → RecordHistogram(ctx, "query_seconds", latency, "op", "find")
worker    → SetGauge("queue_depth", 42, "queue", "publish")
startup   → NewPrometheusManager + GetHandler → /metrics endpoint
```

metricsx itself **does not** do logging, tracing, or transport middleware.
It only manages metric instruments.

---

## 2. 30-second quickstart

```go
package main

import (
    "net/http"

    "github.com/aisphereio/kernel/metricsx"
)

func main() {
    // Bootstrap: Prometheus-backed Manager in one call
    m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)

    // Register metrics at startup
    metricsx.RegisterSystemMetrics(m)  // goroutines, memory, GC
    m.NewCounter("skill_create_total", "Total skills created")
    m.NewHistogram("skill_query_seconds", "Skill query latency",
        metricsx.DefaultBuckets...)
    m.NewGauge("active_sessions", "Active sessions")

    // Expose /metrics + /debug/pprof/*
    http.Handle("/metrics", metricsx.GetHandler(m))
    go http.ListenAndServe(":9000", nil)

    // In business code:
    m.IncrementCounter(ctx, "skill_create_total", "tenant", "t_acme")
    m.RecordHistogram(ctx, "skill_query_seconds", 0.042, "operation", "find")
    m.SetGauge("active_sessions", 42, "tenant", "t_acme")
}
```

---

## 3. Manager API cheatsheet

### Bootstrap

| Function | Use for |
|---|---|
| `NewPrometheusManager(app, ver, logger)` | One-liner Prometheus Manager |
| `NewPrometheusMeter(app, ver)` | Just the otel Meter (use with NewManager) |
| `NewManager(meter, logger)` | Wrap any otel Meter |
| `Noop()` | Discard all metrics (tests, disabled features) |

### Registration (call at startup)

| Method | Instrument type |
|---|---|
| `m.NewCounter(name, desc)` | Monotonic int64 counter |
| `m.NewUpDownCounter(name, desc)` | float64, can go up or down |
| `m.NewHistogram(name, desc, buckets...)` | float64 distribution |
| `m.NewGauge(name, desc)` | float64 current value |

### Recording (call in business code)

| Method | What it does |
|---|---|
| `m.IncrementCounter(ctx, name, labels...)` | +1 |
| `m.DeltaUpDownCounter(ctx, name, value, labels...)` | +/- value |
| `m.RecordHistogram(ctx, name, value, labels...)` | Observe value |
| `m.SetGauge(name, value, labels...)` | Set current value |

Labels are key-value pairs: `"key1", "value1", "key2", "value2"`.

---

## 4. Instrument types

### Counter (monotonic)

```go
m.NewCounter("requests_total", "Total requests")
m.IncrementCounter(ctx, "requests_total", "method", "GET", "status", "200")
```

Use for: request counts, event counts, error counts. **Never decreases.**

### UpDownCounter (can decrease)

```go
m.NewUpDownCounter("active_sessions", "Active sessions")
m.DeltaUpDownCounter(ctx, "active_sessions", 1, "tenant", "t_acme")   // +1
m.DeltaUpDownCounter(ctx, "active_sessions", -1, "tenant", "t_acme")  // -1
```

Use for: active sessions, in-flight requests, queue size (when you need the
delta pattern rather than absolute set).

### Histogram (distribution)

```go
m.NewHistogram("request_seconds", "Request latency",
    metricsx.DefaultBuckets...)  // 0.005, 0.01, 0.025, ..., 10
m.RecordHistogram(ctx, "request_seconds", 0.042, "operation", "find")
```

Use for: latencies, payload sizes, any distribution. Buckets define the
boundaries for aggregation.

### Gauge (current value)

```go
m.NewGauge("queue_depth", "Current queue depth")
m.SetGauge("queue_depth", 42, "queue", "skill_publish")
```

Use for: current state that can go up or down (queue depth, connection count,
memory usage). Each `SetGauge` replaces the previous value.

---

## 5. nil safety

A nil Manager is a valid no-op. All methods are safe to call on nil:

```go
var m metricsx.Manager  // nil
m.IncrementCounter(ctx, "test")  // no-op, no panic
m.SetGauge("test", 1.0)          // no-op, no panic
```

This happens when `NewManager(nil, nil)` is called (e.g. metrics disabled in
config). Business code can pass the Manager through without nil-checks.

Use `Noop()` explicitly for tests:

```go
func TestCreateSkill(t *testing.T) {
    svc := NewService(metricsx.Noop())  // discard metrics in tests
    // ...
}
```

---

## 6. Labels and cardinality

Labels are key-value pairs passed as varargs:

```go
m.IncrementCounter(ctx, "requests_total",
    "method", "GET",        // label 1
    "status", "200",        // label 2
    "tenant", "t_acme",     // label 3
)
```

**Cardinality warning**: metricsx logs a warning if you pass more than 20
labels (potential Prometheus cardinality explosion). The metric is still
recorded, but you'll see a warning in logs.

**Best practices**:
- Keep label count under 10
- Never put dynamic IDs (user_id, request_id, skill_id) in labels — use
  metricsx only for low-cardinality dimensions (method, status, tenant,
  operation, error_code)
- For high-cardinality data, use logs (logx) or tracing (otel), not metrics

---

## 7. System metrics

`RegisterSystemMetrics(m)` registers 5 standard Go runtime gauges:

| Metric | Description |
|---|---|
| `app_go_goroutines` | Number of goroutines |
| `app_sys_memory_alloc` | Allocated memory in bytes |
| `app_sys_total_alloc` | Total allocated memory in bytes |
| `app_go_num_gc` | Number of GC cycles |
| `app_go_sys` | Memory obtained from OS in bytes |

These are auto-updated on each `/metrics` scrape via `GetHandler`.

---

## 8. /metrics endpoint

`GetHandler(m)` returns an HTTP handler serving:
- `/metrics` — Prometheus exposition format (with auto system metrics)
- `/debug/pprof/*` — Go pprof profiling endpoints

```go
http.Handle("/metrics", metricsx.GetHandler(m))
http.ListenAndServe(":9000", nil)
```

Scrape with Prometheus or curl:

```bash
curl http://localhost:9000/metrics
```

---

## 9. DefaultBuckets

`DefaultBuckets` provides sensible histogram bucket boundaries for request
latency in seconds:

```go
m.NewHistogram("request_seconds", "Request latency",
    metricsx.DefaultBuckets...)
// Buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
```

For non-latency histograms, provide custom buckets:

```go
m.NewHistogram("payload_bytes", "Payload size in bytes",
    100, 1000, 10000, 100000, 1000000,
)
```

---

## 10. Forbidden patterns (AI 必读)

```go
// ❌ Use otel/metric directly in business code
meter.Int64Counter("requests_total", metric.WithDescription("..."))

// ❌ Put dynamic IDs in labels (cardinality explosion)
m.IncrementCounter(ctx, "requests_total",
    "request_id", reqID,     // ❌ high cardinality
    "user_id", userID,       // ❌ high cardinality
)

// ❌ Skip registration, then record (silently fails)
m.IncrementCounter(ctx, "never_registered")  // logs error, no-ops

// ❌ Nil-check before every call (unnecessary — nil Manager is safe)
if m != nil {
    m.IncrementCounter(ctx, "test")
}
```

Required:

```go
// ✅ Register at startup, record in business code
m.NewCounter("requests_total", "Total requests")
m.IncrementCounter(ctx, "requests_total", "method", "GET", "status", "200")

// ✅ Low-cardinality labels only
m.IncrementCounter(ctx, "requests_total",
    "method", "GET",
    "status", "200",
    "tenant", "t_acme",
)

// ✅ No nil-check needed (nil Manager is no-op)
m.IncrementCounter(ctx, "test")  // safe even if m is nil
```

---

## 11. Integration with errorx

Count errors by error_code for alerting:

```go
import "github.com/aisphereio/kernel/errorx"

err := h.svc.Create(ctx, req)
if err != nil {
    m.IncrementCounter(ctx, "errors_total",
        "error_code", errorx.CodeOf(err).String(),
        "http_status", fmt.Sprintf("%d", errorx.HTTPStatusOf(err)),
        "retryable", fmt.Sprintf("%t", errorx.RetryableOf(err)),
    )
    return err
}
```

**Never** put `request_id` / `trace_id` / `skill_id` in labels — those go in
logs (logx) or tracing (otel).

---

## 12. Integration with logx

`metricsx.Logger` is an alias of `logx.Logger` — pass a
`logx.Logger` directly:

```go
import (
    "github.com/aisphereio/kernel/logx"
    "github.com/aisphereio/kernel/metricsx"
)

logger, _, _ := logx.New(cfg)
m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)  // logx.Logger satisfies metricsx.Logger
```

metricsx reuses the Kernel logger contract instead of duplicating log methods.

---

## 13. 测试

```bash
go test ./metricsx -v
go test ./metricsx -race
go test ./metricsx -cover

go run ./examples/metricsx-basic
```

---

## 14. 文档地图

metricsx 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (metricsx/README.md)             ← 你正在看的，单一入口
├── metricsx/doc.go                         ← go doc 输出源
└── metricsx/example_test.go                ← Go 标准示例

深度规范（架构师/PR review 时看）
└── docs/contracts/metricsx.md              ← 不可破坏契约

AI 编码指南（AI 写业务代码时看）
├── docs/ai/metricsx.md                     ← 合并版 AI 指南
└── AGENTS.md                               ← 项目级 AI 规则

可运行示例
└── examples/metricsx-basic/                ← 最小示例
```

**优先级**：日常开发只看本 README + `docs/ai/metricsx.md` 即可。

---

## 15. Examples 索引（按场景查找）

### Bootstrap

| Function | Example |
|---|---|
| `NewManager` | `ExampleNewManager` |
| `Noop` | `ExampleNoop` |
| `NewPrometheusManager` | `ExampleNewPrometheusManager` |

### Registration

| Method | Example |
|---|---|
| `NewCounter` | `ExampleManager_NewCounter` |
| `NewUpDownCounter` | `ExampleManager_NewUpDownCounter` |
| `NewHistogram` | `ExampleManager_NewHistogram`, `ExampleManager_NewHistogram_defaultBuckets` |
| `NewGauge` | `ExampleManager_NewGauge` |

### Recording

| Method | Example |
|---|---|
| `IncrementCounter` | `ExampleManager_IncrementCounter` |
| `DeltaUpDownCounter` | `ExampleManager_DeltaUpDownCounter` |
| `RecordHistogram` | `ExampleManager_RecordHistogram` |
| `SetGauge` | `ExampleManager_SetGauge` |

### nil safety

| Topic | Example |
|---|---|
| nil Manager no-op | `ExampleManager_nilSafety` |

### System metrics

| Function | Example |
|---|---|
| `RegisterSystemMetrics` | `ExampleRegisterSystemMetrics` |
| `GetHandler` | `ExampleGetHandler` |

### Constants

| Topic | Example |
|---|---|
| `DefaultBuckets` | `ExampleDefaultBuckets` |

### Errors

| Type | Example |
|---|---|
| `ErrNotRegistered` | `ExampleErrNotRegistered` |
| `ErrAlreadyRegistered` | `ExampleErrAlreadyRegistered` |

### Business scenarios (in `example_business_test.go`)

| Scenario | Example |
|---|---|
| Bootstrap at startup | `Example_businessBootstrap` |
| Counter — count events | `Example_businessCounter` |
| Histogram — record latency | `Example_businessHistogram` |
| Gauge — current state | `Example_businessGauge` |
| UpDownCounter — inc/dec | `Example_businessUpDownCounter` |
| nil Manager in tests | `Example_businessNilManagerInTests` |
| System metrics on scrape | `Example_businessSystemMetrics` |
| Multi-tenant labels | `Example_businessMultiTenant` |
| Error metrics | `Example_businessErrorMetrics` |
| Cardinality warning | `Example_businessCardinalityWarning` |

---

## 16. 设计哲学一句话

> **metricsx is the recommended metrics package. Register at startup, record in business code. nil Manager is safe. Low-cardinality labels only.**
> If you're about to write `meter.Int64Counter(...)` directly or put `user_id` in a label — STOP, use `metricsx.NewCounter` and keep labels low-cardinality.
