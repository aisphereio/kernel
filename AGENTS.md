# Aisphere Kernel Agent 开发规范

本文件是所有 AI Agent / 人类开发者在 Kernel 仓库内写代码时必须遵守的规则。Kernel 当前允许破坏性重构，但不允许业务代码绕过框架范式自行实现基础设施。

## 1. 总原则

Kernel 的开发范式是：

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

业务组件只写领域逻辑。ctx、trace、authn、authz、audit、ratelimit、retry、breaker、timeout、error 协议转换由 Kernel 统一提供。

如果 Kernel 当前表达不了需求，必须先修 Kernel，再写业务代码。不要绕过框架临时实现一套。

## 2. 目录边界

| 目录 | 职责 | 规则 |
|---|---|---|
| `api/` | proto option、公共契约 | 新 RPC 必须声明 access policy |
| `cmd/protoc-gen-*` | 代码生成器 | 生成 glue code，不写业务逻辑 |
| `cmd/buf-check-*` | 契约检查 | 失败即阻断，不允许业务绕过 |
| `requestx/` | 请求元信息中心 | 中间件、审计、限流、鉴权都读 `requestx.Info` |
| `admissionx/` | 准入插件链 | 跨接口默认值和校验放这里 |
| `middleware/` | server/client 中间件 | 不在业务层重复实现 |
| `serverx/` | 一键服务装配 | 业务组件必须通过它启动服务 |
| `bootx/` | 启动治理校验 | 多副本、provider、限流策略必须校验 |
| `ratelimitx/` | 限流 provider 抽象 | 方案参考 `docs/contracts/rate-limit.md` |
| `clientpolicyx/` | 服务间调用策略 | 方案参考 `docs/contracts/downstream-policy.md` |
| `validation/` | 验证业务代码 | 不属于 Kernel 核心包，核心包禁止依赖它 |
| `layout/` | 项目模板 | 体现推荐使用方式 |
| `docs/` | 中文规范文档 | 能力变化必须同步文档 |

更多说明参考：`docs/architecture/kernel-boundary.md`。

## 3. 业务开发硬规则

### 3.1 禁止绕过 RequestInfo

禁止业务代码解析 raw HTTP path、raw gRPC full method、operation string。

必须使用：

```go
info, ok := requestx.FromContext(ctx)
```

相关文档：

- `docs/contracts/request-info.md`
- `docs/contracts/generated-request-info.md`

### 3.2 新接口必须声明 Access Policy

只要 RPC 暴露为 HTTP/gRPC API，就必须声明 access policy。

常见分类：

```text
PUBLIC         login/register/token
AUTHENTICATED /me/logout/refresh
AUTHORIZED    业务资源 CRUD
INTERNAL       服务间调用
SYSTEM         healthz/readyz/metrics/version/debug
```

相关文档：`docs/contracts/access-policy.md`。

### 3.3 系统路由由 serverx 管理

业务 proto 不要自己写：

```text
/healthz
/readyz
/version
/metrics
```

这些由 `serverx` 统一挂载，并标记为 SYSTEM。

相关文档：`docs/contracts/serverx.md`。

### 3.4 跨接口规则进入 admissionx

以下逻辑不要散落在 handler/service：

- 默认 tenant/project/owner。
- 状态机校验。
- 删除前依赖检查。
- 发布前安全检查。
- 复杂业务准入规则。

必须进入 `admissionx` mutating / validating chain。

相关文档：`docs/contracts/admission.md`。

### 3.5 服务间调用必须走 Kernel 治理链路

禁止业务直接使用：

```go
grpc.Dial(...)
http.Client{}
retry for {}
limiter.Allow(...)
```

必须通过 Kernel client factory、`serverx.ClientMiddlewareForDownstream` 或后续统一 `serverx.Clients()`。

服务间治理必须包含：

```text
metadata propagation
requestx.Info client direction
timeout
client rate limit
circuit breaker
retry
errorx decode
```

