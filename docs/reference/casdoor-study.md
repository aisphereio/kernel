# Casdoor 源码学习记录

本轮通过 GitHub Actions 拉取 `casdoor/casdoor`，打包 object/controllers/authz/util/router 等关键目录作为学习包。

学习重点：

```text
object/user.go           用户字段、第三方账号绑定、登录状态
object/organization.go   组织配置、登录策略、MFA、用户字段配置
object/group.go          多级组、ParentId、IsTopGroup、Children
object/role.go           Users/Groups/Roles/Domains
object/permission.go     Resources/Actions/Effect
controllers/*            OAuth/OIDC 和管理 API 形态
```

## Kernel 采纳

```text
1. 保留用户、组织、多级组、membership 的核心抽象。
2. 支持 group.ParentID 和 group.Path。
3. 支持祖先/子孙查询。
4. 支持 effective membership，用户加入子组后可推导父组 inherited membership。
5. Casdoor 作为 OAuth/OIDC、外部用户中心和同步源。
6. Kernel IAM 暴露 provider-neutral iamx.Service。
```

## Kernel 不采纳

```text
1. 不让业务服务直接 import Casdoor SDK。
2. 不把 Casdoor 的所有用户字段都进入核心模型。
3. 不把 Casdoor role/permission 直接作为 Kernel 资源授权模型。
4. 不让 IAM 代理业务流量。
5. 不让 Casdoor object 成为业务包的持久化模型。
```

## Adapter 策略

Casdoor 对接路径：

```text
authn/casdoor
  -> authn.IdentityAdmin
  -> iamx/authnadapter.Directory
  -> iamx.Service
```

其中 `iamx/authnadapter` 做字段映射和 provider 能力降级。例如当前 provider-neutral user delete 语义在 Casdoor adapter 中降级为 `DisableUser`，避免把 Casdoor 的删除语义泄露给业务。

## DB 策略

Kernel 自己的 IAM DB 由 `iamx/db.Store` 负责：

```text
kernel_iam_users
kernel_iam_organizations
kernel_iam_groups
kernel_iam_memberships
```

DB Store 通过 `database/sql` 实现，避免业务层依赖 GORM 或 Casdoor object。启动装配时可以由 dbx 提供底层连接。

## Casdoor SDK 已知问题

### GetOAuthToken 缺少 RedirectURL

Casdoor Go SDK 的 `GetOAuthToken(code, state string)` 方法在构造 `oauth2.Config` 时**没有设置 `RedirectURL`**。SDK 源码中对应的行被注释掉了：

```go
// casdoor-go-sdk/casdoorsdk/auth.go
func (c *Client) GetOAuthToken(code, state string) (*oauth2.Token, error) {
    config := oauth2.Config{
        ClientID:     c.ClientId,
        ClientSecret: c.ClientSecret,
        // RedirectURL: c.RedirectUrl,  ← 这行被注释掉了！
        Endpoint: oauth2.Endpoint{
            TokenURL:  c.Endpoint + "/api/login/oauth/access_token",
            AuthStyle: oauth2.AuthStyleInParams,
        },
    }
    return config.Exchange(context.Background(), code)
}
```

**影响**：Casdoor 的 token endpoint 要求 `redirect_uri` 参数与授权请求中的 `redirect_uri` 匹配。缺少该参数会导致 Casdoor 拒绝 code exchange 请求，返回 400 错误。

**解决方案**：不要直接调用 Casdoor SDK 的 `GetOAuthToken`。应使用 Kernel 的 `exchangeOAuthToken`（在 `authn/casdoor/token.go` 中），或者使用标准 `golang.org/x/oauth2` 库手动构造 `oauth2.Config` 并设置 `RedirectURL`。

### ParseJwtToken 使用全局 jwt.TimeFunc

Casdoor SDK 的 `ParseJwtToken` 使用 `golang-jwt` 库的全局 `jwt.TimeFunc` 进行时间比较。如果 Casdoor 服务器和验证服务之间的系统时间存在偏差，JWT 的 `iat`/`exp` 校验会失败。

**解决方案**：在调用 `ParseJwtToken` 前临时修改 `jwt.TimeFunc` 添加时钟偏差，调用后恢复。注意加锁保护全局变量。

最终范式：

```text
Casdoor 提供登录、token、外部用户中心能力
Kernel IAM DB 管用户、组织、多级组和 membership
业务资源授权继续走 accessx/authz
Gateway 只做路由和边界准入
```
