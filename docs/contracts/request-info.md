# RequestInfo 契约

`requestx.Info` 是 Kernel 的请求元信息中心，设计上参考 Kubernetes apiserver 的 RequestInfo。

## 1. 目的

一个请求只应该被归一化一次，然后被所有治理能力复用：

```text
transport context
  + proto access policy
  + ctx request_id / trace_id / tenant / principal
  -> requestx.Info
  -> authn / authz / audit / ratelimit / metrics / log / trace / admission
```

## 2. 关键字段

```go
type Info struct {
    Direction string // server/client
    Protocol  string // http/grpc
    Service   string
    Method    string
    Operation string
    Exposure  accessv1.Exposure
    Action    string
    Resource  string
    TenantID  string
    RequestID string
    TraceID   string
    UserID    string
    CallerService string
    TargetService string
    IsSystem bool
    IsInternal bool
}
```

## 3. 生成规则

生成器应该根据 proto policy metadata 生成 `requestx.Resolver`：

```go
func TodoServiceRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error)
```

在所有生成器都完善之前，`autowire.WithRequestInfoResolver` 可以接受手写或生成的 resolver。业务 handler 禁止重复维护这份映射。

## 4. 运行时规则

`autowire.Server` 和 `autowire.Client` 会在链路前段安装 `middleware/requestinfo`，后续中间件通过：

```go
info, ok := requestx.FromContext(ctx)
```

读取请求信息。

## 5. 禁止模式

业务代码禁止这样写：

```go
tr, _ := transport.FromServerContext(ctx)
if strings.Contains(tr.Operation(), "Delete") { ... }
```

正确做法是把 metadata 加到 proto access policy，然后使用生成的 request metadata。