相关文档：

- `docs/contracts/downstream-policy.md`
- `docs/contracts/rate-limit.md`

### 3.6 限流配置必须表达 scope/backend

单实例本地保护可以用：

```text
scope=LOCAL_INSTANCE
backend=MEMORY
```

多副本全局限流禁止使用 memory，必须使用：

```text
scope=GLOBAL_CLUSTER
backend=REDIS 或 EXTERNAL
```

启动时由 `bootx.ValidateGovernance` 校验。

相关文档：

- `docs/contracts/rate-limit.md`
- `docs/contracts/boot-governance-validation.md`


### 3.8 Gateway 路由必须由契约驱动

Gateway 不允许手写业务路由表。外部接口必须通过 proto 的 `google.api.http` 和 `aisphere.access.v1.policy` 生成 Route Manifest。

Gateway 只做边界路由和边界准入：

```text
PUBLIC      -> 放行
AUTHORIZED  -> passive authn，有 Authorization 才转发
INTERNAL    -> 外部不暴露
```

资源级授权必须在业务服务中通过 Kernel `authn/authz/accessx` 执行。Gateway 不允许默认实现复杂业务 authz。

服务发现使用 Kubernetes Service；本地验证允许用 `gatewayx.StaticHosts` 替代。路由注册表生产使用 etcd；本地验证允许用 `gatewayx.MemoryRegistry`。

相关文档：

- `docs/contracts/gateway-routing.md`
- `docs/demos/platform-gateway-iam-skill-flow.md`

### 3.7 错误必须使用 errorx

业务代码禁止裸返回：

```go
errors.New(...)
fmt.Errorf(...)
panic(err)
```

必须使用 `errorx`，并保证 HTTP/gRPC 能统一转换。

相关文档：`docs/contracts/errorx.md`。

## 4. 验证业务代码规则

IAM/Gateway 等验证业务代码统一放在 `validation/`，例如：

```text
validation/iamgateway
```

验证代码可以依赖 Kernel 公开 API；Kernel 核心包不能依赖 validation。

运行：

```bash
go test ./validation/iamgateway
```

相关文档：`docs/demos/iam-gateway-demo.md`。

## 5. 构建和超时处理

本地沙箱因为依赖、离线代理或编译时间限制失败时，不要判定为业务失败，也不要绕过框架。

处理方式：

1. 先跑 targeted tests。
2. 把超时记录进 `TODO_STATUS.md`。
3. 委托 GitHub Actions 做完整构建。
4. 必要时由 GitHub Actions 产出离线依赖包再回灌。

相关文档：`docs/ai/github-actions-build-delegation.md`。

## 6. 提交前检查

至少运行：

```bash
go test ./serverx ./validation/iamgateway ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
```

如果涉及 proto 或生成器，还要运行对应 `cmd/protoc-gen-*` 和 `cmd/buf-check-*` 测试。

## 7. 文档要求

- 新能力必须同步 `docs/contracts/*.md`。
- 设计变化必须同步 `docs/architecture/*.md` 或 `docs/design/*.md`。
- Agent 规则变化必须同步本文件。
- 文档默认使用中文。
- 过期文档移动到 `docs/archive/legacy/`，不要继续作为新开发依据。

## IAM / Casdoor 开发硬规则

1. Casdoor 只作为 OAuth/OIDC、外部用户中心和用户组织模型参考源，不是业务包的直接依赖。
2. 业务组件不得直接 import Casdoor SDK 或 Casdoor object 结构。
3. 用户、组织、多级组、membership 必须走 `iamx.Service` / `iamx.Directory`。
4. Casdoor adapter 只能放在 `authn/casdoor` 或 `iamx/authnadapter` 这类 provider adapter 包中。
5. 如果 IAM 领域模型表达不了需求，先扩展 `iamx` 和文档，再实现 adapter；不得在 Gateway/Hub/Runtime/Sandbox 中私自定义用户组织模型。
6. authn、iamx、authz 的边界必须保持：token/session 属于 authn，用户组织组属于 iamx，资源权限属于 authz/accessx。
7. 相关设计参考：`docs/architecture/iam-boundary.md`、`docs/contracts/iam-domain.md`、`docs/reference/casdoor-study.md`。

