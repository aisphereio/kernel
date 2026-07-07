# Envoy Gateway + Casdoor OIDC Authn 体系

本文定义 Kernel 侧对接 Envoy Gateway + Casdoor OIDC 的第一阶段协议、生成和运行时边界。

## 1. 阶段目标

第一阶段只实现 OIDC/JWT 入口认证，不接入 Gateway ExternalAuth。

```text
Casdoor = OIDC Provider
Envoy Gateway = OIDC 登录、JWT 验签、claimToHeaders、header sanitize、路由转发
Kernel = proto 注解、Gateway API 生成契约、服务端 middleware、内部调用 middleware
IAM = 作为后端业务服务/目录服务存在；不作为 Gateway ExternalAuth
```

该阶段的关键目标：

```text
1. Browser 访问受保护页面时由 Envoy Gateway 触发 Casdoor OIDC 登录。
2. CLI/Agent/API Client 直接携带 Bearer token，由 Envoy Gateway 通过 Casdoor JWKS 验签。
3. Gateway 将已验证 JWT 的有限 claim 转成 x-aisphere-external-* header。
4. Gateway 清理客户端伪造的 x-aisphere-* / x-internal-* header。
5. 后端 Kernel middleware 只读取 Gateway 生成的 external identity header。
6. 业务 authz 暂时仍在服务内通过 Kernel accessx / IAM client 完成，不通过 Gateway ExternalAuth。
```

> 后续阶段再引入 IAM ExternalAuth、x-aisphere-principal 注入、internal JWT 和 Gateway 侧资源级授权。

## 2. 安全分级

Kernel/proto 中的接口统一分为三类，但本阶段 Gateway 只区分是否需要 OIDC/JWT：

| 模式 | Gateway 认证 | Gateway 授权 | 后端要求 | 典型接口 |
|---|---:|---:|---|---|
| `AUTH_NONE` | 否 | 否 | 不要求 principal | `/healthz`、`/readyz`、`/openapi.json`、`/api/v1/public/*` |
| `AUTHN_ONLY` | 是 | 否 | 可读取 external identity | `/api/v1/me`、`/api/v1/profile` |
| `AUTHZ` | 是 | 否 | 后端 accessx/IAM client 做业务授权 | 业务资源接口 |

约束：

```text
1. 不把 Casdoor OIDC SecurityPolicy 挂到 Gateway 全局。
2. public route 不挂 OIDC/JWT SecurityPolicy。
3. authn/authz route 都挂 OIDC/JWT SecurityPolicy。
4. 所有 route 都必须执行 header sanitize。
5. 本阶段不生成 extAuth，不生成 headersToBackend，不生成 x-aisphere-internal-jwt。
```

## 3. Proto 注解契约

Kernel 当前 access policy 仍是路由和权限事实源。新体系要求 generator 能从 proto 方法得到以下语义：

```text
auth mode        : AUTH_NONE / AUTHN_ONLY / AUTHZ
resource_type    : hub.agent / hub.workflow / iam.project / git.repository
action           : read / create / update / delete / publish / clone
public_exposed   : 是否生成公网 HTTPRoute
internal_only    : 是否仅生成内部路由
service_call     : 是否允许 service principal 调用
```

示例：

```proto
rpc PublishAgent(PublishAgentRequest) returns (PublishAgentReply) {
  option (google.api.http) = {
    post: "/api/v1/agents/{agent_id}/publish"
    body: "*"
  };

  option (aisphere.access.v1.policy) = {
    mode: AUTHZ
    resource: "hub.agent"
    action: "publish"
    exposure: PUBLIC
  };
}
```

本阶段 `AUTHZ` 不代表 Gateway 会调用 IAM ExternalAuth，只代表 Gateway 必须完成 OIDC/JWT 认证，资源级授权仍由服务端 Kernel access chain 处理。

## 4. Gateway API 生成规则

每个服务生成三组 HTTPRoute：

```text
<service>-public-route     # AUTH_NONE
<service>-authn-route      # AUTHN_ONLY
<service>-protected-route  # AUTHZ
```

对应策略：

```text
public route     : no SecurityPolicy
authn route      : OIDC/JWT SecurityPolicy
protected route  : OIDC/JWT SecurityPolicy
```

禁止生成：

```text
extAuth
headersToBackend
x-aisphere-principal 注入
x-aisphere-internal-jwt 注入
```

## 5. Header 规范

### 5.1 Gateway 输出的外部身份 Header

