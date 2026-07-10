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

## 10. 常见陷阱与实战经验

本章记录在 IAM 服务开发过程中遇到的实际问题和解决方案，帮助后续开发者避免重复踩坑。

### 10.1 Casdoor OAuth Code Exchange 失败（500 AUTHN_IDENTITY_BACKEND_FAILED）

**现象**：前端回调 Casdoor 拿到 `code` 后，调用 `POST /v1/iam/auth/exchange` 返回 500，错误码 `AUTHN_IDENTITY_BACKEND_FAILED`，消息 `"casdoor code exchange failed"`。

**根因**：Kernel 的 `casdoor.Client` 默认使用 Casdoor SDK 的 `GetOAuthToken(code, state)` 方法交换 code。但 Casdoor SDK 的 `GetOAuthToken` 在构造 `oauth2.Config` 时**没有设置 `RedirectURL`**（SDK 源码中该行被注释掉了）。Casdoor 的 token endpoint 要求 `redirect_uri` 参数与授权请求中的 `redirect_uri` 匹配，缺少该参数导致 Casdoor 拒绝交换。

**解决方案**：在 IAM 中创建 `casdoorClockSkewProvider` 包装器，重写 `ExchangeCode` 方法，使用标准 `golang.org/x/oauth2` 库手动构造 `oauth2.Config` 并设置 `RedirectURL`：

```go
config := oauth2.Config{
    ClientID:     p.cfg.ClientID,
    ClientSecret: p.cfg.ClientSecret,
    RedirectURL:  strings.TrimSpace(req.RedirectURI),  // 关键：必须设置
    Endpoint: oauth2.Endpoint{
        AuthURL:   endpoint + "/api/login/oauth/authorize",
        TokenURL:  endpoint + "/api/login/oauth/access_token",
        AuthStyle: oauth2.AuthStyleInParams,
    },
}
token, err := config.Exchange(ctx, req.Code, opts...)
```

**对比 Hub 实现**：Hub 的 `data/authn.go` 直接调用 `svc.ExchangeCode(ctx, authn.AuthCodeExchangeRequest{...})`，使用的是 Kernel 的 `casdoor.Client`（即 `authn/casdoor/token.go` 中的 `exchangeOAuthToken`）。Kernel 的 `exchangeOAuthToken` **已经正确设置了 `RedirectURL`**。而 IAM 使用了 `casdoorClockSkewProvider` 包装，该包装的原始实现调用了 `p.sdk.GetOAuthToken(req.Code, req.State)`（SDK 方法，缺少 `RedirectURL`），导致失败。

**教训**：包装 Kernel provider 时，必须检查被包装方法是否完整实现了所有参数传递。Casdoor SDK 的 `GetOAuthToken` 和 Kernel 的 `exchangeOAuthToken` 行为不一致。

### 10.2 Casdoor JWT 时钟偏差（Clock Skew）

**现象**：Casdoor 签发的 JWT 在 IAM 服务端验证时偶尔失败，提示 token 过期或未生效。

**根因**：Casdoor 服务器和 IAM 服务之间的系统时间可能存在偏差（尤其是在 Docker 或跨主机部署时）。JWT 的 `iat`（签发时间）和 `exp`（过期时间）校验是严格的，默认不允许时钟偏差。

**解决方案**：在 `casdoorClockSkewProvider` 中，重写 `VerifyToken` 和 `principalFromAccessToken` 方法，在解析 JWT 时设置 60 秒的时钟偏差：

```go
const casdoorTokenClockLeeway = 60 * time.Second

func (p *casdoorClockSkewProvider) parseJwtTokenWithLeeway(token string) (*casdoorsdk.Claims, error) {
    jwtTimeFuncMu.Lock()
    defer jwtTimeFuncMu.Unlock()
    previous := jwt.TimeFunc
    jwt.TimeFunc = func() time.Time { return time.Now().Add(casdoorTokenClockLeeway) }
    defer func() { jwt.TimeFunc = previous }()
    return p.sdk.ParseJwtToken(token)
}
```

**注意**：`jwt.TimeFunc` 是全局变量，修改时需要加锁保护，避免并发竞争。

### 10.3 GetMe 端点 AUTHZ_PERMISSION_DENIED（SpiceDB 权限拒绝）

**现象**：登录成功后，前端调用 `GET /v1/iam/me` 返回 403，错误码 `AUTHZ_PERMISSION_DENIED`，消息 `"spicedb permission did not matched"`。

**根因**：生成的 `IAMAuthServiceKernelAccessResolver` 为 `GetMe` 方法生成了 SpiceDB 权限检查规则：

```go
"/iam.v1.IAMAuthService/GetMe": {
    Action:   "read",
    Resource: "iam:self",    // 这个资源类型在 SpiceDB schema 中不存在
    Mode:     "SELF_CHECK",
}
```

SpiceDB schema 中 `iam` 类型只有 `admin` 关系，没有 `self` 资源类型。所以 SpiceDB 返回 permission denied。

**对比 Hub 实现**：Hub 的 `/v1/authn/me` 端点**没有 SpiceDB 权限检查**。Hub 的 authn middleware 只做 token 验证（`Authenticate`），不做资源级授权。`GetMe` 是获取当前用户自身信息的端点，只需要认证，不需要授权。

