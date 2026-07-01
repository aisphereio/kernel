# Aisphere Kernel 文档中心

本目录只保留当前 Kernel 开发范式需要阅读的主线文档。过期、阶段性或历史对比材料已经移入 `docs/archive/legacy/`，不要作为新业务开发依据。

## 1. 先读这三份

1. [Kernel 边界与分层](architecture/kernel-boundary.md)
2. [规范驱动开发范式](ai/kernel-development-paradigm.md)
3. [Agent 开发规则](../AGENTS.md)

这三份文档定义了 Kernel 的核心原则：**业务只声明契约，框架负责检查、生成、装配、治理和验证**。

## 2. Contract：写业务前必须对齐

| 主题 | 文档 | 作用 |
|---|---|---|
| Access Policy | [contracts/access-policy.md](contracts/access-policy.md) | 定义 PUBLIC / AUTHENTICATED / AUTHORIZED / INTERNAL / SYSTEM |
| RequestInfo | [contracts/request-info.md](contracts/request-info.md) | 请求元信息中心，禁止业务解析 raw path/method |
| 生成 RequestInfo | [contracts/generated-request-info.md](contracts/generated-request-info.md) | 由 proto option 生成 request resolver |
| Admission | [contracts/admission.md](contracts/admission.md) | 跨接口默认值、准入校验、状态机规则 |
| 服务间调用 | [contracts/downstream-policy.md](contracts/downstream-policy.md) | 下游调用 timeout/retry/breaker/service-auth |
| 限流 | [contracts/rate-limit.md](contracts/rate-limit.md) | memory/redis/external、单副本/多副本语义 |
| Serverx | [contracts/serverx.md](contracts/serverx.md) | 一键服务装配、系统路由、transport 生命周期 |
| Gateway 路由 | [contracts/gateway-routing.md](contracts/gateway-routing.md) | Route Manifest、RouteRegistry、K8s Service upstream |
| 启动治理校验 | [contracts/boot-governance-validation.md](contracts/boot-governance-validation.md) | 多副本、provider、authn/authz/audit 配置校验 |
| 数据开发范式 | [contracts/dbx-data-development.md](contracts/dbx-data-development.md) | SQL migration、dbx、migrationx、dbrepo 的规范 |

## 3. Design：框架设计依据

| 主题 | 文档 |
|---|---|
| Kubernetes apiserver 可学习范式 | [design/k8s-apiserver-lessons.md](design/k8s-apiserver-lessons.md) |
| Kernel 边界与分层 | [architecture/kernel-boundary.md](architecture/kernel-boundary.md) |
| AuthN/AuthZ/Audit 设计 | [design/security-authn-authz-auditx.md](design/security-authn-authz-auditx.md) |

## 4. Validation：验证业务代码放这里

验证代码不属于 Kernel 核心能力包，统一放在 `validation/`。

| 验证场景 | 文档 | 测试命令 |
|---|---|---|
| IAM + Gateway 治理链路 | [demos/iam-gateway-demo.md](demos/iam-gateway-demo.md) | `go test ./validation/iamgateway` |
| 平台完整链路验证 | [demos/platform-gateway-iam-skill-flow.md](demos/platform-gateway-iam-skill-flow.md) | `go test ./validation/platformflow` |

## 5. AI / Agent 工作方式

| 主题 | 文档 |
|---|---|
| 开发范式 | [ai/kernel-development-paradigm.md](ai/kernel-development-paradigm.md) |
| GitHub Actions 委托构建 | [ai/github-actions-build-delegation.md](ai/github-actions-build-delegation.md) |

## 6. 文档维护规则

- 新业务开发只参考当前 README 链接出的文档。
- 历史分析、对比、阶段性草稿放入 `docs/archive/legacy/`。
- 文档默认使用中文。
- 代码能力变化时，必须同步更新对应 `docs/contracts/*.md` 和 `AGENTS.md`。
- 如果文档与代码冲突，以代码和测试为准，并立即修正文档。

## IAM 与 Casdoor 边界

- `docs/architecture/iam-boundary.md`：Kernel IAM 与 Casdoor 的职责边界。
- `docs/contracts/iam-domain.md`：用户、组织、多级组、membership 的 Kernel 领域模型规范。
- `docs/reference/casdoor-study.md`：Casdoor 源码学习记录和采纳/不采纳决策。

## 身份与权限边界

- Authn/Authz Provider 边界：`docs/architecture/authn-authz-provider-boundary.md`
- IAM/Casdoor 边界：`docs/architecture/iam-boundary.md`
- IAM 领域视图规范：`docs/contracts/iam-domain.md`

## 验证业务与开发流程

- [IAM 微服务 Kernel 开发流程](demos/iam-service-kernel-flow.md)

## 新增服务治理文档

- [服务配置契约：HTTP / gRPC](contracts/service-config.md)
- [Gateway 到服务真实 gRPC 验证链路](demos/platform-real-grpc-flow.md)

## 自动装载

- [ServiceModule 自动装载契约](contracts/service-module-autoload.md)
- [Gateway / IAM / SkillService 自动装载验证流](demos/platform-autoload-flow.md)