由 Envoy Gateway `claimToHeaders` 从已验证 Casdoor JWT 中提取：

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
```

这些 header 表示 Casdoor external identity，不等价于 Aisphere internal principal。

### 5.2 本阶段不输出的内部 Header

本阶段 Gateway 不负责输出以下 header：

```text
x-aisphere-principal
x-aisphere-user-id
x-aisphere-org-id
x-aisphere-project-id
x-aisphere-roles
x-aisphere-authz-decision-id
x-aisphere-internal-jwt
```

这些 header 预留给后续 IAM ExternalAuth 阶段。本阶段如果业务需要 Aisphere user/org/project，应由后端 Kernel middleware 或业务服务通过 IAM client 根据 external identity 查询映射。

## 6. Header Sanitize

Kernel deploy generator 应生成或引用全局 `ClientTrafficPolicy`：

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: ClientTrafficPolicy
metadata:
  name: aisphere-sanitize-headers
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: aisphere-gateway
  headers:
    earlyRequestHeaders:
      remove:
        - x-aisphere-external-sub
        - x-aisphere-external-email
        - x-aisphere-external-name
        - x-aisphere-external-username
        - x-aisphere-principal
        - x-aisphere-user-id
        - x-aisphere-org-id
        - x-aisphere-project-id
        - x-aisphere-roles
        - x-aisphere-authz-decision-id
        - x-aisphere-internal-jwt
        - x-internal-jwt
```

规则：

```text
public/authn/authz 都必须 sanitize。
sanitize 在 JWT claimToHeaders 生成可信 x-aisphere-external-* 前执行。
业务服务不得信任客户端直接传入的 x-aisphere-*。
```

## 7. Casdoor OIDC/JWT 生成模板

Authn/protected route 生成 OIDC/JWT：

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: hub-authn-policy
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: hub-authn-route
  oidc:
    provider:
      issuer: "https://casdoor.aisphere.local"
    clientID: "aisphere-gateway"
    clientSecret:
      name: casdoor-gateway-client-secret
    redirectURL: "https://hub.aisphere.local/oauth2/callback"
    logoutPath: "/logout"
    scopes: ["openid", "profile", "email"]
    refreshToken: true
    forwardAccessToken: true
    passThroughAuthHeader: true
    disableTokenEncryption: false
  jwt:
    providers:
      - name: casdoor
        issuer: "https://casdoor.aisphere.local"
        audiences: ["aisphere-gateway"]
        remoteJWKS:
          uri: "https://casdoor.aisphere.local/.well-known/jwks"
        extractFrom:
          headers:
            - name: Authorization
              valuePrefix: "Bearer "
        claimToHeaders:
          - claim: sub
            header: x-aisphere-external-sub
          - claim: email
            header: x-aisphere-external-email
          - claim: name
            header: x-aisphere-external-name
          - claim: preferred_username
            header: x-aisphere-external-username
```

## 8. Kernel 服务端运行时

Kernel inbound middleware 在本阶段支持：

| 来源 | 校验方式 | Principal 来源 |
|---|---|---|
| public request | 不校验 | 无 |
| Gateway OIDC/JWT request | 信任 Gateway 输出的 `x-aisphere-external-*`；可选后端二次校验 forwarded access token | Casdoor external identity |
| Internal service request | `Authorization: Bearer <service-token>` 验签 | IAM service token |

Inbound 处理顺序：

```text
requestinfo
  -> metadata/header normalization
  -> external identity restore
  -> optional backend token verification
  -> accessx guard for AUTHZ methods
  -> auditx
  -> handler
```

注意：`AUTHZ` 方法仍然可以在后端执行 `accessx`。此时 accessx 的 Principal 可以先使用 external identity，再通过 IAM Directory/client 做内部 user 映射。

## 9. Kernel 客户端运行时

服务调用其他内部服务时，outbound middleware 自动注入：

```http
Authorization: Bearer <aisphere-service-token>
x-aisphere-call-type: service
x-aisphere-source-service: <service-name>
x-aisphere-request-id: <request-id>
```

service token 由 IAM 签发。该能力与 Gateway OIDC 无耦合。

## 10. 后续阶段

本阶段明确不做：

```text
IAM ExternalAuth
Gateway resource-level authz
x-aisphere-principal 注入
x-aisphere-internal-jwt 注入
headersToBackend
```

后续阶段再扩展为：

```text
Gateway OIDC/JWT
  -> IAM ExternalAuth
  -> IAM 返回 Aisphere internal principal
  -> Gateway 注入 x-aisphere-principal
  -> 后端可校验 x-aisphere-internal-jwt
```

## 11. 禁止事项

```text
1. 禁止 x-jwt-secret / x-auth-secret 这种共享 secret header。
2. 禁止把 Casdoor sub 直接作为 Aisphere user_id。
3. 禁止业务服务自己维护 public/authn/authz route list。
4. 禁止 public route 依赖 IAM 才能健康检查。
5. 禁止 Gateway 全局挂 OIDC 后再做路径例外。
```
