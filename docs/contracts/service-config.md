# Kernel 服务配置契约：HTTP / gRPC

Kernel 服务必须通过 `serverx.Config` 启动，生产配置通过 `serverx.LoadConfigFile` 从 `configx` 加载。

## 推荐配置

```yaml
app:
  name: skill-service
  version: dev

deployment:
  replicas: 1

server:
  http:
    enabled: true
    address: ":18082"
    timeout: "2s"
  grpc:
    enabled: true
    address: ":19082"
    timeout: "2s"

system_routes:
  enabled: true
  healthz: true
  readyz: true
  version: true
```

## 规则

- `server.http.enabled=true` 时，serverx 创建 HTTP Server，注册系统路由和业务 HTTP binding。
- `server.grpc.enabled=true` 时，serverx 创建 gRPC Server，注册业务 gRPC service。
- 至少开启一种 transport；生产服务推荐 HTTP + gRPC 都开启。
- Gateway 作为外部入口通常只开启 HTTP；后续需要内部管理能力时也可以开启 gRPC。
- Gateway 到内部 service 的默认协议是 gRPC，HTTP reverse proxy 只是兼容路径。
- 业务不得自己解析配置创建裸 `http.Server` 或裸 `grpc.Server`，必须通过 `serverx.New`。

## 开发范式

```text
kernel new skill-service
  -> 写 proto
  -> make api
  -> serverx.LoadConfigFile
  -> serverx.New
  -> 注册生成的 HTTP/gRPC/Gateway binding
  -> 写业务 handler
```
