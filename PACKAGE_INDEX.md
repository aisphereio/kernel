# Kernel Package Index

本文件是 Kernel 包边界的全局入口。新业务、生成代码和 AI Agent 先看这里，再进入具体包文档。

## 1. 核心主线包

| 分类 | 包 | 边界 |
|---|---|---|
| 基础设施 | `errorx`, `logx`, `configx`, `metricsx`, `contextx` | 错误、日志、配置、指标、请求上下文主线；`contextx.Principal` 是 `authn.Principal` 的无损镜像 |
| 服务装配 | `serverx`, `bootx`, `securityx` | 服务启动、治理校验、安全 runtime 构造；中间件链仍由 `serverx` / `middleware/autowire` 统一装配 |
| Transport | `transportx/http`, `transportx/grpc`, `requestx` | HTTP/gRPC transport 与请求元信息主线 |
| 安全 | `authn`, `authz`, `accessx`, `auditx` | `authn` 只回答“是谁”，`authz` 只回答“能不能”，`accessx` 编排 authn + authz + audit，`auditx` 只记录审计事实 |
| Gateway | `gatewayx`, `admissionx`, `ratelimitx`, `clientpolicyx` | Gateway runtime、准入、限流、下游治理 |

## 2. 可选能力包

| 分类 | 包 | 说明 |
|---|---|---|
| 数据 | `dbx`, `migrationx`, `dbrepo` | 数据库连接、迁移、泛型仓库 |
| 缓存 | `cachex` | Redis/内存等缓存能力 |
| 对象存储 | `objectstorex` | S3/MinIO 兼容对象存储 |
| 分布式事务 | `dtmx` | DTM Saga/TCC 等分布式事务封装 |
| 后台任务 | `taskx` | provider-neutral durable task runtime；业务只依赖 Job/Handler 契约 |
| 后台任务 Provider | `taskx/dapr` | 生产默认：Dapr Jobs gRPC、Scheduler 持久化与多副本分发；挂载现有 Kernel gRPC Server |
| 本地任务 Provider | `taskx.Scheduler`, `RedisLocker` | 本地、测试、单机和应急降级；不作为生产任务中心主线 |
| 注册发现/负载均衡 | `registry`, `selectorx` | 服务注册发现和节点选择 |
| 编解码 | `encodingx` | JSON/Proto/XML/YAML/Form 编解码注册 |
| IAM 领域 | `iamx` | Kernel IAM 控制面领域模型和 Directory facade，不是外部 IdP SDK |
| 资源控制面 | `resourcex` | 资源、角色模板和 Grant 的控制面事实模型 |
| Kubernetes SDK | `kubernetesx` | controller-runtime / client-go 封装：CRUD、Server-Side Apply、Discovery、Probe、错误归一化；业务只依赖 `kubernetesx.Client`，不直接 import client-go |
| Kubernetes 测试 | `kubernetesx/fake` | 纯 Go fake client，单元测试用；SSA 字段冲突与 CRD 真实语义用 envtest/Kind（后续 PR） |

## 3. 框架装配 / Helper

| 包 | 使用规则 |
|---|---|
| `middleware/*` | 框架内部装配包；业务不要手写中间件链，优先通过 `serverx` 使用 |
| `grpcx`, `restx` | transport helper；优先经 `transportx/*` / `serverx` 间接消费 |
| `servicecontextx` | 服务上下文与客户端引用管理；仅 boot/框架装配代码使用 |

## 4. Tooling only

| 路径 | 规则 |
|---|---|
| `cmd/kernel` | CLI；业务禁止 import；默认从 `aisphereio/kernel-layout` 拉取服务模板 |
| `cmd/protoc-gen-*` | 代码生成器；业务禁止 import |
| `cmd/buf-check-*` | proto contract checker；业务禁止 import |

## 5. External layout / runtime / examples

| 路径 | 规则 |
|---|---|
| `aisphereio/kernel-layout` | 生成服务模板、生成服务 Makefile、deploy 模板、layout 文档和 smoke tests 的归属仓库 |
| Dapr Runtime / Scheduler | `taskx/dapr` 的生产外部运行时；平台层统一安装、HA、etcd、mTLS、备份与观测 |
| `examples/*` | 示例和契约输入；没有明确 README 的示例默认不承诺可独立运行 |

## 6. 废弃 / 不在主线

| 路径或概念 | 当前替代 |
|---|---|
| `layout/` | moved to `aisphereio/kernel-layout` |
| `validation/` | removed from Kernel runtime tree; use independent validation, CI temp generation, or generated service tests |
| `buf.gen.deploy.yaml` at Kernel root | generated-service deploy template belongs to `kernel-layout` |
| `middleware/ratelimit` | `ratelimitx` |
| `internal/ratelimit` | `ratelimitx` providers |
| `github.com/aisphereio/kernel/errors` | `errorx` |
| 旧文档中的 `core/httpx/contextx` 表述 | `serverx` / `transportx` / `requestx` / 当前 `contextx` |
| `securityx.AuthnConfig` | `securityx.AuthnBoundaryConfig` |
| `grpcgatewayx` | upstream grpc-gateway generator + `protoc-gen-go-gateway` + `gatewayx` |
| `aisphere/access/v1/access.proto` 短路径 wrapper | 兼容导入路径；新 proto 必须 import `api/aisphere/access/v1/access.proto` |

## 7. 关键边界判定

### `authz` vs `accessx`

`authz` 是授权 provider 合同，核心接口是 `Authorizer.Check` 和关系写入/查询接口。它不负责认证，也不负责请求生命周期编排。

