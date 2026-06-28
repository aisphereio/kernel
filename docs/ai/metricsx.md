# metricsx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的指标处理规范。**只看本文件即可写对所有场景。**

---

## 0. 一句话规则

> 业务代码注册和记录指标时，**必须**使用 `github.com/aisphereio/kernel/metricsx`。
> **禁止**直接使用 `go.opentelemetry.io/otel/metric` 在业务代码中。

---

## 1. 速查：什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| 启动时创建 Manager | `metricsx.NewPrometheusManager(app, ver, logger)` | main.go |
| 注册计数器 | `m.NewCounter(name, desc)` | 启动时 |
| 注册直方图 | `m.NewHistogram(name, desc, buckets...)` | 启动时 |
| 注册 Gauge | `m.NewGauge(name, desc)` | 启动时 |
| 注册 UpDownCounter | `m.NewUpDownCounter(name, desc)` | 启动时 |
| 计数 +1 | `m.IncrementCounter(ctx, name, labels...)` | 业务代码 |
| 记录延迟 | `m.RecordHistogram(ctx, name, value, labels...)` | 业务代码 |
| 设置当前值 | `m.SetGauge(name, value, labels...)` | 业务代码 |
| 增减值 | `m.DeltaUpDownCounter(ctx, name, value, labels...)` | 业务代码 |
| 暴露 /metrics | `metricsx.GetHandler(m)` | HTTP server |
| 注册系统指标 | `metricsx.RegisterSystemMetrics(m)` | 启动时 |
| 测试用 Noop | `metricsx.Noop()` | 测试代码 |

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 启动时 bootstrap

```go
func main() {
    m := metricsx.NewPrometheusManager("aihub", "v1.0.0", logger)
    metricsx.RegisterSystemMetrics(m)
    m.NewCounter("skill_create_total", "Total skills created")
    m.NewHistogram("skill_query_seconds", "Skill query latency",
        metricsx.DefaultBuckets...)
    m.NewGauge("active_sessions", "Active sessions")

    http.Handle("/metrics", metricsx.GetHandler(m))
    go http.ListenAndServe(":9000", nil)
    // ... start app ...
}
```

### 2.2 计数业务事件

```go
m.IncrementCounter(ctx, "skill_create_total",
    "tenant", "t_acme",
    "visibility", "public",
)
```

### 2.3 记录请求延迟

```go
start := time.Now()
// ... business logic ...
m.RecordHistogram(ctx, "request_seconds", time.Since(start).Seconds(),
    "operation", "find_skill",
    "status", "200",
)
```

### 2.4 设置当前状态（Gauge）

```go
m.SetGauge("queue_depth", float64(queueSize),
    "queue", "skill_publish",
)
```

### 2.5 增减值（UpDownCounter）

```go
// Session start
m.DeltaUpDownCounter(ctx, "active_sessions", 1, "tenant", "t_acme")
// Session end
m.DeltaUpDownCounter(ctx, "active_sessions", -1, "tenant", "t_acme")
```

### 2.6 错误计数（按 error_code）

```go
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

### 2.7 多租户标签

```go
m.IncrementCounter(ctx, "api_requests_total",
    "tenant", tenantID,
    "endpoint", "/v1/skills",
    "method", "GET",
)
```

### 2.8 测试中用 Noop

```go
func TestCreateSkill(t *testing.T) {
    svc := NewService(metricsx.Noop())  // discard metrics in tests
    // ...
}
```

### 2.9 nil Manager 安全

```go
// If metrics disabled in config, m might be nil Manager — still safe:
m.IncrementCounter(ctx, "test")  // no-op, no panic
m.SetGauge("test", 1.0)          // no-op, no panic
```

### 2.10 系统指标自动记录

```go
// RegisterSystemMetrics + GetHandler auto-records on each /metrics scrape:
//   app_go_goroutines
//   app_sys_memory_alloc
//   app_sys_total_alloc
//   app_go_num_gc
//   app_go_sys
```

---

## 3. 指标类型选择

| 类型 | 何时用 | 示例 |
|---|---|---|
| Counter | 单调递增的计数 | 请求总数、错误总数、事件总数 |
| UpDownCounter | 可增可减的累计值 | 活跃会话、in-flight 请求 |
| Histogram | 分布型数据 | 延迟、payload 大小 |
| Gauge | 当前快照值 | 队列深度、连接数、内存使用 |

---

## 4. 标签规则

### 4.1 低基数标签（✅ 允许）

```go
"method", "GET"           // 几十种
"status", "200"           // 几十种
"tenant", "t_acme"        // 几百种
"operation", "find_skill" // 几百种
"error_code", "AIHUB_SKILL_NOT_FOUND"  // 几百种
```

### 4.2 高基数标签（❌ 禁止）

```go
"request_id", reqID       // 每个请求一个 → 基数爆炸
"user_id", userID         // 每个用户一个 → 基数爆炸
"skill_id", skillID       // 每个 skill 一个 → 基数爆炸
"trace_id", traceID       // 每个链路一个 → 基数爆炸
```

**铁律**：高基数数据用 logx（日志）或 otel（tracing），不用 metricsx。

metricsx 在标签数 > 20 时会 warning，但不会阻止记录。

---

## 5. 禁止模式

### 5.1 业务代码禁止

```go
// ❌ 直接用 otel/metric
meter.Int64Counter("requests_total", metric.WithDescription("..."))

