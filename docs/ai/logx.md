# logx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的日志处理规范。**只看本文件即可写对所有场景。**

---

## 0. 一句话规则

> 业务代码（handler/service/repository/worker）写日志时，**必须**使用 `github.com/aisphereio/kernel/logx`。
> **禁止**使用 `log.Printf` / `fmt.Println` / `fmt.Printf` / `slog` / `zap` / `logrus`。

---

## 1. 速查：什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| 请求级日志 | `logx.FromContext(ctx).Info(...)` | handler/service/repo 内，自动带 request_id |
| 服务启动日志 | `logger.Info("started", ...)` | main.go 启动后 |
| 错误日志 | `logger.Error("...", logx.Err(err))` | 任何返回 error 的地方 |
| HTTP 访问日志 | `logx.HTTPAccessLog(logger, cfg)` 中间件 | HTTP server 启动时挂载 |
| 上游调用日志 | `logx.LogExternalCall(logger, call)` | 调用 LLM/DB/外部 API 时 |
| 审计面包屑 | `logx.LogAuditHint(logger, hint)` | 敏感操作记录（非合规级） |
| 标准错误日志 | `logx.LogError(logger, msg, errorLog)` | 业务错误，自动带 error_code |
| 测试断言 | `logx.NewTestLogger(t).AssertLogged(...)` | 单元测试 |

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 请求级日志（最常见）

```go
// handler 边界注入请求字段
ctx = logx.Inject(ctx, logger,
    logx.String("request_id", reqID),
    logx.String("trace_id", traceID),
    logx.String("subject_id", userID),
    logx.String("route", routePattern),
)

// 任何下游代码：
logx.FromContext(ctx).Info("request accepted")
logx.FromContext(ctx).Debug("querying",
    logx.String("table", "skills"),
    logx.String("id", id),
)
```

`FromContext(ctx)` 自动返回带 request_id / trace_id / subject_id 的 logger。如果 ctx 没注入 logger，返回 `Noop()`（安全）。

### 2.2 服务启动日志

```go
func main() {
    cfg := logx.DefaultConfig("prod")
    cfg.ServiceName = "aihub"
    cfg.Version = "v1.2.3"
    logger, levelCtl, err := logx.New(cfg)
    if err != nil { panic(err) }
    defer logger.Sync()

    logger.Info("service started",
        logx.String("addr", ":8000"),
        logx.String("env", cfg.Env),
    )

    _ = levelCtl.SetLevel("debug")  // 动态降级到 debug
}
```

### 2.3 错误日志（含 errorx 字段自动提取）

```go
err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithRequestID("req_abc"),
)

// logx.Err 自动提取 error_code / http_status / retryable / grpc_code
logger.Error("create skill failed",
    logx.String("operation", "aihub.skill.create"),
    logx.Err(err),
)
```

输出：
```json
{
  "level": "error",
  "message": "create skill failed",
  "operation": "aihub.skill.create",
  "error": "技能不存在",
  "error_type": "*errorx.Error",
  "error_code": "AIHUB_SKILL_NOT_FOUND",
  "http_status": 404,
  "retryable": false
}
```

### 2.4 标准错误日志（用 LogError helper）

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

### 2.5 HTTP 访问日志（中间件）

```go
mux := http.NewServeMux()
mux.HandleFunc("/api", handler)

handler := logx.HTTPAccessLog(logger, logx.AccessLogConfig{
    Enabled:         true,
    SkipPaths:       []string{"/healthz", "/readyz", "/metrics"},
    SlowThreshold:   500 * time.Millisecond,
    RequestIDHeader: "X-Request-ID",
    RouteHeader:     "X-Route-Pattern",
    LogUserAgent:    true,
})(mux)

http.ListenAndServe(":8000", handler)
```

每个请求产生一条结构化日志：`event=access method=... path=... status=200 latency=42ms latency_ms=42`。

### 2.6 上游 API 调用日志

```go
resp, err := modelClient.Invoke(ctx, req)
logx.LogExternalCall(logger, logx.ExternalCall{
    Provider:   "openai",
    Service:    "chat-completions",
    Operation:  "create",
    Model:      "gpt-4",
    Endpoint:   "https://api.openai.com/v1/chat/completions",
    StatusCode: resp.StatusCode,
    Latency:    time.Since(start),
    Err:        err,
})
```

`StatusCode >= 500` 或 `Err != nil` → ERROR 级；`>= 400` → WARN；否则 INFO。

### 2.7 审计面包屑（非合规级）

```go
logx.LogAuditHint(logger, logx.AuditHint{
    Action:       "aihub.skill.delete",
    ActorID:      principal.SubjectID,
    ResourceType: "skill",
    ResourceID:   skillID,
    Result:       "success",
})
```

**注意**：这只是操作日志面包屑，不是合规审计记录。合规审计走 `auditx`（待建）。

### 2.8 仓库层日志

