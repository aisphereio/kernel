# contextx Contract

模块：`github.com/aisphereio/kernel/contextx`
验收等级目标：L3 可生产试用
版本：v0.1.0-alpha.2

## 1. Contract 目标

`contextx` 是 Kernel 的请求上下文包。它只负责 request-scoped 值的注入和提取，不负责日志、错误包装、tracing span 管理、transport-specific 工作。

身份读取的主线规则：

```text
业务 authn/authz 代码：优先使用 authn.PrincipalFromContext(ctx)
通用请求上下文代码：可以使用 contextx.PrincipalFromContext(ctx)
```

`contextx.Principal` 必须是 `authn.Principal` 的无损镜像，不能再只保留 SubjectID/TenantID/Roles/Scopes/AuthMethod 这类子集。

## 2. 不可破坏行为

### 2.1 值注入与提取

| 函数 | 注入 | 提取 | nil ctx 行为 |
|---|---|---|---|
| RequestID | `WithRequestID(ctx, "req_abc")` | `RequestIDFromContext(ctx)` → string | 返回 "" |
| TraceID | `WithTraceID(ctx, "trace_xyz")` | `TraceIDFromContext(ctx)` → string | 返回 "" |
| Principal | `WithPrincipal(ctx, &Principal{...})` | `PrincipalFromContext(ctx)` → *Principal | 返回 nil |
| Tenant | `WithTenant(ctx, "t_acme")` | `TenantFromContext(ctx)` → string | 返回 "" |
| Logger | `WithLogger(ctx, logger)` | `LoggerFromContext(ctx)` → Logger | 返回 nil |
| Custom | `With(ctx, "key", value)` | `From(ctx, "key")` → any | 返回 nil |
| Custom string | `With(ctx, "key", "v")` | `FromString(ctx, "key")` → string | 返回 "" |

### 2.2 nil 安全

所有 `XxxFromContext` 函数必须对 nil ctx 安全，返回零值，不 panic：

```go
RequestIDFromContext(nil)        // ""
TraceIDFromContext(nil)          // ""
PrincipalFromContext(nil)        // nil
TenantFromContext(nil)           // ""
LoggerFromContext(nil)           // nil
From(nil, "key")                 // nil
```

### 2.3 Principal 类型

`contextx.Principal` 必须完整镜像 `authn.Principal` 的身份字段：

```go
type Principal struct {
    SubjectID   string
    SubjectType string

    Provider   string
    ExternalID string
    Issuer     string
    Audience   []string

    TenantID  string
    OrgID     string
    AppID     string
    ProjectID string

    Username string
    Name     string
    Email    string
    Phone    string

    Roles  []string
    Groups []string
    Scopes []string

    AuthMethod string

    Attributes AttributeSet
    IssuedAt   time.Time
    ExpiresAt  time.Time
}
```

方法签名不可变：

```go
func (p Principal) Normalize() Principal
func (p *Principal) IsAuthenticated() bool
func (p *Principal) IsAnonymous() bool
func (p *Principal) HasRole(role string) bool
func (p *Principal) HasGroup(group string) bool
func (p *Principal) HasScope(scope string) bool
func (p *Principal) Attribute(key string) any
func (p *Principal) Expired(now time.Time) bool
```

nil Principal 的方法必须安全：

```go
var p *Principal
p.IsAuthenticated()  // false
p.HasRole("admin")   // false
p.HasGroup("dev")    // false
p.HasScope("x")      // false
```

### 2.4 authn/contextx 镜像

`contextx.WithAuthnPrincipal(ctx, p)` 必须把 `authn.Principal` 的所有字段镜像到 `contextx.Principal`，包括：

```text
OrgID / AppID / ProjectID / Username / Name / Email / Phone / ExternalID /
Issuer / Audience / Groups / Attributes / IssuedAt / ExpiresAt
```

框架适配层需要从 `contextx.Principal` 回到 `authn.Principal` 时使用：

```go
authnPrincipal := contextx.ToAuthnPrincipal(contextx.PrincipalFromContext(ctx))
```