// ❌ 高基数标签
m.IncrementCounter(ctx, "requests_total",
    "request_id", reqID,
    "user_id", userID,
)

// ❌ 未注册就记录（静默失败）
m.IncrementCounter(ctx, "never_registered")

// ❌ 不必要的 nil 检查
if m != nil {
    m.IncrementCounter(ctx, "test")
}
```

### 5.2 替代写法

```go
// ✅ 启动注册，业务记录
m.NewCounter("requests_total", "Total requests")
m.IncrementCounter(ctx, "requests_total", "method", "GET", "status", "200")

// ✅ 只用低基数标签
m.IncrementCounter(ctx, "requests_total",
    "method", "GET",
    "status", "200",
    "tenant", "t_acme",
)

// ✅ 无需 nil 检查
m.IncrementCounter(ctx, "test")  // 即使 m 是 nil 也安全
```

---

## 6. 完整 handler 示例

```go
package handler

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/aisphereio/kernel/errorx"
    "github.com/aisphereio/kernel/metricsx"
)

type SkillHandler struct {
    metrics metricsx.Manager
    svc     SkillService
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    start := time.Now()

    skill, err := h.svc.Create(ctx, req)
    latency := time.Since(start).Seconds()

    if err != nil {
        h.metrics.IncrementCounter(ctx, "skill_create_total",
            "result", "error",
            "error_code", errorx.CodeOf(err).String(),
        )
        h.metrics.RecordHistogram(ctx, "skill_create_seconds", latency,
            "result", "error",
        )
        writeError(w, err)
        return
    }

    h.metrics.IncrementCounter(ctx, "skill_create_total",
        "result", "success",
        "visibility", skill.Visibility,
    )
    h.metrics.RecordHistogram(ctx, "skill_create_seconds", latency,
        "result", "success",
    )
    writeJSON(w, 201, skill)
}
```

---

## 7. 验收清单

- [ ] 所有指标用 metricsx.Manager，不直接用 otel/metric
- [ ] 启动时注册所有指标（NewCounter/NewHistogram/NewGauge）
- [ ] 标签只用低基数维度（method/status/tenant/operation/error_code）
- [ ] 不把 request_id/user_id/skill_id 放进标签
- [ ] 测试用 `metricsx.Noop()` 注入
- [ ] `/metrics` endpoint 通过 `metricsx.GetHandler(m)` 暴露
- [ ] `go test ./...` 通过
- [ ] `golangci-lint run` 通过

---

## 8. 相关文档

- `metricsx/README.md` — 单一入口用户指南
- `metricsx/doc.go` — `go doc metricsx` 输出源
- `metricsx/example_test.go` — 所有 API 的 Go 标准 Example
- `metricsx/example_business_test.go` — 10 个完整业务场景
- `examples/metricsx-basic/` — 最小可运行示例


## Kernel app bootstrap

For Kernel services, prefer app-level wiring instead of creating separate metrics
managers per component:

```go
app := kernel.New(
    kernel.Name("aihub"),
    kernel.Version(version),
    kernel.LogxLogger(logger),
    kernel.PrometheusMetrics(":9090"),
)
```

Inside lifecycle hooks:

```go
manager := kernel.MetricsFromContext(ctx)
```

Then pass `manager` into dbx/cachex/objectstorex/authn/authz configs with
`MetricsEnabled: true`. See `docs/ai/observability-metrics.md` for the full
startup pattern.
