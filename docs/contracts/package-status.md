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
| `transportx/http` | active | HTTP transport 主线 |
| `transportx/grpc` | active | gRPC transport 主线 |
| `requestx` | active | 请求元信息主线 |
| `accessx` | active | 访问控制 guard 主线 |
| `authn` | active | 认证 provider 主线 |
| `authz` | active | 授权 provider 主线 |
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

## Tooling only

| Package | Rule |
|---|---|
| `cmd/kernel` | CLI，业务禁止 import |
| `cmd/protoc-gen-*` | generator，业务禁止 import |
| `cmd/buf-check-*` | contract checker，业务禁止 import |

## Removed / not mainline

| Package / Path | Replacement |
|---|---|
| `validation` | external validation / generated project tests |
| `middleware/ratelimit` | `ratelimitx` |
| `internal/ratelimit` | `ratelimitx` providers |
| `errors` | `errorx` |
| `core` as docs concept | `serverx` and focused runtime packages |
| `httpx` as docs concept | `transportx/http` |
| `contextx` as docs concept | `requestx` |
| `grpcgatewayx` | upstream grpc-gateway generator + `protoc-gen-go-gateway` + `gatewayx` |

## Rule

新代码如果需要基础设施能力，必须优先检查本表。
