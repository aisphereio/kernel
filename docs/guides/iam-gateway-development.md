# 基于 Kernel 开发 IAM 与 Gateway 指南

本文面向开发者和 AI Agent，说明如何基于 Aisphere Kernel 开发两个核心业务组件：

- `iam-service`：统一认证、登录、身份视图、服务认证、权限管理入口。
- `gateway-service`：统一外部入口、Route Manifest 消费、边界准入、token relay、gRPC invoker 调用内部服务。

目标实现：

```text
Gateway:
  - 基于 etcd 的 Gateway Route Registry
  - 消费各业务服务发布的 Route Manifest
  - 外部 HTTP -> generated gRPC invoker -> 内部服务

IAM:
  - authn 默认基于 Casdoor
  - authz 默认基于 SpiceDB
  - 业务只依赖 Kernel authn/authz/accessx/auditx 接口
```

## 1. 先明确边界

### 1.1 Gateway 不是 IAM

Gateway 只做入口层职责：

```text
外部 HTTP path/method 匹配
PUBLIC / AUTHENTICATED / AUTHORIZED / INTERNAL / SYSTEM 边界判断
token 是否存在的快速准入
Authorization / request_id / trace / tenant metadata 透传
调用 generated gRPC invoker
```

Gateway 不做：

```text
资源级授权
用户、组织、角色管理
SpiceDB 权限决策
Casdoor 登录态管理
业务数据库查询
```

资源级授权必须在业务服务内通过 `accessx -> authn -> authz -> auditx` 执行。

### 1.2 IAM 不是 Gateway

IAM 负责登录、身份、权限管理入口：

```text
OAuth/OIDC 登录跳转
code exchange / refresh / verify / revoke
当前用户 / 组织 / 组 / 角色视图
service credential / internal token 验证
资源关系写入 / 查询 / 授权管理 API
```

IAM 不负责统一反向代理，也不替代 Gateway 路由分发。

### 1.3 Casdoor / SpiceDB / Kernel 分层

```text
Casdoor:
  用户中心、登录中心、组织/用户/组/角色管理后台

SpiceDB:
  资源级 ReBAC/ABAC 授权决策与 relationship 存储

Kernel:
  统一 authn/authz/accessx/auditx/serverx/gatewayx 抽象
```

业务代码不能直接 import Casdoor 或 SpiceDB SDK。provider 实现可以依赖 SDK，但 provider 必须隐藏在 Kernel 接口后面。

## 2. 总体架构

```text
Client / Browser / CLI
  |
  | HTTP
  v
Gateway Service
  - gatewayx.Dispatcher
  - gatewayx.EtcdRegistry
  - generated route matcher
  - generated gRPC invoker registry
  - passive token relay
  |
  | gRPC
  v
IAM Service / Hub Service / Runtime Service / Skill Service
  - serverx
  - requestx.Info
  - authn.Provider(Casdoor)
  - authz.Provider(SpiceDB)
  - accessx.Guard
  - auditx.Recorder
  - admissionx
  - dbx/cachex/objectstorex as needed
```

Gateway 的路由数据来自 etcd：

```text
Business service start/deploy
  -> generated <Service>GatewayManifest()
  -> serverx.RegisterGatewayRoutes(ctx, registry, manifest)
  -> gatewayx.EtcdRegistry writes /aisphere/kernel/routes/<env>/...
  -> Gateway watches/loads prefix
  -> Dispatcher match + invoke
```

注意区分两类注册：

| 类型 | 用途 | Kernel 包 | 推荐实现 |
|---|---|---|---|
| Gateway Route Registry | 保存外部路由、upstream operation、exposure、authn mode | `gatewayx.RouteRegistry` | etcd prefix |
| Service Registry / Discovery | 保存服务实例 endpoint | `registry` | K8s Service/DNS 优先，必要时 etcd/consul/nacos provider |

当前主线建议：Gateway 路由注册表走 etcd；内部服务寻址生产环境优先走 K8s Service DNS，本地可用 `gatewayx.StaticHosts`。

## 3. 推荐仓库与服务形态

建议先用两个独立生成项目验证，再决定是否进入 monorepo：

