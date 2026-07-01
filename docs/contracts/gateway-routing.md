# Gateway 路由契约

Gateway 的路由不允许由业务代码手写维护。Kernel 的主线是：

```text
业务 proto
  -> google.api.http + aisphere.access.v1.policy
  -> 生成 Gateway Route Manifest
  -> Route Registry
  -> Gateway Matcher / Dispatcher
  -> K8s Service upstream
```

## 1. 职责边界

Gateway 负责边界路由和边界准入：

- 匹配外部 HTTP method/path。
- 判断 PUBLIC / AUTHENTICATED / AUTHORIZED / INTERNAL / SYSTEM。
- 对需要登录的外部路由做 passive authn：没有 Authorization 直接 401。
- 转发 Authorization/token 到下游服务。
- 做入口超时、限流、trace/request_id 透传。

Gateway 不负责业务资源级鉴权：

- 不判断用户是否能编辑 `skill:123`。
- 不判断用户是否能删除 `org:abc` 下的角色。
- 不查询业务数据库。
- 不替代 IAM/Authz 服务。

业务服务必须通过 Kernel `authn/authz/accessx` 做权威认证和资源授权。

## 2. Route Manifest

Route Manifest 是由生成器产出的路由契约。MVP 可手写等价结构验证，生产必须由 proto 生成。

```go
manifest := gatewayx.Manifest{
    Service:   "skill-service",
    Namespace: "aisphere",
    Routes: []gatewayx.GatewayRoute{{
        ID:     "skill.get",
        Method: "GET",
        Path:   "/v1/skills/{id}",
        Upstream: gatewayx.UpstreamRef{
            Service:   "skill-service",
            Namespace: "aisphere",
            Protocol:  "grpc",
            Operation: "/aisphere.skill.v1.SkillService/GetSkill",
        },
        Gateway: gatewayx.GatewayPolicy{
            Exposure:  AUTHORIZED,
            AuthnMode: gatewayx.AuthnModePassive,
        },
    }},
}
```

## 3. Gateway AuthnMode

| Mode | 含义 | 使用场景 |
|---|---|---|
| `none` | 不要求 Authorization | login、callback、healthz |
| `passive` | 只要求 Authorization 存在并透传 | 默认业务 API |
| `verify_jwt` | Gateway 本地验 JWT | 生产入口优化 |
| `introspect` | Gateway 调 IAM introspection | 强中心控制场景 |

默认规则：

```text
PUBLIC/SYSTEM        -> none
AUTHENTICATED        -> passive
AUTHORIZED           -> passive
INTERNAL             -> 外部不暴露
```

## 4. 服务发现与注册表

当前决策：

- 服务发现使用 Kubernetes Service / DNS / EndpointSlice。
- 路由注册表使用 etcd 集群。
- Kernel 不自研服务发现，不自研注册中心。

本地验证可用 `gatewayx.StaticHosts` 代替 K8s Service DNS：

```go
gatewayx.StaticHosts{
    "iam-service.aisphere":   "iam.local",
    "skill-service.aisphere": "skill.local",
}
```

生产中同一个 `UpstreamRef` 应解析为：

```text
skill-service.aisphere.svc.cluster.local:9000
```

## 5. 三段验证流程

未登录：

```text
Client -> Gateway /v1/skills/s1
Gateway: route=AUTHORIZED, missing Authorization
Gateway -> 401
SkillService 不执行
```

已登录但无权限：

```text
Client -> Gateway with token
Gateway passive relay
SkillService authn 通过
SkillService accessx/authz 拒绝
返回 403
业务 handler 不执行
```

已登录且有权限：

```text
Client -> Gateway with token
Gateway passive relay
SkillService authn 通过
SkillService accessx/authz 通过
业务 handler 执行
audit success
```

## 6. Agent 规则

- Gateway 路由不得手写硬编码在业务 handler 中。
- 新外部接口必须从 proto 的 `google.api.http` 和 `aisphere.access.v1.policy` 生成 Route Manifest。
- Gateway 只能消费 RouteRegistry，不直接 import 业务服务实现。
- 服务发现使用 K8s Service；本地验证可用 StaticHosts 替代。
- 如 Gateway 无法表达某类路由，先扩展 `gatewayx` 和生成器，不允许业务绕过。

## 7. 生成器和注册流程

Gateway 路由清单由 `protoc-gen-go-gateway` 从 proto 生成，不由业务手写。

生成链路：

```text
proto google.api.http + aisphere.access.v1.policy
  -> protoc-gen-go-gateway
  -> <Service>GatewayManifest()
  -> serverx.RegisterGatewayRoutes(...)
  -> RouteRegistry
  -> Gateway Matcher / Dispatcher
```

本地验证可以使用：

```go
kv := gatewayx.NewMemoryKVStore()
registry := gatewayx.NewEtcdRegistry(kv, "/aisphere/kernel/routes/dev")
_ = serverx.RegisterGatewayRoutes(ctx, registry,
    iamv1.IAMServiceGatewayManifest(),
    skillv1.SkillServiceGatewayManifest(),
)
```

生产中 `EtcdRegistry` 应接入真实 etcd store；本地和单测用 `MemoryKVStore` 模拟 etcd prefix/list/put 语义。

## 8. Gateway / SkillService 的 authn-authz 分工

以 `GET /v1/skills/s1` 为例：

```text
Client
  -> Gateway route match
  -> Gateway 发现 route=AUTHORIZED 且 authn_mode=passive
  -> 无 Authorization：Gateway 直接 401，SkillService 不执行
  -> 有 Authorization：Gateway token relay 转发到 SkillService
  -> SkillService authn middleware 调 Kernel authn.Provider 验 token
  -> SkillService access middleware 根据 generated AccessResolver 生成 action/resource
  -> SkillService authz provider 判断权限
  -> SkillService audit recorder 写入审计
  -> handler 返回 Skill
```

因此：

- Gateway 不做资源级授权。
- SkillService 是业务资源 authn/authz 的权威执行点。
- authn provider 默认由 Casdoor 实现。
- authz provider 默认由 SpiceDB 实现。
- validation 中使用 fake Casdoor-like / SpiceDB-like provider 验证框架链路。

## 9. Gateway 到 Service 的默认协议：generated gRPC invoker

Gateway 到内部服务的主路径不是手写 HTTP reverse proxy，而是生成式 gRPC 调用：

```text
外部 HTTP 请求
  -> Gateway route match
  -> generated bind: HTTP path/query/body -> gRPC request
  -> generated invoker registry: operation -> gRPC client method
  -> backend service gRPC server chain
```

`protoc-gen-go-gateway` 需要同时生成两类代码：

```go
func SkillServiceGatewayManifest() gatewayx.Manifest
func SkillServiceGatewayBindGetSkill(req gatewayx.DispatchRequest, match gatewayx.RouteMatch) (*GetSkillRequest, error)
func RegisterSkillServiceGatewayInvokers(registry *gatewayx.InvokerRegistry, client SkillServiceClient) error
```

Gateway 启动时注册 invoker：

```go
invokers := gatewayx.NewInvokerRegistry()
_ = skillv1.RegisterSkillServiceGatewayInvokers(invokers, skillv1.NewSkillServiceClient(conn))
gateway := gatewayx.NewDispatcher(routeRegistry, staticHosts, invokers)
```

本地验证可以用 `gatewayx.UnaryInvoker` 做 in-process 调用；生产使用 `gatewayx.GRPCUnaryInvoker` 调 generated gRPC client。

这保证 AI 开发业务时不需要写任何 Gateway 转发代码，只需要写 proto 和 handler。
