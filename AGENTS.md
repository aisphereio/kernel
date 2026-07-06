# Aisphere Kernel Agent 开发规范

本文件是所有 AI Agent / 人类开发者在 Kernel 仓库内写代码时必须遵守的规则。Kernel 当前允许破坏性重构，但不允许业务代码绕过框架范式自行实现基础设施。

## 1. 总原则

```text
proto contract
  -> buf-check-aisphere
  -> protoc generators
  -> requestx.Info
  -> serverx/autowire
  -> admissionx
  -> business service
  -> errorx negotiated response
```

业务组件只写领域逻辑。ctx、trace、authn、authz、audit、rate limit、retry、breaker、timeout、error 协议转换由 Kernel 统一提供。

如果 Kernel 当前表达不了需求，必须先修 Kernel，再写业务代码。

AI 写业务代码前必须先阅读并应用 `docs/ai/advanced-feature-playbook.md`。只要需求命中其中的触发条件，就必须使用对应 Kernel 高级能力，不允许手写临时基础设施实现。

## 2. Runtime API 边界

业务代码、服务 boot 代码和生成代码可以 import 的主线 runtime 包：

```text
errorx logx configx metricsx serverx securityx bootx contextx
transportx/http transportx/grpc
requestx accessx authn authz auditx
gatewayx admissionx ratelimitx clientpolicyx
dbx cachex objectstorex dtmx
selectorx registry encodingx
```

工具包不是 runtime API，业务代码禁止 import：

```text
cmd/kernel
cmd/protoc-gen-*
cmd/buf-check-*
```

以下路径/概念已经移除、废弃或不作为主线：

```text
validation/
middleware/ratelimit/
internal/ratelimit/
github.com/aisphereio/kernel/errors
core/httpx/contextx as main docs wording
securityx.AuthnConfig alias; use securityx.AuthnBoundaryConfig
grpcgatewayx
```

场景检查和 generated-shape 实验不能放在 Kernel runtime tree 中。

## 3. 目录边界

| 目录 | 职责 | 规则 |
|---|---|---|
| `api/` | proto option、公共契约 | 新 RPC 必须声明 access policy |
| `cmd/protoc-gen-*` | 代码生成器 | 生成 glue code，业务不 import |
| `cmd/buf-check-*` | 契约检查 | 失败即阻断 |
| `requestx/` | 请求元信息中心 | 中间件、审计、限流、鉴权都读 `requestx.Info` |
| `serverx/` | 一键服务装配 | 业务组件必须通过它启动服务 |
| `securityx/` | 安全配置与 provider-neutral runtime | 不是 middleware 装配层，由 `serverx.RuntimeProviders` 消费 |
| `transportx/` | HTTP/gRPC transport | 新代码使用 transportx |
| `accessx/` | 访问控制 guard | 统一组合 authn/authz/audit |
| `ratelimitx/` | 限流 provider 抽象 | 旧 ratelimit 路径已删除 |
| `clientpolicyx/` | 服务间调用策略 | 下游 timeout/retry/breaker 进入这里 |
| `admissionx/` | 准入插件链 | 跨接口默认值和校验放这里 |
| `gatewayx/` | Gateway runtime | route manifest、registry、matcher、dispatcher |
| `registry/` | 服务注册发现 runtime API | 保持现有包名，不再在文档里写成不存在的 `registryx` |
| `layout/` | 项目模板 | 体现推荐使用方式，不是 runtime API |
| `docs/` | 中文规范文档 | 能力变化必须同步文档 |

更多说明参考：`docs/contracts/runtime-api-boundary.md`、`docs/contracts/package-status.md` 和 `docs/ai/advanced-feature-playbook.md`。

## 4. 业务开发硬规则

### 4.1 禁止绕过 RequestInfo

禁止业务代码解析 raw HTTP path、raw gRPC full method、operation string。必须使用 `requestx.FromContext(ctx)`。

### 4.2 新接口必须声明 Access Policy

