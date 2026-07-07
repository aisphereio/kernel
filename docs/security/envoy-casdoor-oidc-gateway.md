# Envoy Gateway + Casdoor OIDC Authn 体系

本文定义 Kernel 侧对接 Envoy Gateway + Casdoor OIDC 的协议、生成和运行时边界。

## 1. 目标

Aisphere 第一阶段采用：

```text
Casdoor = OIDC Provider
Envoy Gateway = 南北向入口安全执行点
Aisphere IAM = principal 映射、ExtAuth、internal JWT、资源级授权
Kernel = proto 注解、生成器契约、服务端/客户端 middleware
```

该阶段不要求 IAM 自己成为 OIDC Provider。Gateway 直接信任 Casdoor 的 issuer/JWKS；IAM 负责把 Casdoor external identity 映射为 Aisphere principal。

## 2. 安全分级

Kernel/proto 中的接口统一分为三类：

| 模式 | 认证 | 授权 | Gateway 生成行为 | 典型接口 |
|---|---:|---:|---|---|
| `AUTH_NONE` | 否 | 否 | 生成到 public route；不挂 SecurityPolicy | `/healthz`、`/readyz`、`/openapi.json`、`/api/v1/public/*` |
| `AUTHN_ONLY` | 是 | 否 | 生成到 authn route；挂 OIDC/JWT | `/api/v1/me`、`/api/v1/profile` |
| `AUTHZ` | 是 | 是 | 生成到 protected route；挂 OIDC/JWT + IAM ExtAuth | 业务资源接口 |

约束：

```text
1. 不允许把 Casdoor OIDC SecurityPolicy 挂到 Gateway 全局。
2. public route 不挂 OIDC/JWT/ExtAuth。
3. 所有 route 都必须执行 header sanitize。
4. AUTHZ route 必须 fail-closed 调 IAM ExtAuth。
```

## 3. Proto 注解契约

Kernel 当前 access policy 仍是路由和权限事实源。新体系要求 generator 能从 proto 方法上得到以下语义：

```text
auth mode        : AUTH_NONE / AUTHN_ONLY / AUTHZ
resource_type    : hub.agent / hub.workflow / iam.project / git.repository
action           : read / create / update / delete / publish / clone
public_exposed   : 是否生成公网 HTTPRoute
internal_only    : 是否仅生成内部路由
internal_jwt     : 后端是否必须校验 IAM internal JWT
service_call     : 是否允许 service principal 调用
```

推荐 proto option 语义示例：

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
    require_internal_jwt: true
  };
}
```

如果现有 `aisphere.access.v1.policy` 字段名不同，generator 必须做兼容映射，不应要求业务仓库重复维护 route list。

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
protected route  : OIDC/JWT + ExtAuth SecurityPolicy
```

生成器必须将鉴权元数据写入 HTTPRoute metadata，供 IAM ExtAuth 使用：

```yaml
metadata:
  annotations:
    aisphere.io/service: "hub"
    aisphere.io/auth-mode: "authz"
    aisphere.io/resource-type: "hub.agent"
    aisphere.io/action: "publish"
```

## 5. Header 规范

### 5.1 外部身份 Header

由 Envoy Gateway `claimToHeaders` 从已验证 Casdoor JWT 中提取：

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
```

这些 header 只表示 Casdoor external identity，不等价于 Aisphere principal。

### 5.2 内部身份 Header

由 IAM ExtAuth 返回，Gateway 注入给后端：

```text
x-aisphere-principal
x-aisphere-user-id
x-aisphere-org-id
x-aisphere-project-id
x-aisphere-roles
x-aisphere-authz-decision-id
x-aisphere-internal-jwt
```

这些 header 才能被业务服务读取。客户端直接传入的同名 header 必须被清理。

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
Gateway sanitize 在 OIDC/JWT/ExtAuth 注入可信 header 前执行。
业务服务不得信任客户端传入的 x-aisphere-*。
```

## 7. Casdoor OIDC/JWT 生成模板

Authn route 生成 OIDC/JWT：

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
```

Protected route 在此基础上增加 `extAuth`。

## 8. IAM ExtAuth 生成模板

```yaml
extAuth:
  http:
    backendRefs:
      - name: aisphere-iam
        port: 8080
    pathOverride: /v1/extauth/check
    headersToBackend:
      - x-aisphere-principal
      - x-aisphere-user-id
      - x-aisphere-org-id
      - x-aisphere-project-id
      - x-aisphere-roles
      - x-aisphere-authz-decision-id
      - x-aisphere-internal-jwt
  headersToExtAuth:
    - Authorization
    - Cookie
    - X-Request-Id
    - X-Forwarded-For
    - X-Forwarded-Proto
    - X-Forwarded-Host
    - x-aisphere-external-sub
    - x-aisphere-external-email
  failOpen: false
  timeout: 2s
```

## 9. Kernel 服务端运行时

Kernel inbound middleware 必须支持两种来源：

| 来源 | 校验方式 | Principal 来源 |
|---|---|---|
| Gateway request | `x-aisphere-internal-jwt` 可选验签；读取 Gateway/IAM 注入 header | IAM ExtAuth |
| Internal service request | `Authorization: Bearer <service-token>` 验签 | IAM service token |

Inbound 处理顺序：

```text
requestinfo
  -> metadata/header normalization
  -> principal restore
  -> optional internal JWT verification
  -> accessx guard
  -> auditx
  -> handler
```

## 10. Kernel 客户端运行时

服务调用其他内部服务时，outbound middleware 自动注入：

```http
Authorization: Bearer <aisphere-service-token>
x-aisphere-call-type: service
x-aisphere-source-service: <service-name>
x-aisphere-request-id: <request-id>
```

service token 由 IAM 签发，claim 推荐：

```json
{
  "iss": "https://iam.aisphere.local",
  "aud": "aisphere-internal-services",
  "sub": "service:hub",
  "typ": "service_token",
  "service": "hub",
  "namespace": "aisphere-system",
  "scopes": ["iam.project.read", "runtime.invoke"],
  "exp": 1710000600
}
```

## 11. 内部服务互调分阶段

第一阶段：

```text
Kernel service token + NetworkPolicy + optional Gateway->Backend mTLS
```

第二阶段：

```text
Istio/Ambient Mesh mTLS + AuthorizationPolicy + IAM internal JWT
```

Kernel 侧不直接依赖 Istio；只要求 Principal、service token 和 authorization 语义保持 provider-neutral。

## 12. 禁止事项

```text
1. 禁止 x-jwt-secret / x-auth-secret 这种共享 secret header。
2. 禁止把 Casdoor sub 直接作为 Aisphere user_id。
3. 禁止业务服务自己维护 public/authn/authz route list。
4. 禁止 public route 依赖 IAM ExtAuth 才能健康检查。
5. 禁止 Gateway 全局挂 OIDC 后再做路径例外。
```
