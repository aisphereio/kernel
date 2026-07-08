# Envoy Gateway + Casdoor OIDC Authn 体系

本文定义 Kernel 侧对接 Envoy Gateway + Casdoor OIDC 的入口认证、header 投影和业务读取规则。

## 1. 阶段目标

当前阶段不要求 Gateway ExternalAuth。认证由 Envoy Gateway 基于 Casdoor OIDC/JWT 完成，资源级授权仍由后端 Kernel `accessx` / IAM client / SpiceDB 完成。

```text
Casdoor = OIDC Provider
Envoy Gateway = OIDC/JWT 验签、claimToHeaders、header sanitize、路由转发
Kernel = header trusted extraction、Principal 自动注入、accessx/authz/audit 编排
IAM = 后端目录/授权服务；当前不作为 Gateway ExternalAuth 必选组件
```

关键目标：

```text
1. Browser 访问受保护页面时由 Envoy Gateway 触发 Casdoor OIDC 登录。
2. CLI/Agent/API Client 直接携带 Bearer token，由 Envoy Gateway 通过 Casdoor JWKS 验签。
3. Gateway 将已验证 JWT 的有限 claim 转成 x-aisphere-external-* header。
4. Gateway 可以同时投影 x-aisphere-principal / x-aisphere-user-id / x-aisphere-org-id 等内部读取 header。
5. Gateway 必须清理客户端伪造的 x-aisphere-* / x-internal-* header。
6. 后端 Kernel middleware 自动从可信 header 重建 authn.Principal 并注入 ctx。
7. AUTHZ 方法仍由服务端 Kernel access chain 做资源级授权。
```

## 2. 安全分级

Kernel/proto 中的接口统一分为三类：

| 模式 | Gateway 认证 | Gateway 授权 | 后端要求 | 典型接口 |
|---|---:|---:|---|---|
| `AUTH_NONE` | 否 | 否 | 不要求 principal | `/healthz`、`/readyz`、`/openapi.json`、`/api/v1/public/*` |
| `AUTHN_ONLY` | 是 | 否 | 可读取 Principal | `/api/v1/me`、`/api/v1/profile` |
| `AUTHZ` | 是 | 否 | 后端 `accessx` / IAM client / SpiceDB 做业务授权 | 业务资源接口 |

约束：

```text
1. 不把 Casdoor OIDC SecurityPolicy 挂到 Gateway 全局。
2. public route 不挂 OIDC/JWT SecurityPolicy。
3. authn/authz route 都挂 OIDC/JWT SecurityPolicy。
4. 所有 route 都必须执行 header sanitize。
5. 当前阶段不要求 Gateway ExternalAuth。
```

## 3. Header 规范

### 3.1 Casdoor external claim headers

由 Envoy Gateway `claimToHeaders` 从已验证 Casdoor JWT 中提取：

```yaml
claimToHeaders:
  - claim: sub
    header: x-aisphere-external-sub
  - claim: iss
    header: x-aisphere-external-issuer
  - claim: email
    header: x-aisphere-external-email
  - claim: name
    header: x-aisphere-external-name
  - claim: displayName
    header: x-aisphere-external-display-name
  - claim: phone
    header: x-aisphere-external-phone
  - claim: owner
    header: x-aisphere-external-owner
  - claim: id
    header: x-aisphere-external-id
  - claim: scope
    header: x-aisphere-external-scope
```

说明：

```text
sub/id      -> Principal.ExternalID / SubjectID 候选
owner       -> Principal.OrgID / TenantID 候选
name        -> Casdoor username；没有 preferred_username 时也可作为 Username
DisplayName -> Principal.Name
scope       -> 支持空格、逗号或分号分隔
```

### 3.2 Kernel 内部投影 headers

Gateway 可以直接把已验证 claim 投影成业务更容易读取的内部 header：

```yaml
claimToHeaders:
  - claim: sub
    header: x-aisphere-principal
  - claim: id
    header: x-aisphere-user-id
  - claim: owner
    header: x-aisphere-org-id
```