```text
iam-service/
  api/iam/v1/iam.proto
  cmd/iam-service/main.go
  configs/config.yaml
  internal/biz
  internal/data
  internal/provider
  migrations

gateway-service/
  api/gateway/v1/gateway.proto  # 只放管理/快照 API，不放所有业务路由
  cmd/gateway-service/main.go
  configs/config.yaml
  internal/biz
  internal/provider
```

生成命令：

```bash
kernel new iam-service --disable objectstore,dtmx
kernel new gateway-service --disable dbx,cachex,objectstore,dtmx
```

IAM 一般需要：

```text
configx logx errorx metricsx
serverx transportx/http transportx/grpc
requestx accessx authn authz auditx
cachex 可选，用于 token/user/provider cache
dbx/migrationx 可选，用于本地审计、投影视图、关系同步状态
```

Gateway 一般需要：

```text
configx logx errorx metricsx
serverx transportx/http transportx/grpc
requestx gatewayx ratelimitx clientpolicyx
registry 可选，用于服务发现扩展
```

Gateway 不应该依赖业务服务实现代码，只依赖生成出来的 client、manifest、binding、invoker 注册函数。

## 4. IAM Service 开发步骤

### 4.1 定义 IAM proto

IAM 的 proto 建议按功能拆服务：

```text
IAMAuthService:
  BuildLoginURL
  ExchangeCode
  RefreshToken
  VerifyToken
  RevokeToken
  GetMe

IAMDirectoryService:
  GetUser
  ListUsers
  GetOrganization
  ListOrganizations
  ListGroups

IAMPermissionService:
  CheckPermission
  WriteRelationship
  DeleteRelationship
  LookupResources
  LookupSubjects
```

access policy 建议：

| RPC | Exposure | 说明 |
|---|---|---|
| `BuildLoginURL` | PUBLIC | 前端获取登录地址 |
| `ExchangeCode` | PUBLIC | OAuth callback 后换 token |
| `RefreshToken` | AUTHENTICATED | 刷新当前登录态 |
| `VerifyToken` | INTERNAL | 内部服务校验 token 或 introspection |
| `GetMe` | AUTHENTICATED | 当前用户信息 |
| `GetUser/ListUsers` | AUTHORIZED | 管理 API |
| `WriteRelationship/DeleteRelationship` | AUTHORIZED 或 INTERNAL | 权限关系变更，高危，必须 audit |
| `CheckPermission` | INTERNAL | 内部授权检查，不对公网开放 |

示意：

```proto
service IAMAuthService {
  rpc BuildLoginURL(BuildLoginURLRequest) returns (BuildLoginURLReply) {
    option (google.api.http) = { get: "/v1/iam/login-url" };
    option (aisphere.access.v1.policy) = {
      exposure: PUBLIC
      action: "login"
      resource: "iam:session"
    };
  }

  rpc GetMe(GetMeRequest) returns (GetMeReply) {
    option (google.api.http) = { get: "/v1/iam/me" };
    option (aisphere.access.v1.policy) = {
      exposure: AUTHENTICATED
      action: "read"
      resource: "iam:user:self"
    };
  }
}
```

具体 option 名称以 `api/aisphere/*` 当前定义为准，文档表达的是 contract 方向。

### 4.2 生成代码与契约检查

```bash
make tools
make api
make proto-check
```

期望生成/检查出：

```text
IAMAuthService HTTP/gRPC binding
RequestInfoResolver
AccessResolver
GatewayManifest
KernelModule
errorx helpers
OpenAPI
```

如果 `buf-check-aisphere` 报错，不要绕过。应该补 proto 的 `google.api.http`、`aisphere.access.v1.policy` 或 audit 元数据。

### 4.3 接入 Casdoor authn provider

IAM 内部应该把 Casdoor 接成 Kernel `authn.Provider` / `authn.ManagementProvider`。

配置建议：

```yaml
iam:
  authn:
    provider: casdoor
    casdoor:
      endpoint: "http://casdoor:8000"
      organization: "aisphere"
      application: "aisphere-portal"
      client_id: "${CASDOOR_CLIENT_ID}"
      client_secret: "${CASDOOR_CLIENT_SECRET}"
      redirect_uri: "https://portal.example.com/callback"
      jwks_cache_ttl: "10m"
      token_cache_ttl: "2m"
```

provider 实现职责：

