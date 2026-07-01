# 服务间调用契约

`access policy` 是被调用方视角：谁可以调用这个 RPC。

`downstream policy` 是调用方视角：当前服务如何安全地调用下游服务。

业务代码不得自己创建 raw `grpc.ClientConn` 或 `http.Client`，也不得自己写 retry、timeout、service-auth、limiter、breaker。必须使用 Kernel client factory 和 `autowire.Client`。

## 1. 最小配置

```yaml
downstreams:
  iam:
    protocol: grpc
    target: discovery:///iam
    timeout: 800ms
```

## 2. 完整配置

```yaml
downstreams:
  iam:
    protocol: grpc
    target: discovery:///iam
    timeout: 800ms
    service_auth:
      enabled: true
      mode: jwt
    rate_limit:
      enabled: true
      backend: memory
      scope: local_instance
      key: operation
      qps: 200
      burst: 400
    circuit_breaker:
      enabled: true
      name: iam
    retry:
      enabled: true
      max_attempts: 2
      backoff: 25ms
```

## 3. 运行时顺序

`autowire.Client` 负责出站链路顺序：

```text
custom before
  -> requestinfo
  -> metadata propagation
  -> timeout
  -> client rate limit
  -> circuit breaker
  -> retry
  -> custom after
```

Retry 只允许用于临时性错误，例如 unavailable、timeout，或者显式标记 `errorx.WithRetryable(true)` 的错误。

以下错误不能重试：

- 权限拒绝。
- 参数校验失败。
- 资源冲突。
- 业务状态机拒绝。

## 4. 校验规则

`clientpolicyx.DownstreamPolicy.Validate` 必须检查：

- `name`、`target`、`protocol`、`timeout` 必填。
- retry 开启时 `max_attempts >= 2`。
- service auth 开启时必须声明 mode。
- rate limit 必须符合 `docs/contracts/rate-limit.md`。

服务启动时必须调用这些校验。

## 5. Agent 规则

如果下游调用需要新能力，先扩展本契约和 Kernel middleware。不要在业务代码里绕过框架写自定义 client。
