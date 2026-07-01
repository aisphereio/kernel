# AI 高级特性使用手册

本文给 AI Agent 使用。目标是让 AI 在业务开发时知道：什么时候必须使用 Kernel 的高级能力，应该怎么用，以及绝不能绕过什么。

## 1. 总判断流程

遇到业务需求时，先按下面顺序判断：

```text
1. 这个需求是否暴露 API？
   是 -> 先写 proto contract、google.api.http、aisphere.access.v1.policy。

2. 这个需求是否需要 HTTP/gRPC 服务启动？
   是 -> 使用 serverx，不手写 transport glue。

3. 这个需求是否涉及请求身份、租户、资源、动作、调用方？
   是 -> 使用 requestx.Info，不解析 raw path 或 raw grpc method。

4. 这个需求是否涉及登录、权限、审计？
   是 -> 使用 authn/authz/accessx/auditx，不直接依赖 Casdoor/SpiceDB/Casbin/OPA SDK。

5. 这个需求是否涉及 Gateway 路由？
   是 -> 使用 proto 生成 gatewayx Route Manifest，不手写业务路由表。

6. 这个需求是否涉及跨接口默认值、状态流转、发布/删除/分享前校验？
   是 -> 使用 admissionx，不把通用规则散落在 handler/service。

7. 这个需求是否涉及限流、重试、超时、熔断？
   是 -> 使用 ratelimitx/clientpolicyx/serverx client governance chain。

8. 这个需求是否涉及数据库、缓存、对象存储、分布式事务？
   是 -> 使用 dbx/cachex/objectstorex/dtmx/migrationx/dbrepo，不直接使用底层 SDK。
```

如果 Kernel 当前表达不了需求，先补 Kernel 抽象和文档，再写业务代码。

## 2. 高级能力选择矩阵

| 场景 | 必须使用 | 生成/修改内容 | 禁止行为 |
|---|---|---|---|
| 新增对外 API | `api/` proto、`buf-check-aisphere`、`protoc-gen-go-*` | `.proto`、access policy、生成代码 | 手写 HTTP 路由和权限 glue |
| 启动服务 | `serverx` | `serverx.Config`、generated `KernelModule()`、业务 factory | 业务自己 new HTTP/gRPC server |
| 请求上下文 | `requestx` | generated resolver、middleware 注入 | 解析 raw path、raw method、operation string |
| 登录认证 | `authn` | provider 配置、credential extractor | 业务直接调用 Casdoor SDK |
| 授权审计 | `authz`、`accessx`、`auditx` | access resolver、authorizer、audit recorder | 业务绕过 guard 自己判权限 |
| Gateway 路由 | `gatewayx` | Route Manifest、InvokerRegistry、route registry | 手写 Gateway 业务路由表 |
| 边界准入 | `admissionx` | mutating/validating plugin | 把通用默认值/状态校验写散 |
| 服务间调用 | `clientpolicyx`、`serverx.ClientMiddlewareForDownstream` | downstream policy、timeout/retry/breaker/limiter | 裸 `grpc.Dial`、裸 `http.Client`、手写 retry loop |
| 限流 | `ratelimitx` | policy/provider、bootx/serverx 校验 | 使用旧 `middleware/ratelimit` |
| 数据库 | `dbx`、`migrationx`、`dbrepo` | SQL migration、repository、serverx database config | 业务直接 import gorm/sql/driver |
| 缓存 | `cachex` | cache config、cache repository | 业务直接 import go-redis |
| 对象存储 | `objectstorex` | bucket/object config、object repository | 业务直接 import MinIO/S3 SDK |
| 分布式事务 | `dtmx` | saga/tcc branch endpoint、idempotency key | 在业务里手写补偿状态机 |
| 日志 | `logx` | structured fields、context logger | `log.Println`、`fmt.Println`、直接 zap/slog |
| 错误 | `errorx` | 稳定 error_code、HTTP/gRPC 映射、安全 metadata | 裸 `errors.New`、`fmt.Errorf`、panic |
| 指标 | `metricsx` | 低基数 label、histogram/counter/gauge | 高基数字段进 label |
| 注册发现 | `registry` | registrar/discovery provider | 自造 registryx 或手写服务发现 |

## 3. 新业务 API 开发步骤

AI 新增一个业务能力时，按这个顺序提交代码：

```text
1. 修改 api/<domain>/<version>/*.proto
2. 为每个外部 RPC 声明 google.api.http
3. 为每个 RPC 声明 aisphere.access.v1.policy
4. 高危动作补 audit 元数据
5. 运行 make api
6. 运行 make proto-check
7. 使用生成的 RequestInfoResolver / AccessResolver / GatewayManifest / KernelModule
8. 只实现业务 service 和 repository
9. 通用默认值和跨接口校验写 admissionx plugin
10. 运行 make verify
```

不要先写 handler，再回头补 proto。Kernel 的方向是 proto contract first。

## 4. serverx 使用规则

当 AI 需要创建服务、启动 HTTP/gRPC、挂系统路由、装配中间件、连接 DB 或注册 Gateway 路由时，必须优先使用 `serverx`。

应该做：

```text
serverx.LoadConfigFile / DecodeFileConfig
  -> generated <Service>KernelModule()
  -> serverx.BuildServiceFromFactory
  -> app.Run()
```

不要做：

```text
http.ListenAndServe
net/http mux 手写业务路由
grpc.NewServer 手写所有 interceptor
gateway 里手写业务 route table
业务 main 里手动拼 authn/authz/audit/ratelimit middleware
```