## IAM 多级 Group 与 CRUD 规则

- 用户、组织、组、membership 的 CRUD 必须走 `iamx.Service` 或 `iamx.Directory`。
- 不允许业务包直接操作 Casdoor object、Casdoor SDK、SQL 表或 provider-specific API。
- 多级组必须使用 `Group.ParentID` 表达，`Group.Path` 由 Kernel 维护，业务不得手写路径。
- 需要判断用户属于哪些组时，默认优先使用 `ListEffectiveMemberships` 或 `ListUserGroups(...IncludeInherited=true)`，避免遗漏 inherited 父组。
- `Create*` 用于明确创建，已存在必须 conflict；`Update*` 用于明确编辑，不存在必须 not found；`Upsert*` 只用于同步、provisioning、幂等初始化。
- 生产 IAM 持久化优先使用 `iamx/db.Store`；开发和测试可以使用 `iamx/memory.Store`。
- Casdoor adapter 只能作为 OAuth/OIDC、外部用户中心和同步源，不得成为业务核心模型。

## Authn/Authz Provider 硬规则

- 业务代码只能依赖 Kernel 的 `authn`、`authz`、`accessx` 接口。
- 默认 authn provider 是 Casdoor，落点是 `authn/casdoor`，不得在业务包直接 import Casdoor SDK 或 Casdoor object。
- 默认生产 authz provider 是 SpiceDB，落点是 `authz/spicedb`，不得在业务包直接 import Authzed/SpiceDB SDK。
- `accessx.Guard` / `middleware/autowire` / `serverx` 是请求热路径的认证、授权、审计入口；业务 handler 不允许自己拼 token 校验、权限检查和审计。
- Casdoor 负责用户、组织、多级组、角色和登录管理；Kernel 不默认自建用户中心。
- `iamx/db` 仅作为 experimental / 本地缓存 / 测试或未来迁移选项，不作为默认生产路径。
- 资源级权限必须通过 `authz.Authorizer` / `authz.Service`，生产优先 `authz/spicedb`。
- 设计参考：`docs/architecture/authn-authz-provider-boundary.md`、`docs/architecture/iam-boundary.md`、`docs/contracts/iam-domain.md`。


## IAM / authn / authz provider 边界

- 业务服务必须通过 Kernel 的 `authn`、`authz`、`accessx` 接口接入认证授权。
- 默认认证实现是 `authn/casdoor`，默认授权实现是 `authz/spicedb`。
- 业务代码不得直接 import Casdoor SDK 或 SpiceDB/Authzed SDK。
- IAM 微服务开发流程参考 `docs/demos/iam-service-kernel-flow.md`。
- 新服务必须从 `kernel new <service>` 开始，再写 proto、执行 `make api`、接入 `serverx.RuntimeProviders`。
- 如果生成器、middleware 或 provider 无法表达需求，先修 Kernel，再写业务绕路实现是禁止的。

## 10. Gateway Route Manifest 规则

- 外部路由必须由 `protoc-gen-go-gateway` 从 proto 生成，不允许业务手写硬编码路由表。
- 服务启动或部署阶段必须通过 `serverx.RegisterGatewayRoutes` 注册 manifest。
- 本地验证可用 `gatewayx.MemoryKVStore + gatewayx.EtcdRegistry` 模拟 etcd；生产接真实 etcd。
- 本地验证可用 `gatewayx.StaticHosts` 模拟 K8s Service DNS；生产使用 K8s Service。
- Gateway 默认只做 passive authn/token relay；资源级 authn/authz 必须在业务服务内部通过 Kernel `authn/authz/accessx` 执行。