**推荐方案**：使用 `SkipPolicy` 配置驱动，在 `security.access.skip_operations` 中列出 `GetMe`：

```yaml
security:
  access:
    skip_operations:
      - GetMe
      - CreateOrganization
```

然后在 `iamAccessResolver` 中使用 `accessx.NewSkipPolicyResolver` 统一处理：

```go
func newIAMAccessResolver(security conf.SecurityConfig) mwaccess.Resolver {
    skipResolver := accessx.NewSkipPolicyResolver(accessx.AccessConfig{
        SkipOperations: security.Access.SkipOperations,
    })
    return func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
        if policy := skipResolver(operation); policy != accessx.SkipDefault {
            return accessx.Check{SkipPolicy: policy, AuditAction: resolveAuditAction(operation)}, true, nil
        }
        // ... 其他操作走正常 resolver 链
    }
}
```

**旧方案（Deprecated）**：直接在 resolver 中返回 `accessx.Check{}, false, nil` 跳过授权检查：

```go
func iamAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
    if isGetMeOperation(operation) {
        return accessx.Check{}, false, nil  // 跳过授权检查
    }
    // ... 其他操作走正常 resolver 链
}
```

**设计原则**：`AUTHENTICATED` 级别的端点（如 `GetMe`、`RefreshToken`）只需要认证，不需要资源级授权。`AUTHORIZED` 级别的端点才需要 SpiceDB 检查。新代码应使用 `SkipPolicy` 替代硬编码跳过逻辑。

### 10.4 生成代码 vs 手写代码的 Access Resolver 冲突

**现象**：`iamAccessResolver` 中使用了生成的 `IAMAuthServiceKernelAccessResolver`，但手写的 `IAMAuthServiceAccessResolver`（在 `iam_auth_resolver.go` 中）没有被使用。

**根因**：`internal/server/access.go` 中的 `iamAccessResolver` 函数按顺序尝试多个 resolver：

```go
resolvers := []mwaccess.Resolver{
    v1.IAMAuthServiceKernelAccessResolver,   // 生成的 resolver
    v1.IAMDirectoryServiceKernelAccessResolver,
    // ...
}
```

而手写的 `v1.IAMAuthServiceAccessResolver`（在 `api/iam/v1/iam_auth_resolver.go` 中）**没有被包含在这个列表中**。生成的 resolver 对 `GetMe` 使用 `SELF_CHECK` 模式，而手写的 resolver 使用 `CHECK_ONLY` 模式。

**解决方案**：确保 `iamAccessResolver` 中使用的 resolver 列表与期望的行为一致。如果手写 resolver 有特殊逻辑，需要将其加入列表，或者像 `GetMe` 那样在 `iamAccessResolver` 中提前处理。

### 10.5 配置文件加载顺序与环境变量

**现象**：IAM 启动时使用默认配置（`configs/config.yaml`），但该文件中的 `client_secret` 是 `${CASDOOR_CLIENT_SECRET}` 环境变量引用，而环境变量未设置，导致 Casdoor 连接失败。

**根因**：Kernel 的 `configx` 支持环境变量展开（`${VAR_NAME}` 语法）。`configs/config.yaml` 是默认配置，使用环境变量引用；`configs/config.local.yaml` 是本地开发配置，使用硬编码值。

**解决方案**：启动时必须指定本地配置：

```bash
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

**关键区别**：
- `configs/config.yaml`：`client_secret: "${CASDOOR_CLIENT_SECRET}"`，`application_name: "aisphere"`
- `configs/config.local.yaml`：`client_secret: "6d37fc7a95c21c45e543207704345b2ac80586d2"`，`application_name: "aisphere-iam"`

**注意**：两个配置文件的 `application_name` 也不同。`config.yaml` 使用 `aisphere`（Hub 的 app），`config.local.yaml` 使用 `aisphere-iam`（IAM 自己的 app）。Casdoor 中的 app 配置必须与之一致。

### 10.6 数据库连接凭据

**现象**：IAM 启动时数据库连接失败。

**根因**：PostgreSQL 连接字符串中的用户名、密码或数据库名不正确。

**正确凭据**：
- 用户：`postgres`（不是 `aisphere`）
- 密码：`ChangeMe_PostgreSQL_123`（不是 `ChangeMe_PostgreSQL_123root`）
- 数据库：`aisphere_iam`
- 连接字符串：`postgres://postgres:ChangeMe_PostgreSQL_123@36.137.200.194:30080/aisphere_iam?sslmode=disable`

**SpiceDB 凭据**：
- Preshared Key：`keykeykey`（不是 `aisphere`）
- 端点：`36.137.200.194:30084`

### 10.7 前端 `state` 参数传递

**现象**：IAM 前端回调页面调用 `exchangeCode(code, redirectUri)` 时没有传递 `state` 参数，但 IAM 后端期望 `state` 在请求体中。

**根因**：`aisphere-iam-front/src/lib/api/index.ts` 中的 `exchangeCode` 方法最初只接受两个参数（`code`, `redirectUri`），但后端 `ExchangeCode` handler 期望 `state` 字段。