`accessx` 是请求时 access guard，负责把已认证 Principal、授权检查、SkipPolicy 和审计串起来。业务 handler 或 server middleware 需要拦请求时用 `accessx`，只需要判断权限引擎时用 `authz`。

### `authn.IdentityAdmin` vs `iamx.Directory`

`authn.IdentityAdmin` 是外部身份提供方管理面，例如 Casdoor/OIDC provider 的用户、组织、应用、组同步接口。它只应出现在 IAM provisioning / adapter 层。

`iamx.Directory` 是 Kernel IAM 控制面事实接口，用来管理平台内的用户、组织、组和 membership。业务侧依赖 `iamx.Service` / `iamx.Directory`，不要直接依赖 `authn.IdentityAdmin`。

### `resourcex` vs `authz`

`resourcex.Grant` 是控制面事实，适合审计、审批、撤销、展示和同步。

`authz.Relationship` 是 ReBAC/Zanzibar 查询投影，适合 SpiceDB/OpenFGA 高性能权限判断。写入路径应先落控制面事实，再通过 outbox/projector 投影到 `authz` 后端。

### `taskx.Runtime` vs local `Scheduler`

`taskx.Runtime` 是生产 durable task-center contract。业务通过 `Schedule/Get/Delete/RegisterHandler` 声明任务，不感知 Dapr 类型。

`taskx/dapr` 是默认生产 provider：Dapr Scheduler 持久化 Job 并把到期事件经 gRPC callback 分发给一个可用应用副本。

现有 `taskx.Scheduler`、`Every`、`At`、`RedisLocker` 是 local provider，只用于开发、测试、单机和应急降级。禁止 Dapr 与 local scheduler 对同一个 Job 同时启用。

### Dapr Jobs vs Workflow / queue

Dapr Jobs 回答“什么时候触发一次 handler”；跨服务多步骤、等待、补偿和长时间状态机使用 Dapr Workflow。

Jobs 不等同于离散 payload 队列。大量邮件、转码、文件处理等队列 workload 使用 Asynq、River 或独立 worker provider；业务仍应依赖 Kernel 抽象而不是第三方 API。

### `kubernetesx.Client` vs client-go / controller-runtime

`kubernetesx` 是 Kernel 唯一的 Kubernetes 客户端抽象。Hub 业务层只依赖 `kubernetesx.Client`（CRUD + Server-Side Apply + Discovery + Probe + 错误归一化），不得直接 import `k8s.io/client-go`、`k8s.io/apimachinery` 或 `sigs.k8s.io/controller-runtime`。

`Client.RESTConfig()` / `Dynamic()` / `Discovery()` 是基础设施层 escape hatch：仅 Hub Data Adapter 可使用，Hub Biz 层不得依赖这些类型。

第一阶段采用请求驱动模型：`Hub Request -> kubernetesx.Client -> API Server`，不为每个远程集群启动 Manager / Informer / Cache / 长期 Watch。周期 Probe 和状态同步通过 `taskx` 完成；后续真正需要 WarmPool、Sandbox Controller 或长期 Watch 时再拆独立 Controller 进程。

主线文档：

- [docs/contracts/kubernetesx.md](docs/contracts/kubernetesx.md)

### Envoy Gateway + Casdoor OIDC Authn

Authn 入口采用 Envoy Gateway + Casdoor OIDC/JWT。Envoy 负责验签、claimToHeaders 和 header sanitize；Kernel middleware 负责从可信 header 重建 `authn.Principal`，同时无损镜像到 `contextx.Principal`。资源级授权仍由服务端 `accessx` / IAM client / SpiceDB 完成。

业务读取身份的推荐入口：

```go
p, ok := authn.PrincipalFromContext(ctx)
```

`contextx.PrincipalFromContext(ctx)` 只作为通用请求上下文和旧代码兼容入口，不再允许精度损失。

主线文档：

- [docs/security/envoy-casdoor-oidc-gateway.md](docs/security/envoy-casdoor-oidc-gateway.md)
- [docs/contracts/contextx.md](docs/contracts/contextx.md)
- [docs/contracts/authn-gateway-generation.md](docs/contracts/authn-gateway-generation.md)
- [docs/contracts/taskx.md](docs/contracts/taskx.md)

## 8. 依赖关系图

```text
proto contract
  -> cmd/buf-check-* / cmd/protoc-gen-*
  -> requestx + accessx + gatewayx
  -> serverx / middleware/autowire
  -> authn + authz + auditx providers
  -> business service

taskx.Runtime
  -> taskx/dapr
  -> Dapr sidecar gRPC
  -> Dapr Scheduler + etcd
  -> AppCallback on existing Kernel gRPC Server
  -> business idempotent reconciliation handler

taskx local Scheduler
  -> optional Redis lease
  -> local/test/fallback handler

kubernetesx.Client
  -> controller-runtime client + client-go discovery/dynamic
  -> Kubernetes API Server (request-driven, no Manager/Cache in phase 1)
  -> Hub Data Adapter only may use RESTConfig/Dynamic/Discovery escape hatches

kernel-layout
  -> generated service Makefile / deploy template / service skeleton
  -> imports Kernel runtime and generator outputs
```

更多状态细节见：

- [docs/README.md](docs/README.md)
- [docs/contracts/package-status.md](docs/contracts/package-status.md)
- [docs/contracts/runtime-api-boundary.md](docs/contracts/runtime-api-boundary.md)
