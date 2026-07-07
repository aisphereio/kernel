# Gateway API ExternalAuth + Principal JWT 设计

本文定义 Aisphere 平台外部流量进入后，如何通过 Gateway API / Envoy Gateway ExternalAuth 调用 IAM 认证，并将已验证身份以 Principal JWT 形式传递给后端微服务。

## 1. 结论

推荐主线：

```text
Client
  -> Gateway API HTTPRoute
  -> Envoy Gateway SecurityPolicy ExternalAuth
  -> IAM ExternalAuthorize validates original Authorization bearer token
  -> IAM returns X-Aisphere-Principal-JWT
  -> Envoy strips any client-supplied identity headers and forwards only IAM-returned Principal JWT
  -> backend microservice uses kernel security.authn.mode=principal_jwt
  -> business handler reads authn.Principal from context
```

不要把 Principal JWT 当成新的长期登录 token。外部登录态事实源仍然是 Casdoor/OIDC 原生 access token。Principal JWT 只用于 Gateway -> backend 的可信身份传递。

## 2. 职责边界

| 层 | 职责 | 不负责 |
|---|---|---|
| Client | 持有原生 OIDC/Casdoor access token | 伪造平台身份头 |
| Gateway API / HTTPRoute | 路由外部请求到后端服务 | 自己实现用户体系 |
| Envoy Gateway SecurityPolicy ExternalAuth | 请求准入前调用 IAM；转发 IAM 返回的允许头 | 维护用户状态 |
| IAM ExternalAuthorize | 校验原生 token、用户状态、session 状态；签发 Principal JWT | 长期替代 IdP token |
| Kernel `principal_jwt` authn | 后端服务自动验签 Principal JWT 并恢复 Principal | 查询用户是否被封禁 |
| accessx / authz | 判断当前 Principal 是否能操作资源 | 认证外部 token |
| runx / context lifecycle | 控制请求运行中取消、deadline、流式中断 | 认证和授权 |

## 3. 为什么不用明文 X-Aisphere-* 作为主线

旧的 `gateway_trusted` 明文头模型可用，但边界更脆弱：

```text
X-Aisphere-Subject
X-Aisphere-Org-ID
X-Aisphere-Roles
X-Aisphere-Internal-Token
```

这些 header 如果从公网进入时没有被彻底清理，就有被伪造风险。`X-Aisphere-Internal-Token` 能证明请求可能来自 Gateway，但不能证明每个身份字段没有被中间层错误覆盖。

Principal JWT 的优势是：

```text
1. 身份上下文整体签名，后端可以独立验签。
2. 后端服务不需要配置 Casdoor/OIDC/JWKS。
3. 业务代码不需要解析 token，只从 context 取 Principal。
4. Envoy 只需要转发一个 allow-listed header。
5. 后续可从 HS256 shared secret 演进到 RS256/JWKS，不改变业务代码。
```

## 4. 为什么不用 Principal JWT 控制流式请求生命周期

Principal JWT 只表示“请求进入时，IAM 已经认证过该身份”。它不适合表达运行中的取消。

例如模型流式输出可能持续 5 分钟，如果 Principal JWT TTL 设置为 30 秒，会误伤长流式请求；如果 TTL 设置为 5 分钟，又无法在用户被运营禁用后立即中断。

所以必须拆分：

```text
AuthN / Principal JWT：请求开始时你是谁。
runx / context lifecycle：请求运行中要不要继续。
```

用户主动取消、浏览器断开、运营封禁、session revoke、quota exhausted 等运行时事件，必须通过 `context.Context`、run registry、cancel event 处理，不能靠 token TTL 处理。

## 5. Token 语义

### 5.1 外部 token

外部请求必须带原生 IdP token：

```http
Authorization: Bearer <casdoor_or_oidc_access_token>
```

IAM 是唯一直接理解外部 token 的平台组件。普通后端微服务不直接处理 Casdoor/OIDC token。

### 5.2 Principal JWT

IAM ExternalAuth 成功后返回：

```http
X-Aisphere-Principal-JWT: <signed-principal-assertion>
```

该 token 是内部身份断言，默认：

