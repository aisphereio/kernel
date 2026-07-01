# serverx 一键服务装配契约

`serverx` 是 Kernel 的服务装配层。它的存在是为了让业务组件不用手写 transport、系统路由、生命周期 hook 和治理校验。

## 1. 推荐使用方式

业务服务应该依赖：

```go
app, err := serverx.New(ctx, cfg, serverx.WithServerMiddleware(chain...))
```

除非正在修改 Kernel 本身，否则不要手动组装 raw HTTP/gRPC server。

## 2. serverx 负责什么

- 调用 `bootx.ValidateGovernance` 做启动前治理校验。
- 构建 HTTP/gRPC transport。
- 挂载 `/healthz`、`/readyz`、`/version`、`/metrics`。
- 创建 Kernel app 并接入生命周期。
- 构建 downstream governance helper。

## 3. 系统路由

系统路由属于框架，不属于业务 proto。

| Route | Owner | Exposure |
|---|---|---|
| `/healthz` | serverx | SYSTEM/PUBLIC，取决于部署 |
| `/readyz` | serverx | SYSTEM/INTERNAL，通常内部访问 |
| `/version` | serverx | SYSTEM |
| `/metrics` | metricsx/prometheus 或 serverx placeholder | SYSTEM/INTERNAL |

业务如果需要自定义管理 API，应另建明确的 admin API，而不是覆盖系统路由。

## 4. 治理校验

`serverx.New` 会归一化配置，并在服务监听前调用 `bootx.ValidateGovernance`。

必须失败的例子：

- `global_cluster + memory` 限流。
- AUTHORIZED 接口缺少 authn/authz provider。
- audit required 但 audit provider 缺失。
- downstream policy 缺少 protocol、target、timeout。
- service auth 开启但未声明 mode。

## 5. 下游客户端

`serverx.ClientMiddlewareForDownstream` 根据 `clientpolicyx.DownstreamPolicy` 构建标准 client 链：

```text
requestinfo -> metadata -> timeout -> rate limit -> circuit breaker -> retry
```

业务组件后续应该从 Kernel client factory 获取下游 client，不要手写：

```text
grpc.Dial
http.Client
retry loop
limiter
breaker
service-auth headers
```
