# Kernel Package Index

本文件是 Kernel 包边界的全局入口。新业务、生成代码和 AI Agent 先看这里，再进入具体包文档。

## 1. 核心主线包

这些包是业务服务和生成代码的默认依赖面。

| 分类 | 包 | 边界 |
|---|---|---|
| 基础设施 | `errorx`, `logx`, `configx`, `metricsx`, `contextx` | 错误、日志、配置、指标、请求上下文主线 |
| 服务装配 | `serverx`, `bootx`, `securityx` | 服务启动、治理校验、安全 runtime 构造；中间件链仍由 `serverx` / `middleware/autowire` 统一装配 |
| Transport | `transportx/http`, `transportx/grpc`, `requestx` | HTTP/gRPC transport 与请求元信息主线 |
| 安全 | `authn`, `authz`, `accessx`, `auditx` | `authn` 只回答“是谁”，`authz` 只回答“能不能”，`accessx` 编排 authn + authz + audit，`auditx` 只记录审计事实 |
| Gateway | `gatewayx`, `admissionx`, `ratelimitx`, `clientpolicyx` | Gateway runtime、准入、限流、下游治理 |

## 2. 可选能力包

这些包按服务需要启用，不是最小服务必须依赖。

| 分类 | 包 | 说明 |
|---|---|---|
| 数据 | `dbx`, `migrationx`, `dbrepo` | 数据库连接、迁移、泛型仓库 |
| 缓存 | `cachex` | Redis/内存等缓存能力 |
| 对象存储 | `objectstorex` | S3/MinIO 兼容对象存储 |
| 分布式事务 | `dtmx` | DTM Saga/TCC 等分布式事务封装 |
| 注册发现/负载均衡 | `registry`, `selectorx` | 服务注册发现和节点选择 |
| 编解码 | `encodingx` | JSON/Proto/XML/YAML/Form 编解码注册 |
| IAM 领域 | `iamx` | Kernel IAM 控制面领域模型和 Directory facade，不是外部 IdP SDK |
| 资源控制面 | `resourcex` | 资源、角色模板和 Grant 的控制面事实模型 |

## 3. 框架装配 / Helper

| 包 | 使用规则 |
|---|---|
| `middleware/*` | 框架内部装配包；业务不要手写中间件链，优先通过 `serverx` 使用 |
| `grpcx`, `restx` | transport helper；优先经 `transportx/*` / `serverx` 间接消费 |
| `servicecontextx` | 服务上下文与客户端引用管理；仅 boot/框架装配代码使用 |

## 4. Tooling only

| 路径 | 规则 |
|---|---|
| `cmd/kernel` | CLI；业务禁止 import |
| `cmd/protoc-gen-*` | 代码生成器；业务禁止 import |
| `cmd/buf-check-*` | proto contract checker；业务禁止 import |
| `layout/` | 脚手架模板和示例工程结构；Kernel runtime 不反向依赖 layout |

## 5. Scenario validation / examples

| 路径 | 规则 |
|---|---|
| `validation/*` | 只用于跨模块场景验证，不是 runtime API；业务禁止 import |
| `examples/*` | 示例和契约输入；没有明确 README 的示例默认不承诺可独立运行 |

## 6. 废弃 / 不在主线

| 路径或概念 | 当前替代 |
|---|---|
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

`accessx` 是请求时 access guard，负责把已认证 Principal、授权检查、SkipPolicy 和审计串起来。业务 handler 或 server middleware 需要“拦请求”时用 `accessx`，只需要判断权限引擎时用 `authz`。

### `authn.IdentityAdmin` vs `iamx.Directory`

`authn.IdentityAdmin` 是外部身份提供方管理面，例如 Casdoor/OIDC provider 的用户、组织、应用、组同步接口。它只应出现在 IAM provisioning / adapter 层。

`iamx.Directory` 是 Kernel IAM 控制面事实接口，用来管理平台内的用户、组织、组和 membership。业务侧依赖 `iamx.Service` / `iamx.Directory`，不要直接依赖 `authn.IdentityAdmin`。

### `resourcex` vs `authz`

`resourcex.Grant` 是控制面事实，适合审计、审批、撤销、展示和同步。

`authz.Relationship` 是 ReBAC/Zanzibar 查询投影，适合 SpiceDB/OpenFGA 高性能权限判断。写入路径应先落控制面事实，再通过 outbox/projector 投影到 `authz` 后端。

## 8. 依赖关系图

```text
proto contract
  -> cmd/buf-check-* / cmd/protoc-gen-*
  -> requestx + accessx + gatewayx + deploy manifests
  -> serverx / middleware/autowire
  -> authn + authz + auditx providers
  -> business service

iamx/resourcex control-plane facts
  -> transactional outbox/projector
  -> authz relationships
  -> accessx runtime decisions
```

更多状态细节见：

- [docs/README.md](docs/README.md)
- [docs/contracts/package-status.md](docs/contracts/package-status.md)
- [docs/contracts/runtime-api-boundary.md](docs/contracts/runtime-api-boundary.md)