```yaml
security:
  authn:
    enabled: true
    mode: principal_jwt
    principal_jwt:
      header: X-Aisphere-Principal-JWT
      secret: CHANGE_ME_PRINCIPAL_JWT_SECRET
      issuer: aisphere-iam
      audience: [aisphere-internal]
      ttl_ns: 300000000000
      clock_skew_ns: 30000000000
```

业务服务不写 token 解析代码。Kernel middleware 完成：

```text
read X-Aisphere-Principal-JWT
verify signature / issuer / audience / exp / nbf
restore authn.Principal
inject Principal into context
```

业务代码只使用：

```go
principal, ok := authn.PrincipalFromContext(ctx)
```

## 6. Principal JWT claims

建议最小 claims：

```json
{
  "iss": "aisphere-iam",
  "aud": ["aisphere-internal"],
  "typ": "aisphere-principal",
  "sub": "user-id",
  "subject_type": "user",
  "provider": "casdoor",
  "external_id": "casdoor-user-id",
  "tenant_id": "tenant-id",
  "org_id": "org-id",
  "app_id": "app-id",
  "project_id": "project-id",
  "username": "alice",
  "email": "alice@example.com",
  "roles": ["viewer"],
  "groups": ["group-a"],
  "scopes": ["openid", "profile"],
  "iat": 123,
  "nbf": 123,
  "exp": 123
}
```

注意：

```text
roles / groups / scopes 只能作为身份上下文或粗粒度提示。
最终资源权限必须继续走 accessx/authz/SpiceDB。
不要把完整资源权限列表放入 Principal JWT。
```

## 7. Gateway / Envoy 必须做的事

### 7.1 请求进入时清理外部伪造头

Gateway 必须在调用 IAM 和转发 backend 前清理：

```text
X-Aisphere-Principal-JWT
X-Aisphere-Auth-Verified
X-Aisphere-Subject
X-Aisphere-Subject-Type
X-Aisphere-Provider
X-Aisphere-External-ID
X-Aisphere-Issuer
X-Aisphere-Audience
X-Aisphere-Owner
X-Aisphere-Org-ID
X-Aisphere-App-ID
X-Aisphere-Project-ID
X-Aisphere-Username
X-Aisphere-Name
X-Aisphere-Email
X-Aisphere-Groups
X-Aisphere-Roles
X-Aisphere-Scopes
X-Aisphere-Internal-Token
```

公网 client 永远不能直接注入这些 header。

### 7.2 ExternalAuth 只转发 IAM 返回的 Principal JWT

Envoy Gateway SecurityPolicy 的 ExternalAuth 配置中，`headersToBackend` 只允许：

```yaml
headersToBackend:
  - X-Aisphere-Principal-JWT
```

不要把所有 `X-Aisphere-*` 自动透传给后端。

### 7.3 HTTPRoute 示例

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: hub-api
  namespace: aisphere
spec:
  parentRefs:
    - name: public-gateway
      namespace: aisphere-system
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /v1/hub
      backendRefs:
        - name: aisphere-hub
          port: 18080
```

### 7.4 SecurityPolicy 示例

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: hub-api-ext-auth
  namespace: aisphere
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: hub-api
  extAuth:
    http:
      backendRefs:
        - name: aisphere-iam
          port: 18080
      path: /internal/iam/ext-authz
      headersToBackend:
        - X-Aisphere-Principal-JWT
```

具体字段以当前 Envoy Gateway CRD 版本为准。生成器输出前必须以集群安装的 CRD schema 校验。

## 8. IAM ExternalAuthorize 语义

IAM ExternalAuthorize 是 Gateway ExternalAuth 入口，不作为普通业务 API 对外发布。

推荐 proto：

```proto
rpc ExternalAuthorize(google.protobuf.Empty) returns (Principal) {
  option (google.api.http) = {
    post: "/internal/iam/ext-authz"
    body: "*"
  };
  option (aisphere.access.v1.policy) = {
    exposure: PUBLIC
    gateway: { publish: DISABLED }
    audit: { enabled: true event: "iam.external_authorize" risk: "medium" }
    reason: "Gateway ExternalAuth endpoint. IAM validates external bearer token and returns a signed Principal assertion."
  };
}
```

处理逻辑：

