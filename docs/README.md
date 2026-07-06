# Aisphere Kernel 文档中心

本目录只保留当前 Kernel 开发范式需要阅读的主线文档。过期、阶段性或历史对比材料移入 `docs/archive/legacy/`，不要作为新业务开发依据。

## 1. 先读这六份

1. [快速开始](getting-started.md)
2. [Kernel 边界与分层](architecture/kernel-boundary.md)
3. [Runtime API 边界](contracts/runtime-api-boundary.md)
4. [Package 状态表](contracts/package-status.md)
5. [清理审计记录](contracts/cleanup-audit.md)
6. [Agent 开发规则](../AGENTS.md)

标准起步命令：

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel new todo-service
cd todo-service
make tools
```

MVP 骨架：

```bash
kernel new todo-service --mvp
```

本地/私有 layout 才需要 `--repo`。

## 2. Runtime API 边界

| 主题 | 文档 |
|---|---|
| Package 状态 | [contracts/package-status.md](contracts/package-status.md) |
| Runtime API 边界 | [contracts/runtime-api-boundary.md](contracts/runtime-api-boundary.md) |
| Proto 能力矩阵 | [contracts/proto-capability-matrix.md](contracts/proto-capability-matrix.md) |
| 清理审计记录 | [contracts/cleanup-audit.md](contracts/cleanup-audit.md) |

## 2b. 功能矩阵

Kernel 所有运行时能力按分类列出。`状态` 列参考 [package-status.md](contracts/package-status.md)：`stable` 表示接口已冻结，`active` 表示正在积极开发。中间件包属于 framework assembly/helper，业务不要直接手写装配链。

### 应用生命周期

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `kernel` | 应用生命周期管理（启动/停止/信号处理） | active | — | — |
| `bootx` | 启动治理校验 | active | [contract](contracts/boot-governance-validation.md) | — |

### 配置

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `configx` | 统一配置加载（文件/环境/远程） | stable | [contract](contracts/configx.md) | [basic](../examples/configx-basic/), [env](../examples/configx-env/), [watch](../examples/configx-watch/) |

### 日志

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `logx` | 结构化日志（封装 slog） | stable | [contract](contracts/logx.md) | [basic](../examples/logx-basic/), [http](../examples/logx-http/) |

### 指标

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `metricsx` | 业务指标（Counter/Histogram/Gauge，Prometheus） | active | [contract](contracts/metricsx.md) | [basic](../examples/metricsx-basic/) |

### 错误处理

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `errorx` | 结构化业务错误（跨协议协商） | stable | [contract](contracts/errorx.md) | [basic](../examples/errorx-basic/), [http](../examples/errorx-http/) |

### 数据库

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `dbx` | 数据库访问（GORM 封装，Postgres/MySQL） | active | [contract](contracts/dbx.md) | [basic](../examples/dbx-basic/), [scenarios](../examples/dbx-scenarios/) |
| `migrationx` | SQL 迁移（goose 兼容） | active | [data-dev](contracts/dbx-data-development.md) | — |
| `dbrepo` | 泛型 CRUD 仓库（租户/所有者/软删除） | active | [data-dev](contracts/dbx-data-development.md) | — |

### 缓存

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `cachex` | 统一缓存 API（Redis 单节点/集群/Sentinel） | active | — | [basic](../examples/cachex-basic/) |

### 对象存储

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `objectstorex` | 对象存储（MinIO/S3 兼容） | active | — | [basic](../examples/objectstorex-basic/) |

### 分布式事务

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `dtmx` | 分布式事务（DTM Saga） | active | — | [saga](../examples/dtmx-saga/) |

### 认证 / 安全运行时

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `securityx` | 安全配置与 provider-neutral runtime 构造；不是 middleware 装配层 | active | [boundary](contracts/runtime-api-boundary.md) | — |
| `authn` | 认证 Provider 接口（Authenticator/Principal/TokenService/LoginService/LogoutService） | active | — | [casdoor](../examples/authn-casdoor/) |
| `authn/casdoor` | Casdoor 认证适配器（实现全部 authn 接口） | active | — | 同上 |
| `authn/oidcx` | provider-neutral OIDC/JWKS verifier | active | — | — |

### 授权

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `authz` | 授权 Provider 接口（Authorizer/RelationshipWriter） | active | — | [spicedb](../examples/authz-spicedb/) |
| `authz/spicedb` | SpiceDB 授权适配器 | active | — | 同上 |

### 审计

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `auditx` | 审计记录（追加写安全日志） | active | — | — |

### 访问控制门面

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `accessx` | authn + authz + audit 编排（Guard） | active | — | [secure-skill-service](../examples/secure-skill-service/) |

### Admission

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `admissionx` | K8s 风格准入链（Mutating/Validating） | active | [contract](contracts/admission.md) | — |

### 限流

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `ratelimitx` | 限流 Provider 接口（memory/redis/external） | active | [contract](contracts/rate-limit.md) | — |

### 下游治理

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `clientpolicyx` | 下游调用策略（超时/重试/熔断/服务鉴权） | active | [contract](contracts/downstream-policy.md) | — |

### 请求元信息

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `requestx` | 请求元信息（参考 K8s apiserver RequestInfo） | active | [contract](contracts/request-info.md), [generated](contracts/generated-request-info.md) | — |

### 上下文

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `contextx` | 请求上下文注入（request_id/trace_id/principal/tenant/logger） | active | [contract](contracts/contextx.md) | [basic](../examples/contextx-basic/) |

### 传输层

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `transportx` | 传输层接口（Server/Header/Transport） | active | — | — |
| `transportx/http` | HTTP 传输（路由/中间件/CORS/编解码） | active | — | — |
| `transportx/grpc` | gRPC 传输（拦截器/中间件/errorx 适配） | active | — | — |

### 中间件（框架装配包）

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `middleware/access` | 访问控制 guard | helper | — | — |
| `middleware/authn` | 认证（凭证提取/令牌验证） | helper | — | — |
| `middleware/autowire` | 服务端/客户端中间件链自动装配 | helper | — | — |
| `middleware/circuitbreaker` | 熔断 | helper | — | — |
| `middleware/ctxinject` | 上下文注入（logger/metrics/DTM） | helper | — | — |
| `middleware/logging` | 请求日志 | helper | — | — |
| `middleware/metadata` | 元数据传播 | helper | — | — |
| `middleware/recovery` | Panic 恢复 | helper | — | — |
| `middleware/requestinfo` | 请求元信息解析 | helper | — | — |
| `middleware/retry` | 客户端重试 | helper | — | — |
| `middleware/selector` | 节点选择 | helper | — | — |
| `middleware/timeout` | 请求超时 | helper | — | — |
| `middleware/validate` | 请求校验 | helper | — | — |

### 服务组装

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `serverx` | 服务装配（启动校验/传输构建/系统路由/生命周期） | active | [contract](contracts/serverx.md) | — |
| `servicecontextx` | 服务上下文（客户端引用管理） | helper | — | — |

### Gateway

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `gatewayx` | Gateway 运行时（路由 Manifest/注册/匹配/分发/调用） | active | [contract](contracts/gateway-routing.md) | [basic](../examples/gatewayx-basic/) |

### 服务注册发现

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `registry` | 服务注册与发现接口 | active | — | — |

### 负载均衡

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `selectorx` | 负载均衡（P2C/Random/WRR/Filter/Node） | active | — | — |

### 编码

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `encodingx` | 编解码注册（JSON/Proto/ProtoJSON/XML/YAML/Form） | active | — | — |

### IAM 领域

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `iamx` | IAM 领域错误和辅助 | helper | [domain](contracts/iam-domain.md) | — |

### REST / gRPC 辅助

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `restx` | HTTP 运行时辅助（过滤器链/CORS/熔断/自适应降级） | helper | — | — |
| `grpcx` | gRPC 运行时辅助（拦截器/上下文传播/errorx 适配） | helper | — | — |

### CLI 工具

| 工具 | 说明 | 文档 |
|---|---|---|
| `cmd/kernel` | 项目脚手架（`kernel new` / `kernel version` / `kernel run` / `kernel upgrade` / `kernel proto` / `kernel change`） | [getting-started](getting-started.md) |
| `cmd/protoc-gen-go-http` | 从 proto `google.api.http` 生成 HTTP 路由注册 | — |
| `cmd/protoc-gen-go-errors` | 从 proto errors 生成 errorx 辅助 | — |
| `cmd/protoc-gen-go-authz` | 从 access policy 生成 request/access resolver | — |
| `cmd/protoc-gen-go-gateway` | 从 proto 生成 Gateway route manifest | — |
| `cmd/protoc-gen-go-deploy` | 从 proto 生成 Gateway API HTTPRoute 部署清单 | — |
| `cmd/protoc-gen-go-kernel` | 生成 serverx ServiceModule / ServiceCatalog glue | — |
| `cmd/buf-check-aisphere` | Kernel proto contract checker | — |
