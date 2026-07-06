# Package 状态表

本表是 AI Agent 和业务开发者判断包是否可用的权威入口。

## Runtime API

| Package | Status | Rule |
|---|---:|---|
| `errorx` | stable | 业务错误唯一主线 |
| `logx` | stable | 日志唯一主线 |
| `configx` | stable | 配置唯一主线 |
| `metricsx` | active | metrics 主线 |
| `serverx` | active | 服务装配主线 |
| `securityx` | active | 安全配置与 provider-neutral runtime 构造；不是 middleware 装配层 |
| `bootx` | active | 启动治理校验主线 |
| `contextx` | active | 请求上下文注入/提取主线 |
| `transportx/http` | active | HTTP transport 主线 |
| `transportx/grpc` | active | gRPC transport 主线 |
| `requestx` | active | 请求元信息主线 |
| `accessx` | active | 访问控制 guard 主线 |
| `authn` | active | 认证 provider 主线 |
| `authn/casdoor` | active | Casdoor authn 适配器；业务应通过 `authn`/`securityx` 使用 |
| `authn/oidcx` | active | provider-neutral OIDC/JWKS verifier |
| `authz` | active | 授权 provider 主线 |
| `authz/spicedb` | active | SpiceDB authz 适配器；业务应通过 `authz`/`securityx` 使用 |
| `auditx` | active | 审计主线 |
| `gatewayx` | active | Gateway runtime 主线 |
| `admissionx` | active | 准入主线 |
| `ratelimitx` | active | 限流唯一主线 |
| `clientpolicyx` | active | 下游调用治理主线 |
| `dbx` | active | DB 主线 |
| `cachex` | active | Cache 主线 |
| `objectstorex` | active | Object store 主线 |
| `dtmx` | active | 分布式事务主线 |
| `selectorx` | active | selector 主线 |
| `registry` | active | 服务注册发现主线 |
| `encodingx` | active | encoding 主线 |

## Framework assembly / helper packages

| Package | Rule |
|---|---|
| `middleware/*` | 由 `serverx` / `middleware/autowire` 装配消费；业务不要手写装配链 |
| `grpcx` | gRPC runtime helper；优先通过 `transportx/grpc` / `serverx` 使用 |
| `restx` | REST runtime helper；优先通过 `transportx/http` / `serverx` 使用 |
| `servicecontextx` | 服务上下文和客户端引用管理；仅 boot/框架装配代码使用 |
| `iamx` | IAM 领域错误和辅助；不要把它扩展成独立 IAM runtime |

## Scenario tests only

| Package | Rule |
|---|---|
| `validation/*` | 只用于跨模块场景测试和 CI 验证；不是 runtime API，业务禁止 import |

## Tooling only

| Package | Rule |
|---|---|
| `cmd/kernel` | CLI，业务禁止 import |
| `cmd/protoc-gen-*` | generator，业务禁止 import |
| `cmd/buf-check-*` | contract checker，业务禁止 import |

## Removed / deprecated / not mainline

| Package / Path | Replacement |
|---|---|
| `middleware/ratelimit` | `ratelimitx` |
| `internal/ratelimit` | `ratelimitx` providers |
| `errors` | `errorx` |
| `core` as docs concept | `serverx` and focused runtime packages |
| `httpx` as docs concept | `transportx/http` |
| `contextx` as old docs wording for request metadata | `requestx`; current `contextx` only means context injection helpers |
| `securityx.AuthnConfig` | deprecated alias; use `securityx.AuthnBoundaryConfig` |
| `grpcgatewayx` | upstream grpc-gateway generator + `protoc-gen-go-gateway` + `gatewayx` |

## Rule

新代码如果需要基础设施能力，必须优先检查本表。
