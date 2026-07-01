# 启动治理校验契约

Kernel 服务必须在监听端口前校验运行拓扑和治理策略。如果配置错误，应该 fail fast，而不是启动后在流量中暴露问题。

## 1. 当前校验项

`bootx.ValidateGovernance` 检查：

- server rate limit 与部署副本数是否冲突。
- downstream rate limit 与部署副本数是否冲突。
- 必需的 authn/authz/audit provider 是否配置。
- downstream timeout、retry、service-auth、rate-limit 语义是否完整。

## 2. 关键规则

`scope=global_cluster` 不能使用 `backend=memory`。

正确示例：

```yaml
rate_limit:
  scope: global_cluster
  backend: redis
```

`memory` 只允许用于 `local_instance` 自我保护和本地开发。

## 3. Agent 规则

Agent 不得手动创建一堆 provider 后直接启动服务。新的 boot/server 入口必须调用 `bootx.ValidateGovernance`，或者调用更高层的 Kernel boot API。