```text
BuildLoginURL -> Casdoor authorize URL
ExchangeCode -> Casdoor token endpoint
RefreshToken -> Casdoor refresh
VerifyToken -> JWT/JWKS 或 Casdoor introspection
GetUser/GetOrg/ListGroups -> Casdoor 管理 API adapter
```

IAM service 层只调用 Kernel authn 接口，不直接在 service 方法里调用 Casdoor SDK。

### 4.4 接入 SpiceDB authz provider

IAM 权限服务和所有业务服务都应该通过 Kernel `authz.Provider` / `authz.AdminProvider` 使用 SpiceDB。

配置建议：

```yaml
iam:
  authz:
    provider: spicedb
    spicedb:
      endpoint: "spicedb:50051"
      preshared_key: "${SPICEDB_PRESHARED_KEY}"
      insecure: true
      schema_bootstrap: true
      consistency: "minimize_latency"
```

SpiceDB schema 建议先围绕组织、项目、资源建模：

```text
definition user {}

definition organization {
  relation member: user
  relation admin: user
  permission view = member + admin
  permission manage = admin
}

definition project {
  relation parent: organization
  relation viewer: user
  relation editor: user
  relation owner: user
  permission view = viewer + editor + owner + parent->view
  permission edit = editor + owner + parent->manage
  permission manage = owner + parent->manage
}

definition resource {
  relation parent: project
  relation viewer: user
  relation editor: user
  relation owner: user
  permission view = viewer + editor + owner + parent->view
  permission edit = editor + owner + parent->edit
  permission delete = owner + parent->manage
}
```

资源命名建议统一：

```text
iam:organization:<org_id>
iam:project:<project_id>
aihub:skill:<skill_id>
aihub:agent:<agent_id>
runtime:tool:<tool_id>
```

### 4.5 IAM 的 serverx 装配

IAM main 不应该手写所有中间件。推荐路径：

```text
Load config
  -> 构建 Casdoor authn provider
  -> 构建 SpiceDB authz provider
  -> 构建 audit recorder
  -> generated IAM KernelModule
  -> serverx.BuildServiceFromFactory
  -> app.Run
```

伪代码：

```go
func main() {
    ctx := context.Background()

    cfg, err := serverx.LoadConfigFile("configs/config.yaml")
    if err != nil { panic(err) }

    providers := accessx.Providers{
        Authn: casdoorProvider,
        Authz: spicedbProvider,
        Audit: auditRecorder,
    }

    app, err := serverx.BuildServiceFromFactory(ctx,
        cfg,
        iamv1.IAMServiceKernelModule(),
        providerFactory(providers),
        newIAMService,
    )
    if err != nil { panic(err) }

    if err := app.Run(); err != nil { panic(err) }
}
```

实际函数签名以当前 `serverx` 代码为准；开发原则是不在业务 main 中手写 transport、authn/authz/audit middleware。

### 4.6 IAM 必须提供的内部能力

IAM 首版至少要具备：

```text
PUBLIC:
  BuildLoginURL
  ExchangeCode

AUTHENTICATED:
  GetMe
  RefreshToken
  Logout/RevokeToken

INTERNAL:
  VerifyToken / Introspect
  CheckPermission

AUTHORIZED:
  User/Org/Group/Role 管理视图
  Relationship write/delete
  Permission grant/revoke
```

高危动作必须 audit：

```text
grant
revoke
delete
transfer
owner
admin
share
publish
```

## 5. Gateway Service 开发步骤

### 5.1 Gateway 自己的 proto 要保持很薄

Gateway 的业务 proto 不应该列出所有业务路由。那些路由来自各服务的 Route Manifest。

Gateway 自己只需要管理/观测 API：

```text
GetRouteSnapshot
ReloadRoutes
GetUpstreamHealth
GetGatewayVersion
```

这类 API 通常是 INTERNAL/SYSTEM。

### 5.2 etcd Route Registry

Gateway 使用 `gatewayx.RouteRegistry`。生产默认走 etcd prefix：

```yaml
gateway:
  route_registry:
    provider: etcd
    prefix: "/aisphere/kernel/routes/prod"
    endpoints:
      - "http://etcd-0.etcd:2379"
      - "http://etcd-1.etcd:2379"
      - "http://etcd-2.etcd:2379"
    dial_timeout: "3s"
    request_timeout: "2s"
    watch: true
```

