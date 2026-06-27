# logx

`logx` is the Aisphere Kernel logging package. It is the **only** logger that
business code (handler/service/repository/worker) should use. Direct usage of
`log.Printf`, `fmt.Println`, `log/slog`, or `go.uber.org/zap` in business code
is forbidden and will fail CI.

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 logx

Before logx, Go services used `log.Printf` / `fmt.Println` / direct `slog` /
direct `zap` ad-hoc, which caused:

- No structured fields → Loki/ELK/ClickHouse can't index
- No request-scoped correlation → can't trace a request across services
- No redaction → tokens / passwords leaked to logs
- No sampling → noisy logs at high QPS
- No level control → can't dynamically raise level under load
- Each module invented its own logger → inconsistent log shape

logx solves all of this with a single `Logger` interface plus typed `Field`
values. It keeps `log/slog` as the internal engine (so Kernel is stdlib-only),
but exposes a Kernel-facing API that does not leak `slog.Logger` or `slog.Attr`
to business packages.

```text
logx (only defines logging API)
  ↓ stable contract (Logger interface + Field type)
errorx   → logx.Err(err) auto-extracts error_code / http_status / retryable
httpx    → logx.HTTPAccessLog middleware
grpcx    → logx.ServerLogging / ClientLogging middleware
auditx   → logx.LogAuditHint breadcrumb
metricsx → consumes structured fields as Prometheus labels
```

logx itself **does not** call `os.Exit`, does not write to network, does not
do audit-grade recording. It only emits structured logs; other modules consume.

---

## 2. 30-second quickstart

```go
package main

import "github.com/aisphereio/kernel/logx"

func main() {
    cfg := logx.DefaultConfig("dev")
    cfg.ServiceName = "aihub"
    logger, levelCtl, err := logx.New(cfg)
    if err != nil { panic(err) }
    defer logger.Sync()

    logger.Info("service started",
        logx.String("addr", ":8000"),
        logx.String("version", "v0.1.0"),
    )

    _ = levelCtl.SetLevel("debug")  // dynamically lower to debug
}
```

In a handler with request-scoped context:

```go
ctx = logx.Inject(ctx, logger,
    logx.String("request_id", reqID),
    logx.String("trace_id", traceID),
    logx.String("subject_id", userID),
)
logx.FromContext(ctx).Info("request accepted")
```

---

## 3. Logger API cheatsheet

| Method | Use for |
|---|---|
| `logx.New(cfg)` | Bootstrap the logger (returns Logger + LevelController) |
| `logx.DefaultConfig(env)` | Get sane defaults for dev/staging/prod |
| `logx.Noop()` | Discard all logs (tests, disabled features) |
| `logx.FromContext(ctx)` | Get request-scoped logger (Noop if none) |
| `logx.FromContextOr(ctx, fallback)` | Get request logger or fallback |
| `logx.Inject(ctx, logger, fields...)` | Attach logger + fields to ctx |
| `logx.Sync(logger)` | Flush buffers before exit |
| `logx.Slog(logger)` | Unwrap to `*slog.Logger` (adapters only) |