```text
1. 从 Authorization header 读取原生 bearer token。
2. 校验 token signature / issuer / audience / exp。
3. 查询用户是否 disabled / blocked。
4. 查询 session / token 是否 revoked。
5. 校验 org/project/app 上下文是否合法。
6. 构造 authn.Principal。
7. 调 authn.SignPrincipalJWT(principal, cfg)。
8. 在 response header 写入 X-Aisphere-Principal-JWT。
9. 返回 200；失败返回 401/403。
```

## 9. 后端微服务配置

普通后端服务推荐：

```yaml
security:
  authn:
    enabled: true
    mode: principal_jwt
    provider: iam
    principal_jwt:
      header: X-Aisphere-Principal-JWT
      secret: CHANGE_ME_PRINCIPAL_JWT_SECRET
      issuer: aisphere-iam
      audience: [aisphere-internal]
      ttl_ns: 300000000000
      clock_skew_ns: 30000000000
  authz:
    enabled: true
    provider: spicedb
```

高安全服务可以额外启用二次 active check，但不是普通业务默认心智。

## 10. 服务间调用

服务间调用必须传播：

```text
X-Aisphere-Principal-JWT
X-Aisphere-Request-ID
X-Aisphere-Run-ID
X-Aisphere-Parent-Run-ID
X-Aisphere-Deadline
Traceparent
Tracestate
```

身份仍然由 Principal JWT 表达；运行时取消、deadline、长流式中断由 `context.Context` / run lifecycle 表达。

禁止下游调用中途换成：

```go
context.Background()
context.TODO()
```

除非明确创建独立后台任务。

## 11. 与 run lifecycle 的关系

本链路只解决请求入口身份和后端身份恢复，不解决运行中取消。

运行中取消必须由后续 `runx` 设计处理：

```text
Client disconnect / user cancel / admin disable / session revoke
  -> cancel event
  -> active run registry
  -> context cancel
  -> DB/RPC/model stream observe ctx.Done()
```

因此：

```text
Principal JWT is not a cancellation mechanism.
Context is the cancellation mechanism.
```

## 12. 实现状态

当前状态：

```text
kernel:
  - securityx supports authn.mode=principal_jwt
  - authn has PrincipalJWTConfig / SignPrincipalJWT / PrincipalJWTAuthenticator

aisphere-iam:
  - ExternalAuthorize proto-first endpoint exists
  - previous merged implementation returns trusted plaintext headers
  - follow-up implementation should switch to X-Aisphere-Principal-JWT

kernel-layout / service templates:
  - should default backend service authn mode to principal_jwt once IAM emits Principal JWT
```

## 13. 迁移步骤

### Step 1: Kernel

确保服务依赖包含支持 `principal_jwt` 的 Kernel 版本。

### Step 2: IAM

将 ExternalAuthorize 从 plaintext trusted headers 改成：

```go
principalToken, err := authn.SignPrincipalJWT(principal, cfg)
if err != nil {
    return nil, err
}
tr.ReplyHeader().Set(cfg.Header, principalToken)
```

### Step 3: Envoy Gateway

SecurityPolicy `headersToBackend` 只放行：

```yaml
headersToBackend:
  - X-Aisphere-Principal-JWT
```

### Step 4: Backend services

切换：

```yaml
security.authn.mode: principal_jwt
```

### Step 5: Remove legacy dependency

业务服务不再依赖明文 `X-Aisphere-*` Principal headers。`gateway_trusted` 保留为 legacy compatibility mode。

## 14. 验收清单

- [ ] 外部 client 注入 `X-Aisphere-Principal-JWT` 时，Gateway 会清理。
- [ ] 无 `Authorization` 请求被 IAM ExternalAuth 拒绝。
- [ ] 无效原生 token 被 IAM ExternalAuth 拒绝。
- [ ] 用户 disabled / session revoked 后，新请求被拒绝。
- [ ] IAM 成功返回 `X-Aisphere-Principal-JWT`。
- [ ] Envoy 只转发 `X-Aisphere-Principal-JWT` 到 backend。
- [ ] Backend `principal_jwt` middleware 能恢复 `authn.Principal`。
- [ ] 业务 handler 无需解析 token。
- [ ] authz 仍由 accessx/authz/SpiceDB 决策。
- [ ] 流式请求取消由 context/run lifecycle 处理，不依赖 token exp。