服务发布路由：

```text
IAM Service start/deploy
  -> iamv1.IAMAuthServiceGatewayManifest()
  -> serverx.RegisterGatewayRoutes(ctx, etcdRegistry, manifest)

Hub Service start/deploy
  -> hubv1.HubServiceGatewayManifest()
  -> serverx.RegisterGatewayRoutes(ctx, etcdRegistry, manifest)
```

Gateway 消费路由：

```text
Gateway start
  -> connect etcd
  -> load prefix /aisphere/kernel/routes/prod
  -> build matcher snapshot
  -> watch prefix changes
  -> hot reload route snapshot
```

本地开发可以先用：

```go
kv := gatewayx.NewMemoryKVStore()
registry := gatewayx.NewEtcdRegistry(kv, "/aisphere/kernel/routes/dev")
```

生产需要补真实 etcd KVStore adapter：

```go
type EtcdKVStore struct {
    client *clientv3.Client
}

func (s *EtcdKVStore) Put(ctx context.Context, key string, value []byte) error
func (s *EtcdKVStore) List(ctx context.Context, prefix string) ([]gatewayx.KVPair, error)
func (s *EtcdKVStore) Watch(ctx context.Context, prefix string) (<-chan gatewayx.KVEvent, error)
```

接口名称以当前 `gatewayx.KVStore` 为准，原则是用 etcd 承载 Route Manifest，不让 Gateway 手写路由表。

### 5.3 Gateway 到内部服务的调用

默认主路径是 generated gRPC invoker，不是 HTTP reverse proxy。

```text
External HTTP
  -> Gateway route match
  -> generated bind: HTTP path/query/body -> gRPC request
  -> generated invoker registry: operation -> gRPC client method
  -> internal service gRPC server
```

Gateway 启动时组装 invoker registry：

```go
invokers := gatewayx.NewInvokerRegistry()
_ = iamv1.RegisterIAMAuthServiceGatewayInvokers(invokers, iamv1.NewIAMAuthServiceClient(iamConn))
_ = hubv1.RegisterHubServiceGatewayInvokers(invokers, hubv1.NewHubServiceClient(hubConn))

dispatcher := gatewayx.NewDispatcher(routeRegistry, hosts, invokers)
```

内部服务连接建议通过 Kernel client factory 或 `serverx.ClientMiddlewareForDownstream` 注入：

```yaml
downstreams:
  iam-service:
    protocol: grpc
    target: "iam-service.aisphere.svc.cluster.local:9000"
    timeout: "2s"
    retry:
      enabled: true
      max_attempts: 2
    circuit_breaker:
      enabled: true
```

不要在 Gateway 业务代码里裸写 `grpc.Dial`、裸 `http.Client` 或手写 retry loop。

### 5.4 Gateway authn 策略

首版建议：Gateway 对业务 API 使用 passive authn。

```text
PUBLIC      -> 不要求 Authorization
SYSTEM      -> 不要求 Authorization 或只允许内网访问
AUTHENTICATED/AUTHORIZED -> 要求 Authorization 存在，然后 token relay
INTERNAL    -> 不对公网暴露；外部访问返回 404 或 403
```

也就是说：

```text
Gateway 只快速判断 token 是否存在
业务服务才真正调用 Casdoor provider 验 token
业务服务才真正调用 SpiceDB provider 判资源权限
```

后续如果入口压力大，可以增加 Gateway 本地 JWT verify：

```yaml
gateway:
  authn:
    mode: verify_jwt
    jwks_url: "http://casdoor:8000/.well-known/jwks"
    cache_ttl: "10m"
```

但即使 Gateway 本地 verify，业务服务仍然要执行权威 authn/authz，不要把 Gateway 当权限中心。

### 5.5 Gateway 系统路由

Gateway 应该由 `serverx` 挂系统路由：

```text
/healthz
/readyz
/version
/metrics
```

Gateway 自己可以额外提供：

```text
/gateway/routes       INTERNAL/SYSTEM，返回当前 route snapshot
/gateway/upstreams    INTERNAL/SYSTEM，返回 upstream 状态
```