Logger interface methods:

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger    // add persistent fields
    Named(name string) Logger       // add module tag
    WithContext(ctx context.Context) Logger
    Enabled(level LogLevel) bool
    Sync() error
}
```

---

## 4. Field constructors

All structured fields use these constructors — never `slog.Any` directly:

| Constructor | Type | Example |
|---|---|---|
| `logx.String(k, v)` | string | `logx.String("user_id", "u_123")` |
| `logx.Int(k, v)` | int | `logx.Int("status", 404)` |
| `logx.Int64(k, v)` | int64 | `logx.Int64("bytes", 1024)` |
| `logx.Uint64(k, v)` | uint64 | `logx.Uint64("count", 100)` |
| `logx.Bool(k, v)` | bool | `logx.Bool("enabled", true)` |
| `logx.Float64(k, v)` | float64 | `logx.Float64("rate", 0.95)` |
| `logx.Duration(k, v)` | time.Duration | `logx.Duration("latency", 42*time.Millisecond)` (auto-adds `_ms`) |
| `logx.Time(k, v)` | time.Time | `logx.Time("created_at", t)` |
| `logx.Any(k, v)` | any | fallback for complex types |
| `logx.Event(name)` | string ("event" key) | `logx.Event("skill_created")` |
| `logx.Err(err)` | error | `logx.Err(err)` (auto-extracts errorx fields) |
| `logx.Group(k, fields...)` | nested fields | `logx.Group("user", logx.String("id", "u_1"))` |

**`logx.Err(err)`** is special: it auto-extracts `error_code`, `error_reason`,
`http_status`, `grpc_code`, `retryable` from any error implementing those
methods (works with `errorx` without importing it — no cycle).

---

## 5. Context-scoped fields

The standard pattern for HTTP/RPC handlers:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    ctx = logx.Inject(ctx, h.logger,
        logx.String("request_id", requestIDFromHeader(r)),
        logx.String("trace_id", traceIDFromContext(ctx)),
        logx.String("subject_id", principal.SubjectID),
        logx.String("route", routePattern(r)),
    )
    // All downstream code can fetch the logger with fields attached:
    logx.FromContext(ctx).Info("request started")

    // Service / repo / worker all use FromContext:
    h.svc.DoSomething(ctx, req)
}

// In repository:
func (r *Repo) Find(ctx context.Context, id string) (*X, error) {
    logx.FromContext(ctx).Debug("querying",
        logx.String("table", "skills"),
        logx.String("id", id),
    )
    // ...
}
```

`FromContext(ctx)` returns `Noop()` if no logger was injected — safe to call
in any code path.

---

## 6. Configuration

```go
cfg := logx.DefaultConfig("prod")  // "dev" / "staging" / "prod"
cfg.ServiceName = "aihub"
cfg.Env = "prod"
cfg.Version = "v1.2.3"
cfg.NodeID = "pod-abc-123"
cfg.Level = "info"                  // debug / info / warn / error
cfg.Format = logx.FormatJSON        // "json" or "console"
cfg.Output = "stdout"               // "stdout" / "stderr" / file path
cfg.AddSource = false               // include file:line in prod? usually no
cfg.Redact.Enabled = true           // redact sensitive keys
cfg.Sampling.Enabled = true         // sample noisy logs
cfg.AccessLog.Enabled = true        // HTTP access log middleware
cfg.AccessLog.SlowThreshold = 500 * time.Millisecond
logger, levelCtl, err := logx.New(cfg)
```

`DefaultConfig(env)` returns sensible defaults:

| env | format | level | addSource | sampling |
|---|---|---|---|---|
| `dev` / `local` | console | info | true | off |
| `staging` / `prod` | json | info | false | on |

---

## 7. Redaction

`logx.Redactor` automatically redacts sensitive field keys. Default keys:

```text
password / passwd / pwd / token / access_token / refresh_token / id_token
secret / client_secret / authorization / cookie / set_cookie
api_key / private_key / ak / sk
```

Customize:

```go
cfg.Redact = logx.RedactConfig{
    Enabled: true,
    Keys:    []string{"password", "token", "my_custom_secret"},
    Value:   "***",
}
```

Or use `logx.WithFilter(logx.FilterKey("password", "token"))` when building a
handler directly.

**Forbidden in logs**:

```go
// ❌ NEVER — leaks credentials
logger.Info("login",
    logx.String("password", userPassword),
    logx.String("token", accessToken),
)

// ✅ Use redaction (auto-applied by New with cfg.Redact.Enabled=true)
logger.Info("login",
    logx.String("subject_id", userID),  // safe
)
```

---

## 8. Sampling

High-QPS logs (e.g. per-request debug logs) can overwhelm log pipelines.
Enable sampling:

```go
cfg.Sampling = logx.SamplingConfig{
    Enabled:  true,
    Every:    100,                          // keep 1 of every 100 after First
    First:    10,                           // always keep first 10 per level+message
    Window:   time.Second,                  // reset counters every second
    MinLevel: logx.DebugLevel.String(),     // only sample debug/info; warn+ always kept
}
```