Kernel 读取优先级：

```text
SubjectID = firstNonEmpty(x-aisphere-principal, x-aisphere-user-id, x-aisphere-external-id, x-aisphere-external-sub)
OrgID     = firstNonEmpty(x-aisphere-org-id, x-aisphere-external-owner, x-aisphere-owner)
Username  = firstNonEmpty(x-aisphere-external-username, x-aisphere-external-name)
Name      = firstNonEmpty(x-aisphere-external-display-name, x-aisphere-external-name, x-aisphere-external-username)
Phone     = x-aisphere-external-phone
Email     = x-aisphere-external-email
Scopes    = x-aisphere-scopes or x-aisphere-external-scope
```

### 3.3 Header sanitize

所有来自客户端的 Gateway-controlled header 必须先清理，再由 Gateway 注入可信值：

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
        - x-aisphere-auth-verified
        - x-aisphere-subject
        - x-aisphere-subject-type
        - x-aisphere-provider
        - x-aisphere-external-id
        - x-aisphere-external-sub
        - x-aisphere-external-issuer
        - x-aisphere-external-email
        - x-aisphere-external-name
        - x-aisphere-external-display-name
        - x-aisphere-external-username
        - x-aisphere-external-phone
        - x-aisphere-external-owner
        - x-aisphere-external-scope
        - x-aisphere-owner
        - x-aisphere-org-id
        - x-aisphere-app-id
        - x-aisphere-project-id
        - x-aisphere-principal
        - x-aisphere-user-id
        - x-aisphere-username
        - x-aisphere-name
        - x-aisphere-email
        - x-aisphere-phone
        - x-aisphere-groups
        - x-aisphere-roles
        - x-aisphere-scopes
        - x-aisphere-authz-decision-id
        - x-aisphere-internal-jwt
        - x-internal-jwt
```

业务服务不得信任客户端直接传入的 `x-aisphere-*`。

## 4. Kernel 服务端运行时

Inbound 处理顺序：

```text
requestinfo
  -> metadata/header normalization
  -> authn middleware GatewayTrustedExtractor
  -> TrustedHeaderAuthenticator / token authenticator
  -> authn.ContextWithPrincipal(ctx, p)
  -> contextx.WithAuthnPrincipal(ctx, p)
  -> accessx guard for AUTHZ methods
  -> auditx
  -> handler
```

业务 handler 不需要、也不应该自己解析 Gateway header。

推荐写法：

```go
p, ok := authn.PrincipalFromContext(ctx)
if !ok {
    return nil, authn.ErrUnauthenticated("")
}

userID := p.SubjectID // 稳定用户 UUID
orgID := p.OrgID      // Casdoor owner / Aisphere org projection
```

兼容写法：

```go
p := contextx.PrincipalFromContext(ctx)
if p == nil || !p.IsAuthenticated() {
    return nil, authn.ErrUnauthenticated("")
}
```

`contextx.Principal` 是 `authn.Principal` 的无损镜像，但业务 authn/authz 主线仍以 `authn.PrincipalFromContext(ctx)` 为准。

## 5. 内部服务调用

服务调用其他内部服务时，outbound middleware 应自动注入 service token 或 internal JWT，并继续透传 request-id / trace-id：

```http
Authorization: Bearer <aisphere-service-token>
x-aisphere-call-type: service
x-aisphere-source-service: <service-name>
x-aisphere-request-id: <request-id>
```

内部链路建议叠加 mTLS / NetworkPolicy，避免服务绕过 Gateway 或伪造 trusted headers。

## 6. 禁止事项

```text
1. 禁止 x-jwt-secret / x-auth-secret 这种共享 secret header。
2. 禁止业务服务自己解析或信任客户端直接传入的 x-aisphere-*。
3. 禁止业务服务自己维护 public/authn/authz route list。
4. 禁止 public route 依赖 IAM 才能健康检查。
5. 禁止 Gateway 全局挂 OIDC 后再做路径例外。
```