```go
func (r *SkillRepo) Find(ctx context.Context, id string) (*Skill, error) {
    logx.FromContext(ctx).Debug("querying skill",
        logx.String("skill_id", id),
        logx.String("table", "aihub_skills"),
    )

    var row skillModel
    err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
    if err != nil {
        logx.FromContext(ctx).Error("query failed",
            logx.String("skill_id", id),
            logx.Err(err),
        )
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED", ...)
    }
    return toBiz(&row), nil
}
```

### 2.9 Worker 后台任务日志

```go
func (w *Worker) Process(ctx context.Context, job *Job) error {
    logger := w.logger.With(
        logx.String("worker_id", w.id),
        logx.String("job_id", job.ID),
    )

    logger.Info("job started", logx.String("task", job.Task))
    defer logger.Info("job finished")

    if err := w.execute(ctx, job); err != nil {
        logger.Error("job failed",
            logx.Err(err),
            logx.Bool("retryable", errorx.RetryableOf(err)),
        )
        return err
    }
    return nil
}
```

### 2.10 测试断言

```go
func TestCreateSkill(t *testing.T) {
    logger := logx.NewTestLogger(t)
    svc := NewService(logger)

    err := svc.Create(ctx, &CreateReq{Name: "demo"})

    require.NoError(t, err)
    logger.AssertLogged(t, "skill created",
        logx.String("skill_id", "demo"),
        logx.String("operation", "aihub.skill.create"),
    )
}
```

---

## 3. Field 构造器速查

| 类型 | 构造器 | 示例 |
|---|---|---|
| string | `logx.String(k, v)` | `logx.String("user_id", "u_123")` |
| int | `logx.Int(k, v)` | `logx.Int("status", 404)` |
| int64 | `logx.Int64(k, v)` | `logx.Int64("bytes", 1024)` |
| uint64 | `logx.Uint64(k, v)` | `logx.Uint64("count", 100)` |
| bool | `logx.Bool(k, v)` | `logx.Bool("enabled", true)` |
| float64 | `logx.Float64(k, v)` | `logx.Float64("rate", 0.95)` |
| Duration | `logx.Duration(k, v)` | `logx.Duration("latency", 42*time.Millisecond)` (自动加 `_ms`) |
| Time | `logx.Time(k, v)` | `logx.Time("created_at", t)` |
| any | `logx.Any(k, v)` | 复杂类型 fallback |
| event | `logx.Event(name)` | `logx.Event("skill_created")` |
| error | `logx.Err(err)` | 自动提取 errorx 字段 |
| group | `logx.Group(k, fields...)` | 嵌套字段 |

---

## 4. 禁止模式

### 4.1 业务代码禁止（handler/service/repository/worker）

```go
// ❌ 裸 log
log.Printf("skill %s created", id)
fmt.Println("debug")
fmt.Printf("status=%d\n", status)

// ❌ 直接 slog
slog.Info("started", "addr", addr)
slog.Default().Info("...")

// ❌ 直接 zap / logrus
zap.L().Info("...")
logrus.Info("...")

// ❌ 泄漏凭证（redaction 只对 logx 字段生效）
logger.Info("login",
    logx.String("password", userPassword),
    logx.String("token", accessToken),
)
```

### 4.2 替代写法

```go
// ✅ 请求级
logx.FromContext(ctx).Info("skill created",
    logx.String("skill_id", id),
    logx.String("operation", "aihub.skill.create"),
)

// ✅ 服务启动
logger.Info("service started", logx.String("addr", ":8000"))

// ✅ 错误日志
logger.Error("query failed", logx.Err(err))

// ✅ 凭证安全（不要记 password/token；如必须记，redaction 自动处理）
logger.Info("login", logx.String("subject_id", userID))
```

### 4.3 允许的例外

- kernel 内部模块（transport/http, transport/grpc, middleware）可以用 `logx.NewHandler` + `logx.Info` 等 slog 兼容 helper
- 测试代码可以用 `logx.NewTestLogger`
- 业务代码**不能**用包级 `logx.Info` / `logx.InfoContext` 等（这些是 slog 兼容层，仅供 kernel 内部）

---

## 5. Redaction 规则

logx 默认脱敏以下 key（出现在 field key 中即脱敏为 `***`）：

```text
password / passwd / pwd / token / access_token / refresh_token / id_token
secret / client_secret / authorization / cookie / set_cookie
api_key / private_key / ak / sk
```

**铁律**：即使不小心把凭证放进 field，redaction 也会兜底。但**最佳实践是根本不要记凭证**。

自定义脱敏 key：

```go
cfg.Redact = logx.RedactConfig{
    Enabled: true,
    Keys:    append(logx.DefaultRedactKeys(), "my_custom_secret"),
    Value:   "[REDACTED]",
}
```

---

## 6. Sampling 规则

高 QPS 日志（如每请求 debug）开启 sampling 防止日志洪流：

```go
cfg.Sampling = logx.SamplingConfig{
    Enabled:  true,
    Every:    100,                          // 100 条后保留 1 条
    First:    10,                            // 前 10 条全保留
    Window:   time.Second,                  // 每秒重置计数器
    MinLevel: logx.DebugLevel.String(),     // 只采样 debug/info；warn+ 不采样
}
```

