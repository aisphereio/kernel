# Aisphere Kernel 文档中心

本目录只保留当前 Kernel 开发范式需要阅读的主线文档。过期、阶段性或历史对比材料移入 `docs/archive/legacy/`，不要作为新业务开发依据。

## 1. 先读这五份

1. [快速开始](getting-started.md)
2. [Kernel 边界与分层](architecture/kernel-boundary.md)
3. [规范驱动开发范式](ai/kernel-development-paradigm.md)
4. [AI 高级特性使用手册](ai/advanced-feature-playbook.md)
5. [Agent 开发规则](../AGENTS.md)

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

## 2b. 功能矩阵

Kernel 所有运行时能力按分类列出。`状态` 列参考 [package-status.md](contracts/package-status.md)：`stable` 表示接口已冻结，`active` 表示正在积极开发。

### 应用生命周期

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `kernel` | 应用生命周期管理（启动/停止/信号处理） | active | — | — |

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

### 认证

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `authn` | 认证 Provider 接口（Authenticator/Principal/TokenService/LoginService/LogoutService） | active | — | [casdoor](../examples/authn-casdoor/) |
| `authn/casdoor` | Casdoor 认证适配器（实现全部 authn 接口） | active | — | 同上 |

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

### 中间件

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `middleware/access` | 访问控制 guard | active | — | — |
| `middleware/authn` | 认证（凭证提取/令牌验证） | active | — | — |
| `middleware/autowire` | 服务端/客户端中间件链自动装配 | active | — | — |
| `middleware/circuitbreaker` | 熔断 | active | — | — |
| `middleware/ctxinject` | 上下文注入（logger/metrics/DTM） | active | — | — |
| `middleware/logging` | 请求日志 | active | — | — |
| `middleware/metadata` | 元数据传播 | active | — | — |
| `middleware/recovery` | Panic 恢复 | active | — | — |
| `middleware/requestinfo` | 请求元信息解析 | active | — | — |
| `middleware/retry` | 客户端重试 | active | — | — |
| `middleware/selector` | 节点选择 | active | — | — |
| `middleware/timeout` | 请求超时 | active | — | — |
| `middleware/validate` | 请求校验 | active | — | — |

### 服务组装

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `serverx` | 服务装配（启动校验/传输构建/系统路由/生命周期） | active | [contract](contracts/serverx.md) | — |
| `servicecontextx` | 服务上下文（客户端引用管理） | active | — | — |

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

### 启动校验

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `bootx` | 启动治理校验（限流/拓扑/Provider 完整性） | active | [contract](contracts/boot-governance-validation.md) | — |

### IAM 领域

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `iamx` | IAM 领域错误和辅助 | active | [domain](contracts/iam-domain.md) | — |

### REST 辅助

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `restx` | HTTP 运行时辅助（过滤器链/CORS/熔断/自适应降级） | active | — | — |

### gRPC 运行时

| 包 | 说明 | 状态 | 文档 | 示例 |
|---|---|---|---|---|
| `grpcx` | gRPC 运行时辅助（拦截器/上下文传播/errorx 适配） | active | — | — |

### CLI 工具

| 工具 | 说明 | 文档 |
|---|---|---|
| `cmd/kernel` | 项目脚手架（`kernel new` / `kernel version` / `kernel run` / `kernel upgrade` / `kernel proto` / `kernel change`） | [getting-started](getting-started.md) |
| `cmd/protoc-gen-go-http` | 从 proto `google.api.http` 生成 HTTP 路由注册 | — |
| `cmd/protoc-gen-go-errors` | 从 proto error 定义生成 errorx 辅助函数 | — |
| `cmd/protoc-gen-go-authz` | 从 proto access policy 生成 RequestInfoResolver 和 AccessResolver | — |
| `cmd/protoc-gen-go-gateway` | 生成 Gateway Manifest / Binding / Invoker 注册 | — |
| `cmd/protoc-gen-go-kernel` | 生成 `serverx.ServiceModule`（`<Service>KernelModule()`） | — |
| `cmd/buf-check-aisphere` | Proto 合约检查器（access policy / error codes 等） | — |

### 第三方集成

| 分类 | 适配后端 |
|---|---|
| 配置后端 | consul, etcd, kubernetes, nacos, polaris, apollo |
| 注册中心 | consul, etcd, kubernetes, nacos, polaris, eureka, zookeeper, servicecomb, discovery |
| 遥测 | OTel (log/metrics/tracing) |
| 中间件 | JWT, Validator |
| 编码 | msgpack |
| 错误追踪 | Sentry |
| 流量治理 | OpenSergo, Polaris |

### 示例项目

