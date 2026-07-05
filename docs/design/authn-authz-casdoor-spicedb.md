# Authn/Authz/Audit Boundary: Casdoor + SpiceDB

Kernel treats authentication, authorization and auditing as separate concerns.

- `authn`: who is the caller? Casdoor/OIDC maps tokens into `authn.Principal`.
- `authz`: can the caller perform this permission on this resource? SpiceDB owns ReBAC relationships.
- `auditx`: what happened? The decision and context are recorded for traceability.

## Casdoor responsibility

Casdoor is the identity provider and identity-side admin projection:

- users
- organizations
- applications/OAuth clients
- organization groups
- OAuth/OIDC code exchange and JWT verification

Kernel IAM DB remains the business source of truth for platform organizations,
projects and resource metadata. Casdoor is synchronized from IAM when identity
provider state is needed.

## Casdoor applications and credentials

Use two credential sets:

1. Login/OIDC application
   - Used by user-facing services for OAuth code exchange and token verification.
   - Example: `aisphere-hub` under organization `aisphere`.
   - Config path: `security.authn.casdoor.client_id`, `client_secret`, `certificate`.

2. Management application
   - Used by Kernel IAM provisioning code to create or update users,
     organizations, applications and groups.
   - Example: `kernel-iam-admin` under `built-in` or an operations organization.
   - Config path: `security.authn.casdoor.admin.*`.

For local development the adapter can fall back to the login application if the
admin block is omitted. Production should use a dedicated management application
with the smallest Casdoor permissions needed for provisioning.

Do not create a new Casdoor organization just to let Kernel connect to Casdoor.
Organizations represent identity tenants or business organizations. Applications
represent protected services and OAuth clients.

## Groups, not departments

Casdoor's organization tree is modeled as groups. Groups belong to an
organization and can have a parent group. Casdoor group types are `Physical` and
`Virtual`: a user can belong to only one Physical group but multiple Virtual
groups.

Kernel therefore exposes `authn.GroupAdmin`, not `DepartmentAdmin`.

## SpiceDB responsibility

SpiceDB stores the authorization graph projection:

```zed
group:eng#member@user:u1
project:p1#editor@group:eng#member
resource:r1#viewer@user:u2
```

Business services should not call the Casdoor SDK or SpiceDB SDK directly. They
should depend on:

- `authn.Principal`
- `authz.Authorizer`
- `auditx.Recorder`
- optionally `accessx.Guard` at transport boundaries

## Casdoor SDK RedirectURL 陷阱

**重要发现**：Casdoor Go SDK 的 `GetOAuthToken(code, state)` 方法在构造 OAuth2 token 请求时**没有设置 `redirect_uri` 参数**。SDK 源码中对应的行被注释掉了：

```go
// casdoor-go-sdk/casdoorsdk/auth.go (简化)
func (c *Client) GetOAuthToken(code, state string) (*oauth2.Token, error) {
    config := oauth2.Config{
        ClientID:     c.ClientId,
        ClientSecret: c.ClientSecret,
        // RedirectURL 未设置！ ← 这行被注释掉了
        Endpoint: oauth2.Endpoint{...},
    }
    return config.Exchange(ctx, code)
}
```

而 Kernel 的 `authn/casdoor/token.go` 中的 `exchangeOAuthToken` 方法**正确设置了 `RedirectURL`**：

```go
// kernel/authn/casdoor/token.go
func exchangeOAuthToken(ctx context.Context, cfg Config, req AuthCodeExchangeRequest) (*oauth2.Token, error) {
    config := oauth2.Config{
        ClientID:     cfg.ClientID,
        ClientSecret: cfg.ClientSecret,
        RedirectURL:  req.RedirectURI,  // 正确设置
        Endpoint:     oauth2.Endpoint{...},
    }
    return config.Exchange(ctx, req.Code)
}
```

**影响**：如果 IAM 服务包装了 `casdoor.Client` 并直接调用 Casdoor SDK 的 `GetOAuthToken`（而不是 Kernel 的 `exchangeOAuthToken`），Casdoor 的 token endpoint 会因为缺少 `redirect_uri` 而拒绝请求，返回 500 `AUTHN_IDENTITY_BACKEND_FAILED`。

**解决方案**：在包装 provider 中重写 `ExchangeCode` 方法，使用标准 `golang.org/x/oauth2` 库手动构造 `oauth2.Config` 并设置 `RedirectURL`，而不是调用 Casdoor SDK 的 `GetOAuthToken`。

## Casdoor JWT 时钟偏差处理

Casdoor 签发的 JWT 在服务端验证时，如果 Casdoor 服务器和 IAM 服务之间的系统时间存在偏差（常见于 Docker 或跨主机部署），JWT 的 `iat`（签发时间）和 `exp`（过期时间）校验会失败。

Kernel 的 `casdoor.Client.VerifyToken` 使用 `jwt.TimeFunc` 进行时间比较，默认使用 `time.Now()`。如果 Casdoor 服务器时间比 IAM 快 30 秒，新签发的 JWT 在 IAM 验证时会显示"未生效"。

**解决方案**：在 IAM 中创建 `casdoorClockSkewProvider` 包装器，在解析 JWT 时设置 60 秒的时钟偏差：

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

注意 `jwt.TimeFunc` 是 `golang-jwt` 库的全局变量，修改时需要加锁保护。

## External YAML

Example config lives in `configs/security.example.yaml`.

Recommended production shape:

```yaml
security:
  authn:
    provider: casdoor
    casdoor:
      endpoint: "https://door.example.com"
      organization_name: "aisphere"
      application_name: "aisphere-hub"
      client_id: "${CASDOOR_CLIENT_ID}"
      client_secret: "${CASDOOR_CLIENT_SECRET}"
      certificate: "${CASDOOR_CERTIFICATE}"
      admin:
        enabled: true
        organization_name: "built-in"
        application_name: "kernel-iam-admin"
        client_id: "${CASDOOR_ADMIN_CLIENT_ID}"
        client_secret: "${CASDOOR_ADMIN_CLIENT_SECRET}"
        certificate: "${CASDOOR_ADMIN_CERTIFICATE}"
  authz:
    provider: spicedb
    spicedb:
      endpoint: "spicedb:50051"
      token: "${SPICEDB_TOKEN}"
      insecure: false
```
