# Gateway / IAM / SkillService 自动装载验证流

本 demo 验证规范驱动开发闭环：

```text
kernel new skill-service
  -> 写 proto
  -> make api
  -> 生成 ServiceModule
  -> serverx.BuildServiceFromFactory 自动装载
  -> AI 只写业务 handler
```

当前 validation 使用 generated-equivalent 函数模拟 make api 产物，重点验证运行时语义：

1. `POST /v1/iam/login` 是 PUBLIC route，Gateway 转到 IAMService Login，返回 `alice-token`。
2. `GET /v1/skills/s1` 无 token，Gateway 根据 route policy 直接 401，SkillService handler 不执行。
3. `GET /v1/skills/s1` 带 `bob-token`，Gateway relay，SkillService authn 通过、authz 拒绝，handler 不执行。
4. `GET /v1/skills/s1` 带 `alice-token`，SkillService authn/authz/audit 全部通过，handler 返回 Skill。

认证授权都不是业务 handler 手写，而是由 proto 生成 resolver + `serverx.BuildServiceFromFactory` / `serverx.BuildService` 装载的 Kernel middleware 自动完成。


## 本轮边界收敛

- `protoc-gen-go-gateway`：只生成 GatewayManifest、HTTP -> gRPC binding、Gateway invoker 注册。
- `protoc-gen-go-kernel`：生成 `<Service>KernelModule()`，聚合 gateway/authz/request info 等生成物。
- `serverx.BuildServiceFromFactory`：在 migration/data/provider 装载完成后，通过 `ServiceDeps` 注入业务 service。
- 真实 gRPC 验证里，Gateway 会把 Authorization 等 headers 转为 gRPC metadata，服务端再还原 Kernel transport context，确保 SkillService 内部执行权威 authn/authz/audit。
