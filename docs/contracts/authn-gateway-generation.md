# Gateway Authn 生成契约

本文是 `cmd/protoc-gen-go-gateway` / deploy generator 对最新 Authn 体系的生成约束。

## 1. 输入事实源

输入只来自 proto contract：

```text
google.api.http
aisphere.access.v1.policy
service/package metadata
```

禁止业务服务在 `config.yaml` 中重复维护：

```text
public route list
authn route list
authz route list
Gateway SecurityPolicy list
```

## 2. Auth Mode 映射

| policy mode | HTTPRoute | SecurityPolicy | 后端要求 |
|---|---|---|---|
| public / anonymous / AUTH_NONE | `<service>-public-route` | 无 | 不要求 principal |
| authenticated / AUTHN_ONLY | `<service>-authn-route` | OIDC/JWT | 可读取 external claim 或 principal |
| authorized / AUTHZ | `<service>-protected-route` | OIDC/JWT + ExtAuth | 必须有 IAM principal |
| internal | `<service>-internal-route` 或不生成公网 route | service token / internal JWT / mTLS | 必须校验 service token |

具体字段名以现有 `aisphere.access.v1.policy` 为准，生成器要做兼容映射。

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
IAM ExtAuth backend reference
```

## 4. SecurityPolicy 绑定原则

```text
1. 不生成 Gateway 级 OIDC SecurityPolicy。
2. OIDC/JWT 只绑定 authn/protected route。
3. ExtAuth 只绑定 protected route。
4. public route 不绑定 SecurityPolicy。
5. route rule 级 sectionName 可选，用于同一 HTTPRoute 内更细粒度绑定。
```

## 5. Route Metadata

Protected route 必须带 metadata，供 IAM ExtAuth 识别业务语义：

```yaml
metadata:
  annotations:
    aisphere.io/service: "hub"
    aisphere.io/auth-mode: "authz"
    aisphere.io/resource-type: "hub.agent"
    aisphere.io/action: "publish"
    aisphere.io/internal-jwt: "required"
```

## 6. Header Sanitize

所有外部 Gateway listener 都必须关联 sanitize policy：

```text
remove x-aisphere-*
remove x-internal-jwt
remove x-user-id / x-org-id / x-project-id
```

该策略不能被 public route 跳过。

## 7. claimToHeaders 生成规则

生成器只允许输出 external identity：

```text
sub   -> x-aisphere-external-sub
email -> x-aisphere-external-email
name  -> x-aisphere-external-name
preferred_username -> x-aisphere-external-username
```

禁止生成：

```text
sub -> x-aisphere-user-id
email -> x-aisphere-principal
roles -> x-aisphere-roles
```

内部 principal 必须由 IAM ExtAuth 返回。

## 8. ExtAuth 生成规则

Protected route 必须生成：

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

## 9. 验收

生成产物必须满足：

```text
/healthz 不跳 Casdoor
/api/v1/me 未登录跳 Casdoor 或返回 401
/api/v1/agents 未登录拒绝
/api/v1/agents 有 token 时先 JWT 验签，再 ExtAuth
伪造 x-aisphere-principal 会被清除
ExtAuth 失败时 protected route 不转发给后端
```
