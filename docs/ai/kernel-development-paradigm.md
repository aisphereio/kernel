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
2. 声明 google.api.http
3. 声明 aisphere.access.v1.policy
4. 运行 buf-check-aisphere
5. 运行 protoc generators
6. 使用 serverx 装配服务
7. 业务只实现领域 service/repository
8. 跨接口默认值/校验进入 admissionx
9. 服务间调用声明 downstream policy
10. 运行 generated project tests / layout compile smoke / targeted tests
11. 本地超时或缺依赖则委托 GitHub Actions
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
- 生成项目自己的 tests。

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
  -> gatewayx Route Manifest
  -> serverx 注册路由 manifest
  -> generated gRPC invoker 调内部服务
```

开发 IAM：

```text
声明 IAM proto
  -> INTERNAL/AUTHORIZED access policy
  -> service-auth 校验 caller service
  -> authn/authz/audit provider 适配
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

## 6. 高级特性入口

AI 在写业务前必须先读：

- `docs/ai/advanced-feature-playbook.md`
- `docs/contracts/runtime-api-boundary.md`
- `docs/contracts/package-status.md`
- `AGENTS.md`

如果需求命中高级特性的触发条件，必须使用对应 Kernel 包，不允许手写临时实现。

## 7. 验证优先级

每个框架能力至少要有三层验证：

1. 单包测试：验证 provider/middleware 的局部行为。
2. 生成项目测试：验证真实业务项目如何使用 Kernel。
3. GitHub Actions 长构建：验证 fullflow、layout、生成器和依赖环境。

场景检查和 generated-shape 实验不进入 Kernel runtime tree。需要时放入独立验证仓库、生成项目 tests、显式 build tag 或专用 GitHub Actions job。

## 8. 文档同步规则

每次修改框架范式时必须同步：

- `AGENTS.md`
- `docs/README.md`
- 对应的 `docs/contracts/*.md`
- 必要时更新 `docs/ai/advanced-feature-playbook.md`
