# contextx

`contextx` is the Aisphere Kernel request-context package. It is the **only**
package business code should use for request-scoped values: `request_id`,
`trace_id`, `principal`, `tenant`, the request-bound logger, metrics manager, and optional ServiceContext. It works with
logx (FromContext) and errorx (WithRequestID / WithTraceID) without creating
import cycles.

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 contextx

Before contextx, request-scoped values were passed via:

- `context.Value` with raw string keys (typo-prone, no type safety)
- Prop-drilling through function arguments (verbose, leaky abstractions)
- Each module inventing its own context helpers (inconsistent)

contextx provides typed, nil-safe helpers for the 5 most common request-scoped
values, plus a generic `With`/`From` escape hatch for custom values.

```text
contextx (only defines request-scoped value injection/extraction)
  ↓ stable contract (typed WithXxx / XxxFromContext helpers)
handler  → inject request_id / trace_id / principal / logger at boundary
service  → PrincipalFromContext(ctx) for authz
repo     → RequestIDFromContext(ctx) for errorx.WithRequestID
logx     → LoggerFromContext(ctx) for request-scoped logging
errorx   → RequestIDAndTraceIDFromContext(ctx) for error enrichment
```

contextx itself **does not** do logging, error wrapping, tracing span management,
or transport-specific work. It only carries values through `context.Context`.

---

## 2. 30-second quickstart

```go
package handler

import (
    "net/http"

    "github.com/aisphereio/kernel/contextx"
    "github.com/aisphereio/kernel/logx"
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Inject everything at the handler boundary (one call)
    ctx = contextx.InjectRequestContext(ctx,
        contextx.WithRequestIDOption(r.Header.Get("X-Request-ID")),
        contextx.WithTraceIDOption(r.Header.Get("X-Trace-ID")),
        contextx.WithPrincipalOption(principalFromAuth(r)),
        contextx.WithLoggerOption(h.logger),
    )

    // Pass ctx downstream — all values are attached
    h.svc.DoSomething(ctx, req)
}

// Anywhere downstream:
func (r *Repo) Find(ctx context.Context, id string) (*X, error) {
    rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
    logger := contextx.LoggerFromContext(ctx)
    if logger != nil {
        logger.Debug("querying", logx.String("skill_id", id))
    }
    // ...
    return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "not found",
        errorx.WithRequestID(rid),
        errorx.WithTraceID(tid),
    )
}
```

---

## 3. Value types cheatsheet

| Value | Inject | Extract |
|---|---|---|
| request_id | `WithRequestID(ctx, "req_abc")` | `RequestIDFromContext(ctx)` → string |
| trace_id | `WithTraceID(ctx, "trace_xyz")` | `TraceIDFromContext(ctx)` → string |
| principal | `WithPrincipal(ctx, &Principal{...})` | `PrincipalFromContext(ctx)` → *Principal |
| tenant | `WithTenant(ctx, "t_acme")` | `TenantFromContext(ctx)` → string |
| logger | `WithLogger(ctx, logger)` | `LoggerFromContext(ctx)` → Logger |
| custom | `With(ctx, "key", value)` | `From(ctx, "key")` → any |
| custom string | `With(ctx, "key", "v")` | `FromString(ctx, "key")` → string |
| both IDs | (use InjectRequestContext) | `RequestIDAndTraceIDFromContext(ctx)` → (string, string) |

---

## 4. Principal struct

```go
type Principal struct {
    SubjectID  string   // "u_123", "svc:agentkit"
    TenantID   string   // "t_acme", empty for single-tenant
    Roles      []string // ["admin", "viewer"]
    Scopes     []string // ["skill:read", "skill:write"]
    AuthMethod string   // "oauth", "apikey", "mtls", "dev_token"
}
```

Methods:

```go
p.IsAuthenticated() bool      // SubjectID != ""
p.HasRole("admin") bool       // role membership check
p.HasScope("skill:read") bool // OAuth scope check
```

Convenience:

```go
contextx.SubjectIDFromContext(ctx) string  // PrincipalFromContext(ctx).SubjectID or ""
```

---

## 5. InjectRequestContext (handler boundary pattern)

The standard way to attach all request-scoped values at the handler boundary:

```go
ctx = contextx.InjectRequestContext(ctx,
    contextx.WithRequestIDOption(reqID),
    contextx.WithTraceIDOption(traceID),
    contextx.WithPrincipalOption(principal),
    contextx.WithTenantOption(tenantID),     // optional
    contextx.WithLoggerOption(logger),
)
```

This is equivalent to calling each `WithXxx` individually, but more readable
and lets you skip nil/empty values cleanly.

---

## 6. nil safety

All extract functions are nil-safe — they return zero values, never panic:

```go
contextx.RequestIDFromContext(nil)        // ""
contextx.PrincipalFromContext(nil)        // nil
contextx.LoggerFromContext(nil)           // nil
contextx.TenantFromContext(nil)           // ""
contextx.From(nil, "key")                 // nil
```

This is critical because business code calls these in hot paths. Pattern:

```go
// Safe — no nil check needed
if p := contextx.PrincipalFromContext(ctx); p != nil && p.IsAuthenticated() {
    // ...
}

// Safe — LoggerFromContextOr falls back
logger := contextx.LoggerFromContextOr(ctx, h.defaultLogger)
logger.Info("...")
```

---

## 7. Tenant resolution priority

`TenantFromContext` resolves in this order:

1. Explicit `WithTenant(ctx, ...)` — wins if set
2. `PrincipalFromContext(ctx).TenantID` — fallback
3. `""` — if neither is set

```go
// Explicit wins
ctx := contextx.WithPrincipal(ctx, &Principal{TenantID: "t_principal"})
ctx = contextx.WithTenant(ctx, "t_explicit")
contextx.TenantFromContext(ctx)  // "t_explicit"

// Falls back to principal
ctx := contextx.WithPrincipal(ctx, &Principal{TenantID: "t_principal"})
contextx.TenantFromContext(ctx)  // "t_principal"

// Neither
contextx.TenantFromContext(context.Background())  // ""
```

---

## 8. Merge two contexts

When a child goroutine needs values from a parent request context but should
also be cancellable independently:

```go
parentCtx := r.Context()  // request context with trace_id
jobCtx := contextx.WithRequestID(context.Background(), "req_job")

merged, cancel := contextx.Merge(parentCtx, jobCtx)
defer cancel()

// merged inherits values from BOTH:
// - TraceIDFromContext(merged)  → from parentCtx
// - RequestIDFromContext(merged) → from jobCtx
// merged.Done() fires when EITHER parent is done OR cancel() is called
```

`Merge(nil, nil)` is safe — treats nil as `context.Background()`.

---

## 9. Generic With/From (escape hatch)

For custom context values that don't fit the typed helpers:

```go
// Feature flags
ctx = contextx.With(ctx, "feature_x_enabled", true)
ctx = contextx.With(ctx, "ab_test_variant", "control")

// Extract
if v, ok := contextx.From(ctx, "feature_x_enabled").(bool); ok && v {
    // feature X path
}
variant := contextx.FromString(ctx, "ab_test_variant")  // "control"
```

**Prefer typed helpers** (`WithRequestID`, `WithPrincipal`, etc.) for type
safety. Use `With`/`From` only for genuinely custom values.

---

## 10. Integration with logx

`contextx.Logger` is an alias of `logx.Logger` — pass a
`logx.Logger` directly:

```go
import (
    "github.com/aisphereio/kernel/contextx"
    "github.com/aisphereio/kernel/logx"
)

logger, _, _ := logx.New(cfg)
ctx = contextx.WithLogger(ctx, logger)  // logx.Logger satisfies contextx.Logger

// Downstream:
logger := contextx.LoggerFromContext(ctx)  // returns contextx.Logger interface
logger.Info("...")  // works
```

contextx reuses the Kernel logger contract instead of duplicating log field
types.

---

## 11. Integration with errorx

```go
import "github.com/aisphereio/kernel/errorx"

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

`RequestIDAndTraceIDFromContext` returns `("", "")` if neither is set — safe
to pass to `errorx.WithRequestID("")` (no-op).

---

## 12. Forbidden patterns (AI 必读)

```go
// ❌ Use string keys directly — typo-prone, no type safety
ctx = context.WithValue(ctx, "request_id", reqID)
v, _ := ctx.Value("request_id").(string)

// ❌ Prop-drilling logger/principal through function args
func (r *Repo) Find(ctx context.Context, logger logx.Logger, principal *contextx.Principal, id string) ...

// ❌ Type-asserting on contextx internals
if p, ok := ctx.Value(contextx.keyPrincipal).(*contextx.Principal); ok { ... }
```

Required:

```go
// ✅ Use typed helpers
ctx = contextx.WithRequestID(ctx, reqID)
rid := contextx.RequestIDFromContext(ctx)

// ✅ Inject once at boundary, extract everywhere
ctx = contextx.InjectRequestContext(ctx, ...)
logger := contextx.LoggerFromContext(ctx)

// ✅ Use the public extract API
p := contextx.PrincipalFromContext(ctx)
```

---

## 13. 测试

```bash
go test ./contextx -v
go test ./contextx -race
go test ./contextx -cover