Sampling only applies to logs **at or below** `MinLevel`. Errors and warnings
are never sampled.

---

## 9. Access log

For HTTP request/response access logs:

```go
import "net/http"
import "github.com/aisphereio/kernel/logx"

mux := http.NewServeMux()
mux.HandleFunc("/api", handler)

handler := logx.HTTPAccessLog(logger, logx.AccessLogConfig{
    Enabled:       true,
    SkipPaths:     []string{"/healthz", "/readyz", "/metrics"},
    SlowThreshold: 500 * time.Millisecond,
    RequestIDHeader: "X-Request-ID",
    RouteHeader:   "X-Route-Pattern",
    LogUserAgent:  true,
})(mux)

http.ListenAndServe(":8000", handler)
```

Each request produces one structured access log with: `event=access`,
`method`, `path`, `status`, `latency`, `latency_ms`, `code`, `reason`.

---

## 10. External call log

For logging calls to upstream services (LLM APIs, DB, third-party HTTP):

```go
logx.LogExternalCall(logger, logx.ExternalCall{
    Provider:   "openai",
    Service:    "chat-completions",
    Operation:  "create",
    Model:      "gpt-4",
    Endpoint:   "https://api.openai.com/v1/chat/completions",
    StatusCode: 200,
    Latency:    850 * time.Millisecond,
})
// Outputs: event=external_call provider=openai upstream_service=chat-completions ...
```

If `StatusCode >= 500` or `Err != nil`, automatically logs at ERROR level.
If `StatusCode >= 400`, logs at WARN. Otherwise INFO.

---

## 11. Error log (with errorx auto-extraction)

`logx.Err(err)` automatically extracts structured fields from any error
implementing `Code() / ErrorCode() / HTTPStatus() / StatusCode() / GRPCCode() /
Retryable()`. This works with `errorx` without importing it (no cycle).

```go
err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithRequestID("req_abc"),
)

logger.Error("create skill failed",
    logx.String("operation", "aihub.skill.create"),
    logx.Err(err),
)
// Outputs:
// {
//   "level": "error",
//   "message": "create skill failed",
//   "operation": "aihub.skill.create",
//   "error": "技能不存在",
//   "error_type": "*errorx.Error",
//   "error_code": "AIHUB_SKILL_NOT_FOUND",
//   "http_status": 404,
//   "retryable": false
// }
```

For a standardized error log shape, use `logx.LogError`:

```go
logx.LogError(logger, "create skill failed", logx.ErrorLog{
    Operation:    "aihub.skill.create",
    ResourceType: "skill",
    ResourceID:   skillID,
    Code:         errorx.CodeOf(err).String(),
    Reason:       "db_error",
    Err:          err,
})
```

---

## 12. Audit hint (breadcrumb, not compliance record)

`logx.LogAuditHint` leaves an operational breadcrumb that an auditable
operation happened. **It is not a compliance-grade audit record** — those go
through `auditx` (when it exists).

```go
logx.LogAuditHint(logger, logx.AuditHint{
    Action:       "aihub.skill.delete",
    ActorID:      "user_123",
    ResourceType: "skill",
    ResourceID:   "skill_001",
    Result:       "success",
})
```

---

## 13. Test logger

In tests, use `logx.NewTestLogger` to capture log entries and assert on them:

```go
func TestCreateSkill(t *testing.T) {
    logger := logx.NewTestLogger(t)
    svc := NewService(logger)

    svc.Create(ctx, req)

    logger.AssertLogged(t, "skill created",
        logx.String("skill_id", "skill_001"),
    )
}
```

---

## 14. Drop filters

For dropping specific noisy log entries without lowering the global level:

```go
logger, _, _ := logx.New(cfg,
    logx.WithDropFilter(
        logx.DropEvents("debug_ping"),       // drop entries with event=debug_ping
        logx.DropMessages("noisy library log"),  // drop entries by message
    ),
)
```