不要在业务 proto 里重复定义系统路由。

## 6. IAM 与 Gateway 的联调流程

### 6.1 启动依赖

本地最小依赖：

```text
Casdoor
SpiceDB
etcd
PostgreSQL 可选，用于审计/投影视图
Redis 可选，用于 token/user/cache
```

### 6.2 启动顺序

```text
1. 启动 Casdoor，配置 organization/application/client_id/client_secret/redirect_uri
2. 启动 SpiceDB，加载 schema
3. 启动 etcd
4. 启动 iam-service
   - 连接 Casdoor provider
   - 连接 SpiceDB provider
   - 注册 IAM Route Manifest 到 etcd
5. 启动 gateway-service
   - 从 etcd 加载 Route Manifest
   - 连接 iam-service gRPC client
   - 构建 invoker registry
6. 调 Gateway 外部 HTTP API 验证
```

### 6.3 三段验证

未登录：

```text
GET /v1/iam/me
Gateway 发现 route=AUTHENTICATED
没有 Authorization
返回 401
iam-service handler 不执行
```

登录但无权限：

```text
GET /v1/iam/users
Gateway passive relay token
IAM authn/casdoor 验 token 成功
IAM accessx/authz/spicedb 判权限失败
返回 403
audit denied
```

登录且有权限：

```text
GET /v1/iam/users
Gateway passive relay token
IAM authn/casdoor 验 token 成功
IAM authz/spicedb 判权限成功
handler 执行
返回 200
audit success
```

### 6.4 Route Registry 验证

检查 etcd prefix：

```bash
etcdctl get --prefix /aisphere/kernel/routes/dev
```

期望看到每个服务发布的 manifest 路由：

```text
/aisphere/kernel/routes/dev/iam-service/...
/aisphere/kernel/routes/dev/hub-service/...
```

Gateway route snapshot 应包含：

```text
method
path
exposure
authn_mode
upstream.service
upstream.protocol
upstream.operation
```

## 7. 开发任务拆分

### 第一阶段：跑通骨架

IAM：

```text
1. kernel new iam-service
2. 定义 IAMAuthService proto
3. 生成 RequestInfoResolver / GatewayManifest / KernelModule
4. 用 memory authn/authz provider 跑通 serverx
5. 注册 Route Manifest 到 MemoryKVStore 模拟的 EtcdRegistry
```

Gateway：

```text
1. kernel new gateway-service
2. 构建 gatewayx.Dispatcher
3. 从 MemoryKVStore 读取 IAM Route Manifest
4. 用 UnaryInvoker 或 bufconn 调 IAM
5. 验证 PUBLIC / AUTHENTICATED / AUTHORIZED / INTERNAL 分支
```

### 第二阶段：接入真实 provider

IAM：

```text
1. 实现 authn/casdoor provider
2. 实现 authz/spicedb provider
3. 实现 audit recorder
4. 用 Casdoor 完成 login_url -> exchange_code -> get_me
5. 用 SpiceDB 完成 write_relationship -> check_permission
```

Gateway：

```text
1. 实现 etcd KVStore adapter
2. Gateway 启动从 etcd prefix load routes
3. IAM 启动/部署注册 Route Manifest 到 etcd
4. Gateway 到 IAM 使用 generated gRPC invoker
```

### 第三阶段：生产化

```text
1. Gateway route watch 热更新
2. Route snapshot 系统 API
3. Casdoor JWKS/token cache
4. SpiceDB schema bootstrap / migration 管理
5. service-auth / internal token
6. audit 落 PG/Kafka/ClickHouse
7. metrics/tracing/log 全链路字段统一
8. GitHub Actions 跑 kernel new -> make api -> make proto-check -> make verify
```

## 8. 配置模板

### 8.1 iam-service config.yaml