**铁律**：warn 和 error 永远不采样。如果出错一定要看到完整日志。

---

## 7. 消费方模式

### 7.1 errorx 集成

`logx.Err(err)` 通过鸭子类型识别 errorx 错误，**不 import errorx**（避免循环依赖）。识别的方法：

```go
Code() string          // errorx.Code
ErrorCode() string     // 兼容
HTTPStatus() int       // errorx.HTTPStatus
StatusCode() int       // GoFr 风格
GRPCCode() string      // errorx.GRPCCode
Retryable() bool       // errorx.Retryable
```

### 7.2 httpx 集成

```go
// httpx 中间件用 logx.HTTPAccessLog
handler := logx.HTTPAccessLog(logger, cfg)(mux)

// httpx panic 恢复用 logx.Recovery
handler := logx.Recovery(logger)(handler)
```

### 7.3 grpcx 集成

```go
// gRPC server 拦截器
srv := grpc.NewServer(
    grpc.UnaryInterceptor(logx.ServerLogging(logger, extract)),
)

// gRPC client 拦截器
conn, _ := grpc.Dial(addr,
    grpc.WithUnaryInterceptor(logx.ClientLogging(logger, extract)),
)
```

---

## 8. 调试技巧

### 8.1 动态调整日志级别

```go
// 启动时拿到 LevelController
logger, levelCtl, _ := logx.New(cfg)

// 运行时动态调整
_ = levelCtl.SetLevel("debug")  // 临时开 debug
_ = levelCtl.SetLevel("error")  // 只看 error

// 通过 HTTP endpoint 暴露
http.Handle("/admin/log-level", logx.LevelHTTPHandler(levelCtl))
// curl 'http://localhost:8000/admin/log-level?level=debug'
```

### 8.2 添加持久字段

```go
// With 返回新 logger，不修改原 logger
serviceLogger := logger.With(
    logx.String("service", "aihub"),
    logx.String("version", "v1.0"),
)

// Named 添加 module 标签
repoLogger := logger.Named("skill_repo")
```

### 8.3 查看底层 slog logger（仅 adapter 用）

```go
sl, err := logx.Slog(logger)
if err != nil {
    // 不是 slog-backed logger
}
```

---

## 9. 完整 handler 示例

```go
package handler

import (
    "context"
    "net/http"

    "github.com/aisphereio/kernel/errorx"
    "github.com/aisphereio/kernel/logx"
)

type SkillHandler struct {
    logger logx.Logger
    svc    SkillService
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 注入请求字段
    ctx = logx.Inject(ctx, h.logger,
        logx.String("request_id", requestIDFromHeader(r)),
        logx.String("trace_id", traceIDFromContext(ctx)),
        logx.String("subject_id", principalFromContext(ctx).SubjectID),
        logx.String("route", "POST /v1/skills"),
    )

    logx.FromContext(ctx).Info("request started")

    var req CreateSkillRequest
    if err := bindJSON(r, &req); err != nil {
        logx.FromContext(ctx).Warn("bad request", logx.Err(err))
        writeError(w, errorx.BadRequest("AIHUB_SKILL_BODY_INVALID", "invalid body"))
        return
    }

    skill, err := h.svc.Create(ctx, &req)
    if err != nil {
        logx.FromContext(ctx).Error("create failed",
            logx.String("operation", "aihub.skill.create"),
            logx.Err(err),
        )
        writeError(w, err)
        return
    }

    logx.FromContext(ctx).Info("skill created",
        logx.Event("skill_created"),
        logx.String("skill_id", skill.ID),
    )
    writeJSON(w, 201, skill)
}
```

---

## 10. 验收清单（写完 logx 代码后自检）

- [ ] 所有日志用 `logx.FromContext(ctx)` 或注入的 logger，不用 `log.Printf` / `slog` / `zap`
- [ ] handler 边界调用 `logx.Inject(ctx, logger, ...)` 注入 request_id / trace_id / subject_id
- [ ] 错误日志用 `logx.Err(err)` 自动提取 errorx 字段
- [ ] 没有把 password / token / secret 直接记到日志（依赖 redaction 兜底）
- [ ] 测试用 `logx.NewTestLogger(t)` 捕获并断言日志
- [ ] `go test ./...` 通过
- [ ] `./scripts/check-logx-usage.sh` 通过（如已配置）
- [ ] `golangci-lint run` 通过（depguard 禁止 import log/slog/zap）

---

## 11. 相关文档

- `logx/README.md` — 单一入口用户指南
- `logx/doc.go` — `go doc logx` 输出源
- `logx/example_test.go` — 所有 API 的 Go 标准 Example
- `logx/example_business_test.go` — 10 个完整业务场景
- `docs/design/logx.md` — 完整设计规范（1307 行，深度参考）
- `docs/contracts/logx.md` — 不可破坏契约
- `docs/process/logx-acceptance-checklist.md` — CI 验收清单
- `examples/logx-basic/` — 最小可运行示例
- `examples/logx-http/` — HTTP access log 完整示例
