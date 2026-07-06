# Package 状态表

本表是 AI Agent 和业务开发者判断包是否可用的权威入口。更完整的导航见根目录 [PACKAGE_INDEX.md](../../PACKAGE_INDEX.md)。

## Status 定义

| Status | 含义 |
|---|---|
| `stable` | 核心 API，默认可用于业务代码，破坏性调整必须走迁移文档 |
| `active` | 主线能力，正在演进，允许小幅 API 调整 |
| `optional` | 按需启用能力，不是最小服务必选依赖 |
| `helper` | 框架装配/辅助包，业务一般不直接使用 |
| `tooling` | CLI/generator/checker，禁止业务 import |
| `scenario` | 场景验证或示例输入，不是 runtime API |
| `compat` | 兼容入口，只为旧代码保留，新代码禁止使用 |
| `deprecated` | 已废弃，禁止新代码使用 |
| `not-mainline` | 不在当前主线，必须使用替代方案 |

## Core runtime API

| Package | Status | Rule |
|---|---:|---|
| `errorx` | stable | 业务错误唯一主线 |
| `logx` | stable | 日志唯一主线 |
| `configx` | stable | 配置唯一主线 |
| `contextx` | active | 请求上下文注入/提取主线；不要恢复旧 `core/httpx/contextx` 文档含义 |
| `metricsx` | active | metrics 主线 |
| `serverx` | active | 服务装配主线 |
| `securityx` | active | 安全配置与 provider-neutral runtime 构造；不是 middleware 装配层 |
| `bootx` | active | 启动治理校验主线 |
| `transportx/http` | active | HTTP transport 主线 |
| `transportx/grpc` | active | gRPC transport 主线 |
| `requestx` | active | 请求元信息主线 |
| `accessx` | active | 请求时 access guard；编排 authn + authz + audit + SkipPolicy |
| `authn` | active | 认证 provider 主线；回答“调用者是谁” |
| `authn/casdoor` | active | Casdoor authn 适配器；业务应通过 `authn`/`securityx` 使用 |
| `authn/oidcx` | active | provider-neutral OIDC/JWKS verifier |
| `authz` | active | 授权 provider 主线；回答“能不能访问资源” |
| `authz/spicedb` | active | SpiceDB authz 适配器；业务应通过 `authz`/`securityx` 使用 |
| `auditx` | active | 审计主线 |
| `gatewayx` | active | Gateway runtime 主线 |
| `admissionx` | active | 准入主线 |
| `ratelimitx` | active | 限流唯一主线 |
| `clientpolicyx` | active | 下游调用治理主线 |

## Optional runtime API

| Package | Status | Rule |
|---|---:|---|
| `dbx` | optional | DB 主线 |
| `migrationx` | optional | DB migration 主线 |
| `dbrepo` | optional | 泛型 CRUD repository 辅助 |
| `cachex` | optional | Cache 主线 |
| `objectstorex` | optional | Object store 主线 |
| `dtmx` | optional | 分布式事务主线 |
| `selectorx` | optional | selector 主线 |
| `registry` | optional | 服务注册发现主线 |
| `encodingx` | optional | encoding 主线 |
| `iamx` | optional | Kernel IAM 控制面模型和 Directory facade；不要把 authn provider 投影当 IAM 事实 |
| `resourcex` | optional | 资源、角色模板和 Grant 控制面事实；authz relationships 是查询投影 |

## Framework assembly / helper packages

| Package | Status | Rule |
|---|---:|---|
| `middleware/*` | helper | 由 `serverx` / `middleware/autowire` 装配消费；业务不要手写装配链 |
| `grpcx` | helper | gRPC runtime helper；优先通过 `transportx/grpc` / `serverx` 使用 |
| `restx` | helper | REST runtime helper；优先通过 `transportx/http` / `serverx` 使用 |
| `servicecontextx` | helper | 服务上下文和客户端引用管理；仅 boot/框架装配代码使用 |

## Scenario tests / examples only

| Path | Status | Rule |
|---|---:|---|
| `validation/*` | scenario | 只用于跨模块场景测试和 CI 验证；不是 runtime API，业务禁止 import |
| `examples/*` | scenario | 示例和 proto/generator 输入；没有明确 README 的示例默认不承诺可独立运行 |

## Tooling only

| Package | Status | Rule |
|---|---:|---|
| `cmd/kernel` | tooling | CLI，业务禁止 import |
| `cmd/protoc-gen-*` | tooling | generator，业务禁止 import |
| `cmd/buf-check-*` | tooling | contract checker，业务禁止 import |
| `layout/` | tooling | 脚手架模板和示例工程结构；runtime 包不能依赖 layout |

## Compatibility / removed / not mainline

| Package / Path | Status | Replacement / Rule |
|---|---:|---|
| `aisphere/access/v1/access.proto` | compat | 兼容 wrapper；新 proto 必须 import `api/aisphere/access/v1/access.proto` |
| `securityx.AuthnConfig` | deprecated | deprecated alias; use `securityx.AuthnBoundaryConfig` |
| `grpcgatewayx` | not-mainline | upstream grpc-gateway generator + `protoc-gen-go-gateway` + `gatewayx` |
| `middleware/ratelimit` | removed | `ratelimitx` |
| `internal/ratelimit` | removed | `ratelimitx` providers |
| `errors` | removed | `errorx` |
| `third_party/errors/errors.proto` | removed | orphan proto removed; no Kernel proto imports it |
| `core` as docs concept | removed | `serverx` and focused runtime packages |
| `httpx` as docs concept | removed | `transportx/http` |
| `contextx` as old docs wording for request metadata | removed | `requestx`; current `contextx` only means context injection helpers |

## Boundary decisions

| Confusing pair | Decision |
|---|---|
| `authz` vs `accessx` | `authz` is provider contract; `accessx` is request-time guard/orchestrator |
| generated authz guard vs `accessx.Guard` | generated guard is a generated-call contract; `accessx.Guard` is the runtime access chain |
| `authn.IdentityAdmin` vs `iamx.Directory` | `IdentityAdmin` manages external IdP projections; `Directory` stores Kernel IAM facts |
| `authz.AuditedAuthorizer` vs `accessx.Guard` | decorator only for direct authorizer checks; normal HTTP/gRPC path audits through `accessx.Guard` |
| `resourcex.Grant` vs `authz.Relationship` | Grant is control-plane fact; Relationship is query-optimized authorization projection |

## Rule

新代码如果需要基础设施能力，必须优先检查本表和 [PACKAGE_INDEX.md](../../PACKAGE_INDEX.md)。发现不存在或旧入口时，不允许重新创建同名兼容层；要么改用当前主线包，要么先更新本契约文档再实现新抽象。
