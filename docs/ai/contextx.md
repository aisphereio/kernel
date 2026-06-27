# contextx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的请求上下文处理规范。**只看本文件即可写对所有场景。**

---

## 0. 一句话规则

> 业务代码处理 request-scoped 值（request_id / trace_id / principal / tenant / logger）时，**必须**使用 `github.com/aisphereio/kernel/contextx`。
> **禁止**使用 `context.WithValue(ctx, "string_key", ...)` 字符串 key。

---

## 1. 速查：什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| Handler 边界注入请求字段 | `contextx.InjectRequestContext(ctx, opts...)` | handler 入口 |
| 取 request_id | `contextx.RequestIDFromContext(ctx)` | 任何需要 request_id 的地方 |
| 取 trace_id | `contextx.TraceIDFromContext(ctx)` | 任何需要 trace_id 的地方 |
| 取 principal | `contextx.PrincipalFromContext(ctx)` | 鉴权、日志、审计 |
| 取 subject_id | `contextx.SubjectIDFromContext(ctx)` | 日志、审计（便捷） |
| 取 tenant | `contextx.TenantFromContext(ctx)` | 多租户隔离 |
| 取 logger | `contextx.LoggerFromContext(ctx)` | 请求级日志 |
| 同时取 request_id + trace_id | `contextx.RequestIDAndTraceIDFromContext(ctx)` | 构造 errorx 错误 |
| 合并两个 context | `contextx.Merge(parent1, parent2)` | Worker 子任务 |
| 自定义值 | `contextx.With(ctx, key, value)` | 特性开关、AB 测试 |

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 Handler 边界注入（最常见）

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    ctx = contextx.InjectRequestContext(ctx,
        contextx.WithRequestIDOption(r.Header.Get("X-Request-ID")),
        contextx.WithTraceIDOption(r.Header.Get("X-Trace-ID")),
        contextx.WithPrincipalOption(principalFromAuth(r)),
        contextx.WithLoggerOption(h.logger),
    )
    h.svc.DoSomething(ctx, req)
}
```

### 2.2 取 request_id 构造 errorx 错误

```go
func (r *Repo) Find(ctx context.Context, id string) (*X, error) {
    rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
    // ...
    return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "not found",
        errorx.WithRequestID(rid),
        errorx.WithTraceID(tid),
        errorx.WithMetadata("skill_id", id),
    )
}
```

### 2.3 服务层鉴权

```go
func (s *Service) Delete(ctx context.Context, id string) error {
    p := contextx.PrincipalFromContext(ctx)
    if p == nil || !p.IsAuthenticated() {
        return errorx.Unauthorized("AUTH_REQUIRED", "login required")
    }
    if !p.HasRole("admin") {
        return errorx.Forbidden("AIHUB_ADMIN_REQUIRED", "admin role required",
            errorx.WithMetadata("subject_id", p.SubjectID),
        )
    }
    return s.repo.Delete(ctx, id)
}
```

### 2.4 OAuth scope 检查

```go
func (s *Service) Download(ctx context.Context, name string) error {
    p := contextx.PrincipalFromContext(ctx)
    if !p.HasScope("skill:download") {
        return errorx.Forbidden("AIHUB_SCOPE_DENIED", "missing scope",
            errorx.WithMetadata("required_scope", "skill:download"),
        )
    }
    return s.repo.Download(ctx, name)
}
```

### 2.5 多租户隔离

```go
func (r *Repo) List(ctx context.Context) ([]*Skill, error) {
    tenantID := contextx.TenantFromContext(ctx)
    if tenantID == "" {
        return nil, errorx.BadRequest("TENANT_REQUIRED", "tenant required")
    }
    return r.db.WithContext(ctx).
        Where("tenant_id = ?", tenantID).
        Find(&rows).Error
}
```

### 2.6 请求级日志

```go
func (r *Repo) Find(ctx context.Context, id string) (*X, error) {
    logger := contextx.LoggerFromContext(ctx)
    if logger != nil {
        logger.Debug("querying",
            logx.String("skill_id", id),
            logx.String("table", "skills"),
        )
    }
    // ...
}
```

### 2.7 Worker 合并 context

```go
func (w *Worker) Process(parentCtx context.Context, job *Job) error {
    jobCtx := contextx.WithRequestID(context.Background(), "req_job_"+job.ID)
    merged, cancel := contextx.Merge(parentCtx, jobCtx)
    defer cancel()

    // merged has trace_id from parentCtx AND request_id from jobCtx
    return w.execute(merged, job)
}
```

### 2.8 匿名请求处理（nil 安全）

```go
func (s *Service) PublicList(ctx context.Context) ([]*Skill, error) {
    // Safe — PrincipalFromContext returns nil for anonymous, never panics
    p := contextx.PrincipalFromContext(ctx)
    if p == nil || !p.IsAuthenticated() {
        return s.repo.ListPublic(ctx)
    }
    return s.repo.ListForUser(ctx, p.SubjectID)
}
```

### 2.9 自定义上下文值（特性开关）

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    if featureXEnabled(r) {
        ctx = contextx.With(ctx, "feature_x_enabled", true)
    }
    h.svc.DoSomething(ctx, req)
}

// Downstream:
if v, ok := contextx.From(ctx, "feature_x_enabled").(bool); ok && v {
    // feature X path
}
```

### 2.10 Tenant 回退优先级