### 2.5 Tenant 解析优先级

`TenantFromContext(ctx)` 必须按以下优先级返回：

1. 显式 `WithTenant(ctx, ...)` 的值（如果非空）
2. `PrincipalFromContext(ctx).TenantID`（如果 principal 存在）
3. `PrincipalFromContext(ctx).OrgID`（兼容 Casdoor owner → org/tenant 的 Gateway 投影）
4. `""`（都没有）

### 2.6 SubjectIDFromContext

`SubjectIDFromContext(ctx)` 等价于：

```go
p := PrincipalFromContext(ctx)
if p == nil { return "" }
return p.SubjectID
```

对 nil ctx 返回 ""。

### 2.7 RequestIDAndTraceIDFromContext

返回 `(RequestIDFromContext(ctx), TraceIDFromContext(ctx))`。对 nil ctx 返回 `("", "")`。

### 2.8 InjectRequestContext

```go
func InjectRequestContext(ctx context.Context, opts ...RequestContextOption) context.Context
```

- nil ctx → 等价于 `InjectRequestContext(context.Background(), opts...)`
- nil option → 跳过（不 panic）
- 空 string / nil Principal / nil Logger → 跳过对应字段
- 返回的 ctx 包含所有非空字段的 WithXxx 调用结果

### 2.9 Logger 接口

`Logger` 复用 `logx.Logger` 契约：

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger
    Named(name string) Logger
    WithContext(ctx context.Context) Logger
    Enabled(level LogLevel) bool
    Sync() error
}
```

`Field` struct 字段不可变：`Key string` + `Value any`。

`LogLevel` 类型为 string，可选值：`debug` / `info` / `warn` / `error`。

### 2.10 Merge

```go
func Merge(parent1, parent2 context.Context) (context.Context, context.CancelFunc)
```

- nil parent → 等价于 `context.Background()`
- 返回的 ctx 的 `Done()` 在以下任一条件触发时关闭：
  - parent1.Done() 关闭
  - parent2.Done() 关闭
  - 返回的 CancelFunc 被调用
- `Value(key)` 先查 parent1，再查 parent2
- `Deadline()` 返回两者中较早的 deadline
- CancelFunc 是幂等的（多次调用安全）

## 3. Breaking Change 判定

以下变更必须作为 breaking change 记录：

- 删除或修改任何 `WithXxx` / `XxxFromContext` 函数签名
- 删除或重命名 `Principal` 字段
- 让 `contextx.Principal` 相比 `authn.Principal` 发生身份字段精度损失
- 修改 `Principal` 的方法签名
- 修改 `Logger` 类型别名或 `logx.Logger` 契约
- 修改 `Field` 类型别名或 `logx.Field` 字段
- 修改 `LogLevel` 可选值
- 修改 `TenantFromContext` 的优先级规则
- 修改 `Merge` 的语义（如改为 parent2 优先）
- 修改 nil ctx 的行为（从返回零值改为 panic）

## 4. 业务使用准则

业务 handler 不应该解析 Gateway header，也不应该自己从 `transport.Header` 重建身份。Kernel 中间件会在请求进入 handler 前完成：

```text
Gateway trusted headers / bearer token
  -> authn.Authenticator
  -> authn.ContextWithPrincipal(ctx, p)
  -> contextx.WithAuthnPrincipal(ctx, p)
  -> handler(ctx, req)
```

推荐写法：

```go
p, ok := authn.PrincipalFromContext(ctx)
if !ok {
    return nil, authn.ErrUnauthenticated("")
}

userID := p.SubjectID // 稳定用户 UUID
orgID := p.OrgID      // Casdoor owner / Aisphere org projection
```

兼容写法：

```go
p := contextx.PrincipalFromContext(ctx)
if p == nil || !p.IsAuthenticated() {
    return nil, authn.ErrUnauthenticated("")
}
```

## 5. 验收命令

```bash
go test ./contextx -v
go test ./contextx -race
go test ./contextx -cover
go test ./authn -v
go test ./...
go vet ./...
go run ./examples/contextx-basic
```