| 示例 | 覆盖能力 |
|---|---|
| [authn-casdoor](../examples/authn-casdoor/) | Casdoor 认证集成 |
| [authz-spicedb](../examples/authz-spicedb/) | SpiceDB 授权集成 |
| [cachex-basic](../examples/cachex-basic/) | 缓存读写 + GetOrSet |
| [configx-basic](../examples/configx-basic/) | 文件配置加载 |
| [configx-env](../examples/configx-env/) | 环境变量配置 |
| [configx-watch](../examples/configx-watch/) | 配置热更新 |
| [contextx-basic](../examples/contextx-basic/) | 上下文值注入 |
| [dbx-basic](../examples/dbx-basic/) | 数据库 CRUD |
| [dbx-scenarios](../examples/dbx-scenarios/) | 数据库高级场景 |
| [dtmx-saga](../examples/dtmx-saga/) | DTM Saga 分布式事务 |
| [errorx-basic](../examples/errorx-basic/) | 业务错误定义与使用 |
| [errorx-http](../examples/errorx-http/) | HTTP 错误响应 |
| [gatewayx-basic](../examples/gatewayx-basic/) | Gateway 路由与分发 |
| [logx-basic](../examples/logx-basic/) | 结构化日志 |
| [logx-http](../examples/logx-http/) | HTTP 请求日志 |
| [metricsx-basic](../examples/metricsx-basic/) | 业务指标采集 |
| [objectstorex-basic](../examples/objectstorex-basic/) | 对象存储操作 |
| [proto-authz](../examples/proto-authz/) | Proto 声明式授权 |
| [secure-skill-service](../examples/secure-skill-service/) | 完整安全服务示例 |

### 快速索引

按场景查找对应能力：

| 场景 | 入口 |
|---|---|
| 想创建新服务 | `kernel new` → [getting-started](getting-started.md) |
| 想加配置 | `configx` |
| 想加日志 | `logx` |
| 想加指标 | `metricsx` |
| 想加数据库 | `dbx` + `migrationx` + `dbrepo` |
| 想加缓存 | `cachex` |
| 想加对象存储 | `objectstorex` |
| 想加分布式事务 | `dtmx` |
| 想加认证（登录/登出/令牌） | `authn` + `authn/casdoor` |
| 想加授权 | `authz` + `authz/spicedb` |
| 想加审计 | `auditx` |
| 想加限流 | `ratelimitx` |
| 想加准入校验 | `admissionx` |
| 想加服务间调用治理 | `clientpolicyx` |
| 想加 Gateway 路由 | `gatewayx` |
| 想加服务注册发现 | `registry` + `selectorx` |
| 想自定义错误 | `errorx` |
| 想了解服务装配 | `serverx` |
| 想了解请求链路 | `requestx` + `contextx` |
| 想了解传输层 | `transportx/http` + `transportx/grpc` |
| 想了解中间件 | `middleware/autowire` |
| 想了解启动校验 | `bootx` |
| 想了解 IAM 领域 | `iamx` |
| 想了解编码 | `encodingx` |
| 想了解 REST 辅助 | `restx` |
| 想了解 gRPC 辅助 | `grpcx` |

---

## 4. Contract：写业务前必须对齐

| 主题 | 文档 |
|---|---|
| Access Policy | [contracts/access-policy.md](contracts/access-policy.md) |
| RequestInfo | [contracts/request-info.md](contracts/request-info.md) |
| 生成 RequestInfo | [contracts/generated-request-info.md](contracts/generated-request-info.md) |
| Admission | [contracts/admission.md](contracts/admission.md) |
| 服务间调用 | [contracts/downstream-policy.md](contracts/downstream-policy.md) |
| 限流 | [contracts/rate-limit.md](contracts/rate-limit.md) |
| Serverx | [contracts/serverx.md](contracts/serverx.md) |
| Gateway 路由 | [contracts/gateway-routing.md](contracts/gateway-routing.md) |
| 启动治理校验 | [contracts/boot-governance-validation.md](contracts/boot-governance-validation.md) |
| 数据开发范式 | [contracts/dbx-data-development.md](contracts/dbx-data-development.md) |

## 4. Guides：业务组件开发

| 主题 | 文档 |
|---|---|
| IAM + Gateway 开发指南 | [guides/iam-gateway-development.md](guides/iam-gateway-development.md) |

## 5. Design

| 主题 | 文档 |
|---|---|
| Kubernetes apiserver 可学习范式 | [design/k8s-apiserver-lessons.md](design/k8s-apiserver-lessons.md) |
| Kernel 边界与分层 | [architecture/kernel-boundary.md](architecture/kernel-boundary.md) |
| AuthN/AuthZ/Audit 设计 | [design/security-authn-authz-auditx.md](design/security-authn-authz-auditx.md) |

## 6. Validation 状态

`validation/` 已从 runtime tree 移除。场景检查、generated-shape 实验、IAM/Gateway/SkillService 联调验证，不再放在 Kernel 主模块默认包图里。

后续需要验证业务时，使用独立仓库、生成项目自己的 tests、显式 build tag 或 GitHub Actions 专用 job。

## 7. AI / Agent 工作方式

| 主题 | 文档 |
|---|---|
| 开发范式 | [ai/kernel-development-paradigm.md](ai/kernel-development-paradigm.md) |
| 高级特性使用手册 | [ai/advanced-feature-playbook.md](ai/advanced-feature-playbook.md) |
| GitHub Actions 委托构建 | [ai/github-actions-build-delegation.md](ai/github-actions-build-delegation.md) |

AI 写业务代码前必须先判断需求是否命中高级特性触发条件。命中后必须使用对应 Kernel 包，不允许手写临时基础设施实现。

## 8. 文档维护规则

- 新业务开发只参考当前 README 链接出的文档。
- 历史分析、对比、阶段性草稿放入 `docs/archive/legacy/`。
- 文档默认使用中文。
- 代码能力变化时，必须同步更新对应 `docs/contracts/*.md`、`docs/ai/*.md` 和 `AGENTS.md`。
- 如果文档与代码冲突，以代码和测试为准，并立即修正文档。