---

## 15. Forbidden patterns (AI 必读)

In handler / service / repository / worker code:

```go
// ❌ FORBIDDEN — use logx.FromContext(ctx) or injected logger
log.Printf("...")
fmt.Println("...")
fmt.Printf("...")

// ❌ FORBIDDEN — bypasses redaction / sampling / fields
slog.Info("...")
slog.Default().Info("...")

// ❌ FORBIDDEN — logx is the only logger
zap.L().Info("...")
logrus.Info("...")

// ❌ FORBIDDEN — leaks credentials (redaction only works through logx)
logx.FromContext(ctx).Info("login",
    logx.String("password", userPassword),
)
```

Required:

```go
// ✅ In handlers / services / repos / workers
logger := logx.FromContext(ctx)
logger.Info("skill created",
    logx.String("skill_id", id),
    logx.String("operation", "aihub.skill.create"),
)

// ✅ When initializing a service
svc := NewService(logx.Noop())  // or real logger from main

// ✅ In tests
logger := logx.NewTestLogger(t)
```

---

## 16. Framework slog compatibility (kernel internals only)

Kernel internals (transport/http, transport/grpc, middleware) can use
slog-compatible helpers while migrating:

```go
handler := logx.NewHandler(
    logx.WithWriter(os.Stdout),
    logx.WithFormat(logx.FormatJSON),
    logx.WithFilter(logx.FilterKey("password", "token")),
)
logx.SetDefault(slog.New(handler))
logx.Info("listening", "addr", addr)
```

**Business code MUST NOT use these helpers** — use `logx.Logger` interface +
typed `logx.Field` values instead.

---

## 17. OpenTelemetry extractor (optional)

The optional `logx/otelx` package is behind the `otel` build tag and can
extract `trace_id` / `span_id` from OpenTelemetry contexts without making
core `logx` depend on OTel:

```go
//go:build otel

import "github.com/aisphereio/kernel/logx/otelx"

handler := logx.NewHandler(
    logx.WithFieldExtractor(otelx.ExtractTraceFields),
)
```

---

## 18. 测试

```bash
go test ./logx -v
go test ./logx -race
go test ./logx -cover
go test ./logx -bench=.

go run ./examples/logx-basic
go run ./examples/logx-http
```

---

## 19. 文档地图

logx 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (logx/README.md)                 ← 你正在看的，单一入口
├── logx/doc.go                             ← go doc 输出源
└── logx/example_test.go                    ← Go 标准示例（按 API 类别）

深度规范（架构师/PR review 时看）
├── docs/design/logx.md                     ← 设计规范（1307 行）
└── docs/contracts/logx.md                  ← 不可破坏契约

AI 编码指南（AI 写业务代码时看）
├── docs/ai/logx.md                         ← 合并版 AI 指南
└── AGENTS.md                               ← 项目级 AI 规则

验收与运维（CI/CD 时看）
└── docs/process/logx-acceptance-checklist.md

