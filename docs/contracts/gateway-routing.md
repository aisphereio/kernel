# Envoy Gateway 路由契约

Envoy Gateway 的路由不允许由业务代码手写维护。Kernel 的主线是：

```text
业务 proto
  -> google.api.http + aisphere.access.v1.policy
  -> 生成 Envoy Gateway HTTPRoute Manifest
  -> Route Registry
  -> Envoy Gateway Matcher
  -> K8s Service upstream
```

## 1. 职责边界

Envoy Gateway 负责边界路由和边界准入：

- 匹配外部 HTTP method/path。
- 通过 SecurityPolicy 做 OIDC 登录 / JWT 验证。
- 通过 ClientTrafficPolicy 清理伪造 header。
- 通过 claimToHeaders 注入可信身份 headers。
- 做入口超时、限流、trace/request_id 透传。

Envoy Gateway 不负责业务资源级鉴权：

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

## 3. 三段验证流程

未登录：

```text
Client -> Envoy Gateway /v1/skills/s1
Envoy Gateway: route=AUTHORIZED, missing session/token
Envoy Gateway -> 302 重定向到 Casdoor 或 401
SkillService 不执行
```

已登录但无权限：

```text
Client -> Envoy Gateway with token
Envoy Gateway OIDC/JWT 验证通过
Envoy Gateway 注入可信 headers 并转发
SkillService authn 通过
SkillService accessx/authz 拒绝
返回 403
```

已登录且有权限：

```text
Client -> Envoy Gateway with token
Envoy Gateway OIDC/JWT 验证通过
Envoy Gateway 注入可信 headers 并转发
SkillService authn 通过
SkillService accessx/authz 通过
Skill handler 执行
audit success
```

## 4. 生成器和注册流程

Envoy Gateway 路由清单由 `protoc-gen-go-gateway` 从 proto 生成，不由业务手写。

生成链路：

```text
proto google.api.http + aisphere.access.v1.policy
  -> protoc-gen-go-gateway
  -> <Service>GatewayManifest()
  -> serverx.RegisterGatewayRoutes(...)
  -> RouteRegistry
  -> Envoy Gateway HTTPRoute
```

## 5. Envoy Gateway / SkillService 的 authn-authz 分工

以 `GET /v1/skills/s1` 为例：

```text
Client
  -> Envoy Gateway route match
  -> Envoy Gateway 发现 route=AUTHORIZED
  -> 无 Authorization：Envoy Gateway 直接 401 或重定向到 Casdoor
  -> 有 Authorization：Envoy Gateway JWKS 验签
  -> Envoy Gateway claimToHeaders 注入可信 headers
  -> Envoy Gateway 转发到 SkillService
  -> SkillService authn middleware 从 headers 恢复 Principal
  -> SkillService access middleware 根据 generated AccessResolver 生成 action/resource
  -> SkillService authz provider 判断权限
  -> SkillService audit recorder 写入审计
  -> handler 返回 Skill
```

因此：

- Envoy Gateway 不做资源级授权。
- SkillService 是业务资源 authn/authz 的权威执行点。
- authn provider 默认由 Casdoor 实现。
- authz provider 默认由 SpiceDB 实现。