## Gateway -> Service gRPC 调用规则

- Gateway 到内部 service 的默认生产协议是 generated gRPC client，不是手写 HTTP reverse proxy。
- `protoc-gen-go-gateway` 必须生成：`<Service>GatewayManifest`、HTTP -> gRPC request binding、`Register<Service>GatewayInvokers`。
- Gateway 只能通过 `gatewayx.InvokerRegistry` 调用 upstream operation，业务不得在 Gateway handler 中手写 switch/if 分发到某个 service 方法。
- 本地验证可使用 `gatewayx.UnaryInvoker` 做 in-process 调用；生产必须使用 `gatewayx.GRPCUnaryInvoker` + generated gRPC client。
- authn/authz 的权威执行点仍然是 backend service 的 Kernel server chain；Gateway 只做 passive authn/token relay/route governance。


## 服务配置与 gRPC 规则

- 新服务必须通过 `serverx.LoadConfigFile` / `serverx.New` 加载配置并创建 transport。
- 服务端 HTTP 和 gRPC 必须由配置 `server.http.enabled`、`server.grpc.enabled` 控制。
- 至少开启一种 transport；业务不得自己 new 裸 `http.Server` 或 `grpc.Server`。
- Gateway 到内部服务的默认协议是 generated gRPC client。
- HTTP reverse proxy 仅作为兼容路径，不是 Kernel 规范驱动主路径。
- `kernel new -> proto -> make api` 后必须注册生成的 HTTP/gRPC/Gateway binding；AI 只写业务 handler 和 provider wiring。

## ServiceModule 自动装载硬规则

- 新服务必须优先使用 `serverx.BuildServiceFromFactory(ctx, cfg, <Service>KernelModule(), factory, ...)`，让 DB/Data/Providers 在 Kernel 装载完成后注入业务。旧的 `BuildService(..., impl, ...)` 仅用于兼容已有代码。
- `<Service>KernelModule()` 必须由 `protoc-gen-go-kernel` 在 `make api` 中生成，承载 GatewayManifest、RequestInfoResolver、AccessResolver、HTTP/gRPC 注册函数；`protoc-gen-go-gateway` 只负责 GatewayManifest/binding/invoker。
- 业务 handler 不得手写 token 校验、权限检查、audit 记录、Gateway path 解析、gRPC invoker 注册。
- Provider 初始化必须通过 `serverx.ProviderFactory` 或平台 boot 包，业务包不得直接 import Casdoor SDK、SpiceDB/Authzed SDK。
- proto 声明的 `aisphere.access.v1.policy` 是唯一认证/授权来源；业务代码不得重复表达权限规则。

## DBX / MigrationX / DBRepo 规则

- SQL migration 是数据库结构的唯一真实来源；不要把 CREATE TABLE / INDEX / TRIGGER / VIEW 写进 proto option。
- 新服务必须把数据库变更放在 `migrations/`，优先使用 goose-compatible SQL：`-- +goose Up` / `-- +goose Down`。
- 生产环境 `database.migration.mode` 默认使用 `validate`；开发/测试可使用 `dev_apply` 自动执行 migration。多副本部署使用 `apply/dev_apply` 必须有外部迁移 job/锁策略，或显式 `allow_concurrent=true`。
- GORM AutoMigrate 只允许作为 dev/test 辅助，不作为生产主路径。
- 业务代码不得直接 `sql.Open` / `gorm.Open`；必须通过 `serverx.BuildService` 打开 Kernel-managed `dbx.DB`。
- 常规资源表读写优先使用 `dbrepo.ResourceRepository[T]` 或后续 generated repo；不得在 handler 中重复手写 tenant 过滤、soft delete、分页、排序白名单。`TenantScoped` / `OwnerScoped` 表缺少 tenant/subject 时必须 fail-closed。
- 复杂 SQL 可以作为 migration 或经过 review 的 dbx RawSQL escape hatch；普通 CRUD 不得绕过 dbrepo。