```go
// TenantFromContext resolves in order:
// 1. Explicit WithTenant (wins)
// 2. Principal.TenantID (fallback)
// 3. "" (neither set)

ctx := contextx.WithPrincipal(ctx, &Principal{TenantID: "t_principal"})
ctx = contextx.WithTenant(ctx, "t_explicit")
tenant := contextx.TenantFromContext(ctx)  // "t_explicit"
```

---

## 3. Principal 字段

```go
type Principal struct {
    SubjectID  string   // "u_123", "svc:agentkit"
    TenantID   string   // "t_acme"
    Roles      []string // ["admin", "viewer"]
    Scopes     []string // ["skill:read", "skill:write"]
    AuthMethod string   // "oauth", "apikey", "mtls", "dev_token"
}
```

方法：

```go
p.IsAuthenticated() bool      // SubjectID != ""
p.HasRole("admin") bool       // 角色检查
p.HasScope("skill:read") bool // OAuth scope 检查
```

便捷函数：

```go
contextx.SubjectIDFromContext(ctx) string  // Principal.SubjectID or ""
```

---

## 4. 禁止模式

### 4.1 业务代码禁止

```go
// ❌ 字符串 key — 拼写错误、无类型安全
ctx = context.WithValue(ctx, "request_id", reqID)
v, _ := ctx.Value("request_id").(string)

// ❌ 通过函数参数传递 logger/principal
func (r *Repo) Find(ctx context.Context, logger logx.Logger, p *contextx.Principal, id string) ...

// ❌ 类型断言 contextx 内部
if p, ok := ctx.Value(contextx.keyPrincipal).(*contextx.Principal); ok { ... }
```

### 4.2 替代写法

```go
// ✅ 用 typed helper
ctx = contextx.WithRequestID(ctx, reqID)
rid := contextx.RequestIDFromContext(ctx)

// ✅ 边界注入一次，到处提取
ctx = contextx.InjectRequestContext(ctx, ...)
logger := contextx.LoggerFromContext(ctx)

// ✅ 用公开提取 API
p := contextx.PrincipalFromContext(ctx)
```

---

## 5. nil 安全

所有 `XxxFromContext` 函数都是 nil 安全的：

```go
contextx.RequestIDFromContext(nil)        // ""
contextx.PrincipalFromContext(nil)        // nil
contextx.LoggerFromContext(nil)           // nil
contextx.TenantFromContext(nil)           // ""
contextx.From(nil, "key")                 // nil
```

业务代码无需 nil 检查：

```go
// Safe — no nil check needed
if p := contextx.PrincipalFromContext(ctx); p != nil && p.IsAuthenticated() {
    // ...
}

// Safe — LoggerFromContextOr falls back
logger := contextx.LoggerFromContextOr(ctx, h.defaultLogger)
```

---

## 6. 与 logx 集成

`contextx.Logger` 复用 `logx.Logger` — 直接传 `logx.Logger`：

```go
logger, _, _ := logx.New(cfg)
ctx = contextx.WithLogger(ctx, logger)  // logx.Logger satisfies contextx.Logger

// Downstream:
logger := contextx.LoggerFromContext(ctx)
logger.Info("...")
```

contextx 复用 Kernel 的 `logx.Logger` / `logx.Field` 契约，避免重复声明导致接口漂移。

---

## 7. 与 errorx 集成

```go
rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "not found",
    errorx.WithRequestID(rid),
    errorx.WithTraceID(tid),
)
```

`RequestIDAndTraceIDFromContext` 在两者都未设置时返回 `("", "")` — 安全传给
`errorx.WithRequestID("")`（no-op）。

---

## 8. 完整 handler 示例

```go
package handler

import (
    "context"
    "net/http"

    "github.com/aisphereio/kernel/contextx"
    "github.com/aisphereio/kernel/errorx"
    "github.com/aisphereio/kernel/logx"
)

type SkillHandler struct {
    logger logx.Logger
    svc    SkillService
}

func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 1. Inject request context at boundary
    ctx = contextx.InjectRequestContext(ctx,
        contextx.WithRequestIDOption(r.Header.Get("X-Request-ID")),
        contextx.WithTraceIDOption(r.Header.Get("X-Trace-ID")),
        contextx.WithPrincipalOption(principalFromAuth(r)),
        contextx.WithLoggerOption(h.logger),
    )

    // 2. Authz check
    p := contextx.PrincipalFromContext(ctx)
    if p == nil || !p.HasRole("admin") {
        writeError(w, errorx.Forbidden("AIHUB_ADMIN_REQUIRED", "admin required"))
        return
    }

    // 3. Business logic (ctx carries everything)
    name := r.URL.Query().Get("name")
    if err := h.svc.Delete(ctx, name); err != nil {
        writeError(w, err)  // svc already returns errorx with request_id
        return
    }

    writeJSON(w, 204, nil)
}
```

---

## 9. 验收清单

- [ ] 所有 request-scoped 值用 contextx typed helper，不用字符串 key
- [ ] handler 边界调用 `InjectRequestContext` 注入 request_id / trace_id / principal / logger
- [ ] errorx 错误带 `WithRequestID(rid)` / `WithTraceID(tid)`
- [ ] 多租户查询用 `TenantFromContext` 而非参数传递
- [ ] 日志用 `LoggerFromContext` 而非参数传递
- [ ] `go test ./...` 通过
- [ ] `golangci-lint run` 通过

---

## 10. 相关文档

- `contextx/README.md` — 单一入口用户指南
- `contextx/doc.go` — `go doc contextx` 输出源
- `contextx/example_test.go` — 所有 API 的 Go 标准 Example
- `contextx/example_business_test.go` — 10 个完整业务场景
- `examples/contextx-basic/` — 最小可运行示例
