# 限流契约

Kernel 把限流当成契约，而不是某个业务随手加的中间件。任何限流声明都必须说清楚两个问题：

```text
状态存在哪里？保护范围是什么？
```

## 1. Backend

| Backend | 状态位置 | 适用场景 | 是否支持多副本全局配额 |
|---|---|---|---|
| `memory` | 当前进程 / 当前 Pod | 本地开发、单副本、每个 Pod 自我保护 | 否 |
| `redis` | 共享 Redis | 多 Pod 共享的租户/用户/API 配额 | 是 |
| `external` | 网关 / Service Mesh / 云控制面 | 边缘流量治理、全局统一治理 | 是 |

## 2. Scope

| Scope | 含义 |
|---|---|
| `local_instance` | 每个进程/Pod 都有自己的 bucket |
| `global_cluster` | 所有副本共享一个逻辑配额 |

## 3. 必须失败的配置

下面的配置必须在启动阶段失败：

```yaml
replicas: 3
rate_limit:
  scope: global_cluster
  backend: memory
```

原因：memory 只存在于当前进程。3 个 Pod 会得到 3 个独立 bucket，不可能形成全局一致配额。正确选择是：

```text
backend=redis 或 backend=external
```

## 4. 服务端限流

服务端限流保护当前服务不被打爆：

```yaml
server:
  rate_limit:
    enabled: true
    backend: memory
    scope: local_instance
    key: operation
    qps: 100
    burst: 200
```

Kubernetes 多副本下，如果要做租户/用户全局配额：

```yaml
server:
  rate_limit:
    enabled: true
    backend: redis
    scope: global_cluster
    key: tenant
    qps: 1000
    burst: 2000
```

## 5. 客户端限流

客户端限流保护下游服务，也控制当前服务的出口流量：

```yaml
downstreams:
  iam:
    protocol: grpc
    target: discovery:///iam
    timeout: 800ms
    rate_limit:
      enabled: true
      backend: memory
      scope: local_instance
      key: operation
      qps: 200
      burst: 400
```

含义：当前这个服务实例每秒最多向 IAM 发 200 个请求。多副本时这是每个 Pod 的本地限制，不是全局限制。

## 6. Agent 规则

- 业务代码不得自己创建 limiter。
- 业务代码不得自己判断多副本限流语义。
- 所有限流策略必须进入 access policy、downstream policy 或 serverx 配置。
- `bootx.ValidateGovernance` 必须在服务启动前检查策略。