go run ./examples/contextx-basic
```

---

## 14. 文档地图

contextx 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (contextx/README.md)             ← 你正在看的，单一入口
├── contextx/doc.go                         ← go doc 输出源
└── contextx/example_test.go                ← Go 标准示例

深度规范（架构师/PR review 时看）
├── docs/contracts/contextx.md              ← 不可破坏契约
└── docs/design/contextx.md                 ← 设计规范（如已建）

AI 编码指南（AI 写业务代码时看）
├── docs/ai/contextx.md                     ← 合并版 AI 指南
└── AGENTS.md                               ← 项目级 AI 规则

可运行示例
└── examples/contextx-basic/                ← 最小示例
```

**优先级**：日常开发只看本 README + `docs/ai/contextx.md` 即可。

---

## 15. Examples 索引（按场景查找）

### RequestID / TraceID

| Function | Example |
|---|---|
| `WithRequestID` | `ExampleWithRequestID` |
| `RequestIDFromContext` | `ExampleRequestIDFromContext` |
| `WithTraceID` | `ExampleWithTraceID` |
| `TraceIDFromContext` (nil-safe) | `ExampleTraceIDFromContext_nil` |
| `RequestIDAndTraceIDFromContext` | `ExampleRequestIDAndTraceIDFromContext` |

### Principal

| Function | Example |
|---|---|
| `WithPrincipal` | `ExampleWithPrincipal` |
| `PrincipalFromContext` (nil-safe) | `ExamplePrincipalFromContext` |
| `Principal.IsAuthenticated` | `ExamplePrincipal_IsAuthenticated` |
| `Principal.HasRole` | `ExamplePrincipal_HasRole` |
| `Principal.HasScope` | `ExamplePrincipal_HasScope` |
| `SubjectIDFromContext` | `ExampleSubjectIDFromContext` |

### Tenant

| Function | Example |
|---|---|
| `WithTenant` | `ExampleWithTenant` |
| `TenantFromContext` (fallback to Principal) | `ExampleTenantFromContext_fallbackToPrincipal` |
| `TenantFromContext` (explicit wins) | `ExampleTenantFromContext_explicitWins` |

### Logger

| Function | Example |
|---|---|
| `WithLogger` | `ExampleWithLogger` |
| `LoggerFromContext` (nil-safe) | `ExampleLoggerFromContext` |
| `LoggerFromContextOr` | `ExampleLoggerFromContextOr` |

### Generic With/From

| Function | Example |
|---|---|
| `With` | `ExampleWith` |
| `From` | `ExampleFrom` |
| `FromString` | `ExampleFromString` |

### InjectRequestContext

| Function | Example |
|---|---|
| `InjectRequestContext` | `ExampleInjectRequestContext` |

### Merge

| Function | Example |
|---|---|
| `Merge` | `ExampleMerge` |
| `Merge` (nil parents) | `ExampleMerge_nilParents` |

### Business scenarios (in `example_business_test.go`)

| Scenario | Example | Layer |
|---|---|---|
| Handler boundary injection | `Example_businessHandlerBoundary` | handler |
| Service authz with roles | `Example_businessServiceAuthz` | service |
| Repository attach IDs to errorx | `Example_businessRepositoryErrorx` | repository |
| Tenant isolation in queries | `Example_businessTenantIsolation` | repository |
| Logger propagation | `Example_businessLoggerPropagation` | cross-layer |
| Worker merged context | `Example_businessWorkerMergedContext` | worker |
| Anonymous request handling | `Example_businessAnonymousRequest` | any |
| Custom context values | `Example_businessCustomValues` | any |
| Tenant fallback priority | `Example_businessTenantFallback` | any |
| Scope-based authorization | `Example_businessScopeAuthz` | service |

---

## 16. 设计哲学一句话

> **contextx is the only request-context package. Inject once at the handler boundary, extract everywhere with typed nil-safe helpers. Never use raw context.WithValue with string keys in business code.**
> If you're about to write `context.WithValue(ctx, "request_id", ...)` or prop-drill a logger through 5 function args — STOP, use `contextx.WithRequestID` / `contextx.InjectRequestContext` instead.


---

## ServiceContext integration

Kernel now exposes the go-zero-style dependency handoff point through
`contextx.ServiceContext`.

Startup owns wiring:

```go
svc := contextx.MustNewServiceContext()
contextx.MustPutDependency(svc, skillClient)
contextx.MustPutDependency(svc, authzGuard)
ctx := contextx.WithServiceContext(context.Background(), svc)
```

Generated Gateway/BFF logic can fetch dependencies from context when it does not
own a typed service struct:

```go
skill := contextx.MustGetDependencyFromContext[skillv1.SkillServiceClient](ctx)
```

Use this sparingly. Normal handler/logic code should still prefer explicit
fields on a generated `ServiceContext` struct. The context bridge is for
middleware, generated grpc-gateway mounting, and workers that receive only
`context.Context`.