可运行示例
├── examples/logx-basic/                    ← 最小示例
└── examples/logx-http/                     ← HTTP access log 示例
```

**优先级**：日常开发只看本 README + `docs/ai/logx.md` 即可。

---

## 20. Examples 索引（按场景查找）

**Before writing logx code, look up the matching Example.** All examples are
runnable (`go test ./logx -run=Example -v`) and show exact output.

### Field constructors

| Field type | Example function |
|---|---|
| String / Int / Int64 / Uint64 / Bool / Float64 | `ExampleString`, `ExampleInt`, `ExampleBool` |
| Duration / Time | `ExampleDuration`, `ExampleTime` |
| Any / Group | `ExampleAny`, `ExampleGroup` |
| Event | `ExampleEvent` |
| Err (errorx auto-extract) | `ExampleErr` |

### Logger methods

| Method | Example |
|---|---|
| `New` / `NewSlog` | `ExampleNew` |
| `Noop` | `ExampleNoop` |
| `FromContext` / `FromContextOr` | `ExampleFromContext` |
| `Inject` | `ExampleInject` |
| `With` / `Named` / `WithContext` | `ExampleWith`, `ExampleNamed` |
| `Sync` | `ExampleSync` |
| `Enabled` | `ExampleEnabled` |

### Context fields

| Function | Example |
|---|---|
| `ContextWithFields` / `FieldsFromContext` | `ExampleContextWithFields` |
| `ContextWithAttrs` / `AttrsFromContext` | `ExampleContextWithAttrs` |
| `Inject` / `FromContext` | `ExampleInject`, `ExampleFromContext` |

### Configuration

| Function | Example |
|---|---|
| `DefaultConfig` | `ExampleDefaultConfig` |
| `NewRedactor` / `DefaultRedactKeys` | `ExampleNewRedactor` |

### Pre-built log helpers

| Function | Example | Use for |
|---|---|---|
| `LogAccess` | `ExampleLogAccess` | HTTP access log |
| `LogExternalCall` | `ExampleLogExternalCall` | Upstream API call log |
| `LogError` | `ExampleLogError` | Standardized error log |
| `LogAuditHint` | `ExampleLogAuditHint` | Audit breadcrumb |

### Filtering / sampling

| Function | Example |
|---|---|
| `FilterKey` / `FilterFunc` | `ExampleFilterKey` |
| `DropEvents` / `DropMessages` | `ExampleDropEvents` |
| `WithDropFilter` | `ExampleWithDropFilter` |

### Test logger

| Function | Example |
|---|---|
| `NewTestLogger` | `ExampleNewTestLogger` |
| `TestLogger.AssertLogged` | `ExampleTestLogger_AssertLogged` |

### Level control

| Function | Example |
|---|---|
| `ParseLevel` / `ParseLogLevel` | `ExampleParseLevel` |
| `LevelController.SetLevel` | `ExampleLevelController` |

### HTTP / RPC middleware

| Function | Example |
|---|---|
| `HTTPAccessLog` | `ExampleHTTPAccessLog` |
| `Recovery` | `ExampleRecovery` |
| `ServerLogging` / `ClientLogging` | `ExampleServerLogging` |
| `RPCRecovery` | `ExampleRPCRecovery` |
| `LevelHTTPHandler` | `ExampleLevelHTTPHandler` |

### Business scenarios (in `example_business_test.go`)

| Scenario | Example | Layer |
|---|---|---|
| Repository query log | `ExampleBusiness_repositoryLog` | repository |
| Service operation log | `ExampleBusiness_serviceLog` | service |
| Upstream call log | `ExampleBusiness_upstreamCall` | service |
| Worker log with redaction | `ExampleBusiness_workerLog` | worker |
| HTTP access log | `ExampleBusiness_httpAccess` | handler |
| Error log with errorx | `ExampleBusiness_errorLog` | service |
| Audit hint | `ExampleBusiness_auditHint` | service |
| Test logger assertion | `ExampleBusiness_testLogger` | test |
| Request-scoped fields | `ExampleBusiness_requestScoped` | handler |
| Sampling noisy logs | `ExampleBusiness_sampling` | cross-layer |

### Advanced

| Topic | Example |
|---|---|
| Noop logger | `ExampleNoop` |
| Slog unwrap | `ExampleSlog` |
| Custom handler builder | `ExampleNewHandler` |
| Format JSON vs console | `ExampleFormat` |
| nil context safety | `ExampleFromContext_nil` |

### Runnable examples

```bash
go run ./examples/logx-basic      # minimal: New + Info + Sync
go run ./examples/logx-http       # HTTP server with access log + recovery
```

---

## 21. 设计哲学一句话

> **logx is the only logger. logx.Err auto-extracts errorx fields. logx.HTTPAccessLog is the only HTTP access log. Redaction, sampling, request-scoped fields are always on.**
> If you're about to write `log.Printf`, `fmt.Println`, `slog.Info`, or `zap.L()` in business code — STOP, use `logx.FromContext(ctx)` instead.
