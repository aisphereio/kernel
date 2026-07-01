# Kernel 规范驱动开发范式

Kernel 的目标不是让每个业务组件自由发挥，而是把工程规范固化为 contract、codegen、middleware、serverx 和测试。

## 1. 一句话原则

```text
业务声明契约，Kernel 负责检查、生成、装配、治理和验证。
```

如果业务开发遇到 Kernel 表达不了的需求，正确动作是：

```text
先修 Kernel，再写业务。
```

不是绕过 Kernel 手写一套 auth、限流、重试、错误协议或系统路由。

## 2. 标准开发流程

```text
1. 写 proto contract
2. 声明 access policy
3. 运行 buf-check-aisphere
4. 运行 protoc generators
5. 使用 serverx 装配服务
6. 业务只实现领域 service
7. 跨接口默认值/校验进入 admissionx
8. 服务间调用声明 downstream policy
9. 运行 validation demo 和 targeted tests
10. 本地超时则委托 GitHub Actions
```

## 3. 与 Kubernetes apiserver 的对应关系

| Kubernetes apiserver | Kernel |
|---|---|
| API Object / Scheme / Codec | proto contract / generated descriptors |
| RequestInfo | `requestx.Info` |
| GenericAPIServer | `serverx` |
| Filters | `middleware/autowire` |
| Authn/Authz | `authn` / `authz` / `accessx` |
| Admission | `admissionx` |
| FlowControl | `ratelimitx` / `clientpolicyx` |
| Status/Error negotiation | `errorx` |
| healthz/readyz/metrics | `serverx` system routes |

## 4. 业务组件应该长什么样

业务组件只应该关注：

- proto contract。
- service 实现。
- repository 实现。
- admission 插件。
- validation 测试。

业务组件不应该关注：

- raw HTTP/gRPC transport。
- authn/authz/audit wiring。
- trace/request id/tenant 透传。
- retry/breaker/limiter 实现。
- error HTTP/gRPC 转换。
- healthz/readyz/metrics/version。

## 5. Gateway/IAM/Hub 的开发方式

开发 Gateway：

```text
声明路由 contract
  -> access policy: PUBLIC/AUTHORIZED/INTERNAL
  -> serverx 装配入口
  -> admissionx 做路由准入
  -> downstream policy 调 IAM/Hub/Runtime
```

开发 IAM：

```text
声明 IAM proto
  -> INTERNAL/AUTHORIZED access policy
  -> service-auth 校验 caller service
  -> authz/audit provider 适配
  -> errorx 统一返回
```

开发 Hub：

```text
声明 Hub 资源 contract
  -> access policy 生成 resolver
  -> requestx.Info 统一资源上下文
  -> admissionx 做发布、删除、分享等准入
  -> downstream policy 调 IAM/Runtime/Sandbox
```

## 6. 验证优先级

每个框架能力至少要有三层验证：

1. 单包测试：验证 provider/middleware 的局部行为。
2. validation 业务测试：验证 Gateway/IAM/Hub 这类真实业务形态。
3. GitHub Actions 长构建：验证 fullflow、layout、生成器和离线依赖。

## 7. 文档同步规则

每次修改框架范式时必须同步：

- `AGENTS.md`
- `docs/README.md`
- 对应的 `docs/contracts/*.md`
- 必要时更新 `validation/` demo 文档