```yaml
app:
  name: iam-service
  version: dev

server:
  http:
    enabled: true
    addr: ":18080"
  grpc:
    enabled: true
    addr: ":19080"

security:
  authn:
    required: true
    provider: casdoor
  authz:
    required: true
    provider: spicedb
  audit:
    required: true

iam:
  authn:
    provider: casdoor
    casdoor:
      endpoint: "http://casdoor:8000"
      organization: "aisphere"
      application: "aisphere-portal"
      client_id: "${CASDOOR_CLIENT_ID}"
      client_secret: "${CASDOOR_CLIENT_SECRET}"
      redirect_uri: "http://localhost:3000/callback"
  authz:
    provider: spicedb
    spicedb:
      endpoint: "spicedb:50051"
      preshared_key: "${SPICEDB_PRESHARED_KEY}"
      insecure: true
  gateway:
    route_registry:
      provider: etcd
      prefix: "/aisphere/kernel/routes/dev"
      endpoints:
        - "http://etcd:2379"

database:
  enabled: true
  driver: postgres
  dsn: "${IAM_DATABASE_DSN}"
  migrations:
    enabled: true
    dir: "migrations"
    mode: apply
```

### 8.2 gateway-service config.yaml

```yaml
app:
  name: gateway-service
  version: dev

server:
  http:
    enabled: true
    addr: ":18000"
  grpc:
    enabled: false

security:
  authn:
    required: false
  authz:
    required: false
  audit:
    required: false

gateway:
  route_registry:
    provider: etcd
    prefix: "/aisphere/kernel/routes/dev"
    endpoints:
      - "http://etcd:2379"
    watch: true
  authn:
    mode: passive
  upstreams:
    iam-service:
      protocol: grpc
      target: "iam-service:19080"
      timeout: "2s"
      retry:
        enabled: true
        max_attempts: 2
      circuit_breaker:
        enabled: true
```

配置字段最终以 `serverx.Config`、`gatewayx`、provider config 的代码定义为准。文档里的 YAML 是目标结构和落地方向。

## 9. AI Agent 开发硬规则

AI 开发 IAM/Gateway 时必须遵守：

```text
1. 先写 proto contract，再写 service 实现。
2. 新外部 RPC 必须有 google.api.http 和 aisphere.access.v1.policy。
3. Gateway 路由必须来自 generated GatewayManifest。
4. Route Manifest 必须注册到 gatewayx.RouteRegistry；生产用 etcd prefix。
5. Gateway 只做边界准入和 token relay，不做资源级授权。
6. IAM authn provider 默认 Casdoor。
7. IAM authz provider 默认 SpiceDB。
8. 业务服务只调用 Kernel authn/authz/accessx/auditx 接口，不直接依赖 provider SDK。
9. 服务启动必须通过 serverx。
10. 服务间调用必须走 clientpolicyx/serverx client governance chain。
11. 限流必须走 ratelimitx。
12. 错误必须走 errorx，日志必须走 logx。
13. 场景验证不放回 Kernel runtime tree，放 generated project tests 或专用 GitHub Actions job。
```

## 10. 验收清单

IAM 验收：

```text
[ ] make api 成功
[ ] make proto-check 成功
[ ] make verify 成功
[ ] BuildLoginURL PUBLIC 可访问
[ ] ExchangeCode 可换 token
[ ] GetMe 无 token 返回 401
[ ] GetMe 有 token 返回当前用户
[ ] CheckPermission INTERNAL 外部不可访问
[ ] WriteRelationship 高危操作有 audit
[ ] Casdoor SDK 只出现在 provider 层
[ ] SpiceDB SDK 只出现在 provider 层
```

Gateway 验收：

```text
[ ] Gateway 不手写业务 route table
[ ] etcd prefix 可看到 IAM Route Manifest
[ ] Gateway route snapshot 可看到 IAM routes
[ ] PUBLIC 路由无 token 可访问
[ ] AUTHENTICATED/AUTHORIZED 路由无 token 返回 401
[ ] 有 token 时能 relay 到 IAM gRPC
[ ] INTERNAL 路由公网不可访问
[ ] Gateway 不做资源级 authz
[ ] Gateway 到 IAM 调用走 generated gRPC invoker
[ ] 下游 timeout/retry/breaker 走 clientpolicyx/serverx
```

联调验收：

```text
[ ] 未登录 -> Gateway 401 -> IAM handler 不执行
[ ] 已登录无权限 -> IAM authn 成功 -> SpiceDB authz denied -> 403 + audit denied
[ ] 已登录有权限 -> IAM authn 成功 -> SpiceDB authz allowed -> 200 + audit success
[ ] Route Manifest 更新后 Gateway 能 reload 或重启加载新路由
```
