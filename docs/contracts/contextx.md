# contextx Contract

模块：`github.com/aisphereio/kernel/contextx`
验收等级目标：L3 可生产试用
版本：v0.1.0-alpha.1

## 1. Contract 目标

contextx 是 Kernel 的请求上下文包。它只负责 request-scoped 值的注入和提取，
不负责日志、错误包装、tracing span 管理、transport-specific 工作。

只要以下 contract 不被破坏，内部实现可以重构。

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

```go
type Principal struct {
    SubjectID  string
    TenantID   string
    Roles      []string
    Scopes     []string
    AuthMethod string
}
```

方法签名不可变：

```go
func (p *Principal) IsAuthenticated() bool      // SubjectID != ""
func (p *Principal) HasRole(role string) bool   // role membership
func (p *Principal) HasScope(scope string) bool // scope membership
```

nil Principal 的方法必须安全：

```go
var p *Principal
p.IsAuthenticated()  // false
p.HasRole("admin")   // false
p.HasScope("x")      // false
```

### 2.4 Tenant 解析优先级

`TenantFromContext(ctx)` 必须按以下优先级返回：

1. 显式 `WithTenant(ctx, ...)` 的值（如果非空）
2. `PrincipalFromContext(ctx).TenantID`（如果 principal 存在）
3. `""`（两者都没有）

### 2.5 SubjectIDFromContext

`SubjectIDFromContext(ctx)` 等价于：

```go
p := PrincipalFromContext(ctx)
if p == nil { return "" }
return p.SubjectID
```

对 nil ctx 返回 ""。

### 2.6 RequestIDAndTraceIDFromContext

返回 `(RequestIDFromContext(ctx), TraceIDFromContext(ctx))`。对 nil ctx 返回
`("", "")`。

### 2.7 InjectRequestContext

```go
func InjectRequestContext(ctx context.Context, opts ...RequestContextOption) context.Context
```

- nil ctx → 等价于 `InjectRequestContext(context.Background(), opts...)`
- nil option → 跳过（不 panic）
- 空 string / nil Principal / nil Logger → 跳过对应字段
- 返回的 ctx 包含所有非空字段的 WithXxx 调用结果

### 2.8 Logger 接口

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

### 2.9 Merge

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
- 修改 `Principal` struct 的字段（删除/重命名/改类型）
- 修改 `Principal` 的方法签名
- 修改 `Logger` 类型别名或 `logx.Logger` 契约
- 修改 `Field` 类型别名或 `logx.Field` 字段
- 修改 `LogLevel` 可选值
- 修改 `TenantFromContext` 的优先级规则
- 修改 `Merge` 的语义（如改为 parent2 优先）
- 修改 nil ctx 的行为（从返回零值改为 panic）

## 4. 验收命令

```bash
go test ./contextx -v
go test ./contextx -race
go test ./contextx -cover
go test ./...
go vet ./...
go run ./examples/contextx-basic
```