只要 RPC 暴露为 HTTP/gRPC API，就必须声明 access policy。

```text
PUBLIC         login/register/token
AUTHENTICATED /me/logout/refresh
AUTHORIZED    业务资源 CRUD
INTERNAL       服务间调用
SYSTEM         healthz/readyz/metrics/version/debug
```

### 4.3 系统路由由 serverx 管理

业务 proto 不要自己写 `/healthz`、`/readyz`、`/version`、`/metrics`。这些由 `serverx` 统一挂载。

### 4.4 跨接口规则进入 admissionx

默认 tenant/project/owner、状态机校验、删除前依赖检查、发布前安全检查、复杂业务准入规则必须进入 `admissionx`。

### 4.5 服务间调用必须走 Kernel 治理链路

禁止业务直接使用 `grpc.Dial`、裸 `http.Client`、手写 retry loop、手写 limiter。必须通过 Kernel client factory、`serverx.ClientMiddlewareForDownstream` 或后续统一 `serverx.Clients()`。

### 4.6 限流必须使用 ratelimitx

旧 `middleware/ratelimit` 和 `internal/ratelimit` 已删除。限流配置和实现只能走 `ratelimitx` policy/provider，并由 bootx/serverx/clientpolicyx 做部署校验。

### 4.7 Gateway 路由必须由契约驱动

Gateway 不允许手写业务路由表。外部接口必须通过 proto 的 `google.api.http` 和 `aisphere.access.v1.policy` 生成 Route Manifest。

Gateway 只做边界路由和边界准入。资源级授权必须在业务服务中通过 Kernel `authn/authz/accessx` 执行。

### 4.8 错误必须使用 errorx

业务代码禁止裸返回 `errors.New`、`fmt.Errorf` 或 `panic(err)`。必须使用 `errorx`。

## 5. 提交前检查

至少运行：

```bash
go test ./serverx ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
govulncheck ./...
```

如果涉及 proto 或生成器，还要运行对应 `cmd/protoc-gen-*` 和 `cmd/buf-check-*` 测试。

## 6. 文档要求

- 新能力必须同步 `docs/contracts/*.md`。
- AI 高级能力触发条件变化必须同步 `docs/ai/advanced-feature-playbook.md`。
- 设计变化必须同步 `docs/architecture/*.md` 或 `docs/design/*.md`。
- Agent 规则变化必须同步本文件。
- 文档默认使用中文。
- 过期文档移动到 `docs/archive/legacy/`，不要继续作为新开发依据。

### 6.1 AuthN 全流程文档

AuthN 全流程设计文档位于 `layout/docs/design/authn-full-flow.md`，涵盖：

- Gateway 本地 OIDC/JWKS 验 JWT → 注入可信 header → 转发后端
- `kernel/authn/oidcx` 包：通用 OIDC/JWKS verifier，不依赖 Casdoor SDK
- 后端两种模式：`casdoor_jwt`（再验 JWT）和 `gateway_trusted`（信任 Gateway）
- 缓存策略：JWKS 进程内缓存、token 结果可选 Redis
- 配置参考、验证命令、已修复问题记录

涉及此流程的代码变更应同步更新该文档。

## 7. Generator 互相依赖约束

`cmd/protoc-gen-go-kernel` 生成的 `_kernel.pb.go` 必须自洽：如果它在 `serverx.ServiceModule` 中引用 resolver，那么 resolver 必须由同一个 generator 生成，或引用 Kernel runtime 中稳定存在的 symbol。

禁止让 `protoc-gen-go-kernel` 直接依赖 `protoc-gen-go-authz` 是否生成某个 `XxxRequestInfoResolver` / `XxxAccessResolver`。否则 PUBLIC、AUTHENTICATED、INTERNAL、SYSTEM-only service 会再次出现 undefined symbol。

新增 proto generator 能力时，必须同步：

```bash
go test ./cmd/protoc-gen-go-kernel ./cmd/protoc-gen-go-authz ./serverx ./requestx ./accessx
```