低层 `transportx/http` 和 `transportx/grpc` 是 framework/runtime 表面，普通业务优先通过 `serverx` 间接使用。

## 5. requestx 使用规则

只要逻辑需要知道“当前请求是谁、访问哪个资源、做什么动作、从哪个服务来、是否 INTERNAL/SYSTEM”，必须从 `requestx.Info` 读。

应该做：

```go
info, ok := requestx.FromContext(ctx)
```

不要做：

```text
从 HTTP path 推断资源
从 gRPC full method 字符串推断动作
在 service 里重复写 operation/action/resource 映射
```

映射应该由 proto option 和 generator 生成 resolver。

## 6. accessx/authn/authz/auditx 使用规则

只要接口不是纯 PUBLIC，AI 必须检查 access policy 是否清晰。

推荐边界：

```text
PUBLIC         login/register/oauth callback/health-like external landing
AUTHENTICATED /me/logout/refresh/current user info
AUTHORIZED    业务资源 CRUD/share/publish/delete
INTERNAL       服务间调用
SYSTEM         healthz/readyz/metrics/version/debug
```

业务服务必须通过 `accessx.Guard` 组合 authn/authz/audit。Gateway 只做边界认证和 token relay，不做最终资源级授权。

Provider 边界：

```text
authn provider: Casdoor/JWT/OIDC 等
uthz provider: SpiceDB/OpenFGA/Casbin/OPA 等
audit provider: PG/Kafka/ClickHouse/object storage 等
```

业务代码不能直接依赖这些 provider SDK。

## 7. gatewayx 使用规则

当需求出现“统一入口、外部路由、Gateway 转发到 IAM/Hub/Runtime/SkillService”时，AI 必须使用 `gatewayx`。

主流程：

```text
proto google.api.http + access policy
  -> protoc-gen-go-gateway
  -> <Service>GatewayManifest()
  -> serverx.RegisterGatewayRoutes
  -> gatewayx.RouteRegistry
  -> gatewayx.Dispatcher
  -> generated gRPC invoker
```

Gateway 的职责：

```text
1. 匹配外部 path/method
2. 做边界 exposure 判断
3. PUBLIC/AUTHENTICATED/AUTHORIZED/INTERNAL/SYSTEM 初步准入
4. token relay / metadata propagation
5. 调内部服务 generated gRPC client
```

Gateway 不做：

```text
资源级授权
业务状态机校验
直接操作 DB
手写每个业务服务的 route table
```

## 8. admissionx 使用规则

当 AI 发现同一类默认值或校验会在多个接口重复出现，应放入 admissionx。

适合 admissionx：

```text
tenant/project/owner 默认值
visibility/lifecycle 默认值
page size 上限
发布前检查
删除前依赖检查
分享/授权前检查
状态机流转校验
跨字段业务不变量
```

不适合 admissionx：

```text
单个 repository 的 SQL 条件
单个 service 私有的小分支逻辑
底层 provider 连接初始化
```

顺序固定：

```text
requestinfo -> authn -> ratelimit -> access/authz/audit -> admission -> business
```

## 9. clientpolicyx / ratelimitx 使用规则

服务间调用必须由 Kernel 治理链路承载。

当需求出现下列关键词时，AI 应使用 `clientpolicyx` 或 `serverx.ClientMiddlewareForDownstream`：

```text
调用 IAM/Hub/Runtime/Sandbox/SkillService
timeout
retry
breaker
circuit breaker
rate limit
backoff
下游不稳定
跨服务 metadata 透传
```

限流只能使用 `ratelimitx`。不要恢复旧 `middleware/ratelimit`。

## 10. dbx / migrationx / dbrepo 使用规则

当需求涉及持久化，AI 必须按数据开发范式处理：

```text
SQL migration 是数据库结构唯一来源
proto 只描述 API 和 access contract
repository 通过 dbx/dbrepo 实现
serverx 负责 database config、migration apply/validate、连接关闭
```

应该做：

```text
migrations/*.sql
repository 使用 dbx.DB / dbrepo.ResourceRepository[T]
业务边界把数据错误转成 errorx
```

不要做：

```text
SQL 写进 proto option
业务直接 import database/sql、gorm driver、pgx、mysql driver
启动时随意 AutoMigrate 生产表
```

`migrationx` 支持 apply/validate/dev_apply/gorm_dev_auto 等模式。生产环境优先 validate/apply，不要默认 gorm_dev_auto。

## 11. cachex / objectstorex / dtmx 使用规则

出现缓存：使用 `cachex`，不要直接用 go-redis。

出现文件、模型、音频、图片、导入导出对象：使用 `objectstorex`，不要直接用 MinIO/S3 SDK。

出现跨服务一致性、补偿、Saga/TCC：使用 `dtmx`。分支动作必须具备幂等性，补偿接口必须可重试，大对象不要放进事务 payload。

## 12. AI 输出代码前的自检清单

每次 AI 准备输出代码或 patch 前，先检查：

```text
是否新增 proto contract？
是否补 access policy？
是否需要 audit？
是否需要 requestx resolver？
是否需要 gateway manifest？
是否通过 serverx 装配？
是否有跨接口规则该进入 admissionx？
是否用了 ratelimitx/clientpolicyx？
是否绕过了 dbx/cachex/objectstorex？
是否用了 errorx/logx/metricsx？
是否更新 docs/contracts 或 docs/ai？
是否能通过 make api、make proto-check、make verify？
```

如果任何一项答案不清楚，先回到 contract 和 Kernel 抽象，不要直接写业务 workaround。
