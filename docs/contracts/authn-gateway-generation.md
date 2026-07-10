# Envoy Gateway OIDC Authn 生成契约

本文是 `cmd/protoc-gen-go-gateway` / deploy generator 对 Envoy Gateway OIDC 认证的生成约束。

## 1. 输入事实源

输入只来自 proto contract：

```text
google.api.http
aisphere.access.v1.policy
service/contract metadata
```

禁止业务服务在 `config.yaml` 中重复维护：

```text
public route list
authn route list
authz route list
Envoy Gateway SecurityPolicy list
```

## 2. Auth Mode 映射

| policy mode | HTTPRoute | SecurityPolicy | 后端要求 |
|---|---|---|---|
| public / anonymous / AUTH_NONE | `<service>-public-route` | 无 | 不要求 principal |
| authenticated / AUTHN_ONLY | `<service>-authn-route` | OIDC/JWT | 可读取 `x-aisphere-external-*` |
| authorized / AUTHZ | `<service>-protected-route` | OIDC/JWT | 后端 `accessx` / IAM client 做业务 authz |
| internal | `<service>-internal-route` 或不生成公网 route | 无公网 OIDC；内部 service token | 必须校验 service token |

`AUTHZ` 只表示该接口需要已认证身份，资源级授权仍在后端执行。

## 3. 生成资源

每个服务生成：

```text
<service>-public-route.yaml
<service>-authn-route.yaml
<service>-protected-route.yaml
<service>-authn-security-policy.yaml
<service>-protected-security-policy.yaml
```

平台级生成或引用：

```text
aisphere-sanitize-headers ClientTrafficPolicy
BackendTLSPolicy / Backend mTLS template
Casdoor client Secret reference
```

## 4. SecurityPolicy 绑定原则

```text
1. 不生成 Envoy Gateway 全局 OIDC SecurityPolicy。
2. OIDC/JWT 只绑定 authn/protected route。
3. public route 不绑定 SecurityPolicy。
4. 每个应用使用独立的 SecurityPolicy（不同 clientID、不同 redirectURL）。
```

## 5. Route Metadata

Protected route 仍应带 metadata，供后端 `requestx/accessx` 复用：

```yaml
metadata:
  annotations:
    aisphere.io/service: "hub"
    aisphere.io/auth-mode: "authz"
    aisphere.io/resource-type: "hub.agent"
    aisphere.io/action: "publish"
```

## 6. Header Sanitize

所有外部 Envoy Gateway listener 都必须关联 sanitize policy：

```text
remove x-aisphere-*
remove x-internal-jwt
remove x-user-id / x-org-id / x-project-id
```

该策略不能被 public route 跳过。

## 7. claimToHeaders 映射规则

生成器只允许输出 external identity：

```text
sub   -> x-aisphere-external-sub
email -> x-aisphere-external-email
name  -> x-aisphere-external-name
preferred_username -> x-aisphere-external-username
iss   -> x-aisphere-external-issuer
owner -> x-aisphere-external-owner
id    -> x-aisphere-external-id
```

## 8. OIDC/JWT 生成规则

Authn/protected route 生成：

```yaml
oidc:
  provider:
    issuer: "https://casdoor.weagent.cc"
  clientID: "YOUR_APP_CLIENT_ID"
  clientSecret:
    name: casdoor-app-oidc
  redirectURL: "https://YOUR_DOMAIN/oauth2/callback"
  logoutPath: "/logout"
  scopes: ["openid", "profile", "email"]
  refreshToken: true
  forwardAccessToken: true
  passThroughAuthHeader: true
jwt:
  providers:
    - name: casdoor
      issuer: "https://casdoor.weagent.cc"
      audiences: ["YOUR_APP_CLIENT_ID"]
      remoteJWKS:
        uri: "https://casdoor.weagent.cc/.well-known/jwks"
      claimToHeaders:
        - claim: sub
          header: x-aisphere-external-sub
        - claim: email
          header: x-aisphere-external-email
```

## 9. 后端处理要求

后端 Kernel middleware 自动完成：

```text
1. 从 x-aisphere-external-* 恢复 external identity。
2. AUTHN_ONLY 接口只要求 external identity 存在。
3. AUTHZ 接口由 accessx 使用 external identity 或 IAM Directory 映射后的 Principal 做业务授权。
4. 内部调用走 service token，不依赖 OIDC cookie。
```

## 10. 验收

生成产物必须满足：

```text
/healthz 不跳 Casdoor
/api/v1/me 未登录跳 Casdoor 或返回 401
/api/v1/agents 未登录拒绝或跳 Casdoor
/api/v1/agents 有 token 时 JWT 验签并输出 x-aisphere-external-*
伪造 x-aisphere-principal 会被清除
```