**解决方案**：
1. 修改 `exchangeCode` 方法，接受可选的 `state` 参数（第三个参数，默认为 `''`）
2. 在请求体中包含 `state` 字段
3. 在回调页面中从 URL 查询参数提取 `state` 并传递给 `exchangeCode`

```typescript
// API 层
exchangeCode: async (code: string, redirectUri: string, state = '') => {
    const raw = await iamRequest<{...}>('/v1/iam/auth/exchange', {
        method: 'POST',
        body: JSON.stringify({ code, redirect_uri: redirectUri, state }),
    });
    // ...
}

// 回调页面
const state = queryParams.get('state') || '';
const tokens = await iamAuthApi.exchangeCode(code, redirectUri, state);
```

### 10.8 前端响应结构嵌套

**现象**：IAM 后端返回的 `ExchangeCode` 响应是嵌套结构 `{ tokens: {...}, principal: {...} }`，但前端期望扁平结构 `{ accessToken: "...", refreshToken: "..." }`。

**根因**：IAM 的 `ExchangeCode` handler 返回 `ExchangeCodeReply`，其中 `tokens` 字段是 `TokenSet` 类型（嵌套），而 Hub 的 `Exchange` handler 返回扁平结构。

**解决方案**：前端 `exchangeCode` 方法需要同时处理两种结构：

```typescript
const t = raw.tokens || raw;
return {
    accessToken: t.accessToken || t.access_token || '',
    refreshToken: t.refreshToken || t.refresh_token || '',
    // ...
};
```

### 10.9 gorilla/mux 路由模式

**现象**：`DELETE /v1/users/{username}` 路由返回 404。

**根因**：Kernel 的 HTTP server 底层使用 `gorilla/mux`。路由模式必须使用 `{username}` 语法（不是 `:username` 或手动路径解析），并且需要使用 `mux.Vars(r)` 提取路径变量。

**解决方案**：
```go
srv.HandleFunc("/v1/users/{username}", localUserHandler.DeleteUser)

// 在 handler 中：
vars := mux.Vars(r)
username := vars["username"]
```

### 10.10 本地用户管理 API（/v1/users）

**背景**：IAM 前端有一个"本地用户"（Local Users）标签页，用于管理 Hub 自有数据库中的用户（非 Casdoor 用户）。该功能调用 `GET /v1/users`、`POST /v1/users`、`DELETE /v1/users/{username}` 接口。

**实现**：这些端点不是通过 protobuf/gRPC 定义的，而是作为普通 HTTP handler 注册在 `internal/server/http.go` 中：

```go
localUserHandler := service.NewLocalUserHandler(resources.LocalUsers)
srv.HandleFunc("/v1/users", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        localUserHandler.ListUsers(w, r)
    case http.MethodPost:
        localUserHandler.SaveUser(w, r)
    default:
        writeJSON(w, http.StatusMethodNotAllowed, ...)
    }
})
srv.HandleFunc("/v1/users/{username}", localUserHandler.DeleteUser)
```

**数据模型**：使用 `iam_local_users` 表（通过 migration `000002_iam_local_users.sql` 创建），包含 `username`、`subject_id`、`display_name`、`email`、`roles_json`、`permissions_json`、`namespaces_json`、`password_hash` 等字段。

**密码存储**：使用 SHA-256 哈希（非生产级安全，仅用于本地开发）。

### 10.11 Hub vs IAM 登录路径对比

开发 IAM 登录功能时，最有效的方法是**对比 Hub 和 IAM 的登录路径实现差异**。以下是关键差异点：

| 环节 | Hub 实现 | IAM 实现 | 差异影响 |
|------|---------|---------|---------|
| **OAuth code exchange** | 使用 Kernel 的 `casdoor.Client.ExchangeCode`（内部调用 `exchangeOAuthToken`，设置 `RedirectURL`） | 使用 `wrapperProvider.ExchangeCode`（最初调用 `sdk.GetOAuthToken`，未设置 `RedirectURL`） | 导致 500 错误 |
| **/me 端点授权** | 无 SpiceDB 检查，只做 token 验证 | 生成的 `AccessResolver` 要求 SpiceDB 检查 `iam:self` | 导致 403 错误 |
| **响应结构** | 扁平结构 `{ accessToken, refreshToken, ... }` | 嵌套结构 `{ tokens: {...}, principal: {...} }` | 前端需要兼容处理 |
| **路由注册** | 使用 `gorilla/mux` 的 `HandleFunc` | 使用 Kernel 的 `RegisterIAMAuthServiceHTTPServer`（也是 gorilla/mux） | 路由模式一致 |
| **配置加载** | 使用 `config.yaml`（环境变量） | 使用 `config.local.yaml`（硬编码值） | 启动参数不同 |

**方法论**：当 IAM 的某个功能出现问题时，先看 Hub 的对应实现。如果 Hub 能正常工作而 IAM 不能，说明 IAM 的实现与 Hub 有差异。逐层对比（proto → service → data → provider）可以快速定位问题。

## 11. 验收清单

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
