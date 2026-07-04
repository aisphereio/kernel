# Authn/Authz Provider 边界：Kernel 接口，Casdoor/SpiceDB 实现

## 结论

Kernel 的正式边界是：

```text
业务代码 -> Kernel authn/authz/accessx 接口 -> provider 实现
```

默认实现选择：

```text
authn: Casdoor
  - OAuth/OIDC 登录
  - OIDC RP-Initiated Logout（登出）
  - token 签发、刷新、校验、吊销
  - 用户、组织、组、角色管理后台

authz: SpiceDB 优先，Casdoor/Casbin 可作为过渡实现
  - 资源级 ReBAC/ABAC 授权
  - relationship/schema 管理
  - lookup resources / subjects
```

Kernel 不自建默认用户中心，也不让业务组件直接依赖 Casdoor 或 SpiceDB SDK。业务组件只能依赖：

```text
authn.Authenticator / authn.Provider / authn.ManagementProvider
authz.Authorizer / authz.Provider / authz.AdminProvider
accessx.Guard
```

## 为什么这样分层

Casdoor 已经是成熟的 OAuth/OIDC、用户、组织、组、角色管理产品。继续在 Kernel 里重造用户中心会让框架变成业务系统，复杂度过高。

SpiceDB 适合做资源级授权，尤其是多组织、多项目、多资源、多关系场景。Kernel 应该统一授权接口和规则生成，让业务代码不用理解 SpiceDB SDK。

因此：

```text
Casdoor 是 IAM Control Plane / 用户中心 / 登录中心
SpiceDB 是 Authz Decision Engine / 关系权限引擎
Kernel 是应用开发框架和治理层
```

## authn 职责

authn 只回答：

```text
这个 token/session/API key/service credential 是谁？是否有效？
如何构建 IdP 登录/登出 URL？
```

Kernel 接口：

```go
type Authenticator interface {
    Authenticate(ctx context.Context, credential Credential) (Principal, error)
}

type TokenService interface {
    ExchangeCode(...)
    RefreshToken(...)
    VerifyToken(...)
    RevokeToken(...)
}

type LoginService interface {
    BuildLoginURL(...)
    HandleCallback(...)
}

type LogoutService interface {
    BuildLogoutURL(...)
}

type Provider interface {
    Authenticator
    TokenService
    LoginService
    LogoutService
}
```

默认 provider：

```text
authn/casdoor.Client
```

业务服务不得直接调用 Casdoor SDK 的 token/user 方法。

## Casdoor 管理职责

用户、组织、多级组、角色管理默认交给 Casdoor 管理后台。

Kernel 只保留 provider-neutral 管理接口，供 IAM provisioning、同步、测试使用：

```go
type ManagementProvider interface {
    Provider
    IdentityAdmin
}
```

业务如果要读取用户/组织/组视图，应通过 Kernel 的 view 接口或 adapter，不得 import Casdoor object。

## authz 职责

authz 只回答：

```text
subject 能不能对 resource 执行 permission/action？
```

Kernel 接口：

```go
type Authorizer interface {
    Check(ctx context.Context, req CheckRequest) (Decision, error)
}

type Service interface {
    Authorizer
    BatchAuthorizer
    ResourceLookup
    SubjectLookup
    RelationshipStore
    SchemaManager
}
```

默认生产 provider：

```text
authz/spicedb.Client
```

过渡 provider 可以是：

```text
Casdoor permission adapter
Casbin adapter
memory/dev adapter
```

但业务代码仍然只能调用 Kernel authz/accessx 接口。

## accessx 职责

accessx 是请求热路径门面，把 authn、authz、audit 组合起来：

```text
credential/principal
  -> authn
  -> access policy / generated resolver
  -> authz
  -> audit
```

业务 handler 不应该自己拼 authn/authz/audit 流程。

### SkipPolicy — 统一的跳过策略

accessx 引入 `SkipPolicy` 来统一控制认证和授权的跳过行为，替代了旧的 `AllowAll` + `AuthzMode` 组合：

| 策略 | 认证 | 授权 | 审计 | 适用场景 |
|------|------|------|------|---------|
| `SkipDefault` | ✅ 需要 | ✅ 需要 | ✅ 记录 | 标准 CRUD 操作 |
| `SkipAuthz` | ✅ 需要 | ❌ 跳过 | ✅ 记录 | GetMe、CreateOrganization |
| `SkipAll` | ❌ 跳过 | ❌ 跳过 | ✅ 记录 | 健康检查、登录、token exchange |

`SkipPolicy` 通过 `AccessConfig` 配置驱动，支持 YAML 配置：

```yaml
security:
  access:
    skip_operations:
      - GetMe
      - CreateOrganization
    public_operations:
      - Login
      - Exchange
```

`SkipPolicyResolver` 由 `accessx.NewSkipPolicyResolver(cfg)` 创建，支持短方法名、完整 gRPC 方法名、HTTP 路径和通配符匹配（如 `/v1/authn/*`）。中间件层在调用 Resolver 之前先检查 SkipPolicy，实现高效短路。

旧的 `AuthzMode` 和 `AllowAll` 字段标记为 Deprecated，但保持向后兼容。当 `SkipPolicy` 设置时，优先于旧字段。

## Gateway/IAM/业务服务流程

MVP 默认：

```text
Client
  -> Gateway
      - route match
      - PUBLIC 放行
      - AUTHENTICATED/AUTHORIZED 要求 Authorization 存在
      - token relay
  -> Business Service
      - Kernel authn/casdoor 校验 token
      - Kernel accessx 生成资源授权请求
      - Kernel authz/spicedb 做资源授权
```

Gateway 不作为 IAM，也不代理所有业务请求到 IAM。IAM/Casdoor 是登录和用户中心，SpiceDB/Authz 是权限决策，Gateway 是路由和边界准入。

## Provider 包装模式

当需要扩展或修改 Kernel provider 的行为时（如添加时钟偏差处理、修改 OAuth 参数传递），应使用**包装器模式（Wrapper Pattern）**：

```go
type casdoorClockSkewProvider struct {
    *casdoor.Client          // 嵌入原始 provider
    cfg casdoor.Config
    sdk *casdoorsdk.Client   // 额外依赖
}

func newCasdoorClockSkewProvider(cfg casdoor.Config, client *casdoor.Client) *casdoorClockSkewProvider {
    // 初始化 SDK
    sdk := casdoorsdk.NewClient(cfg.Endpoint, cfg.ClientID, cfg.ClientSecret, cert, ...)
    return &casdoorClockSkewProvider{Client: client, cfg: cfg, sdk: sdk}
}

// 重写需要修改的方法
func (p *casdoorClockSkewProvider) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
    // 自定义实现，不调用 p.Client.ExchangeCode
}

// 未重写的方法自动委托给嵌入的 casdoor.Client
```

**关键原则**：
1. 嵌入原始 provider，未重写的方法自动委托
2. 只重写需要修改行为的方法
3. 包装器必须实现完整的 `identityProvider` 接口（`Authenticator`、`LoginService`、`LogoutService`、`TokenService`、`ProfileService`、`IdentityAdmin`）
4. 包装器在 `data.go` 的 `newAuthenticator` 中创建，对 service 层透明

## Access Resolver 覆盖模式

Kernel 的 `middleware/access` 使用 `AccessResolver` 链来决定每个请求的授权检查。生成的 `KernelAccessResolver` 从 proto 的 `aisphere.access.v1.policy` 自动生成授权规则。但在某些场景下，生成的规则需要被覆盖：

### 场景：GetMe 不需要 SpiceDB 检查

`GetMe` 是获取当前用户自身信息的端点，proto 中声明为 `exposure: AUTHENTICATED`。但生成的 `AccessResolver` 仍然会生成 `SELF_CHECK` 模式的授权规则（resource: `iam:self`, action: `read`）。如果 SpiceDB schema 中没有 `iam:self` 资源类型，会导致 `AUTHZ_PERMISSION_DENIED`。

**推荐方案**：使用 `SkipPolicy` 配置驱动，在 `security.access.skip_operations` 中列出需要跳过授权检查的操作：

```yaml
security:
  access:
    skip_operations:
      - GetMe
      - CreateOrganization
```

**代码方案**：在 `iamAccessResolver` 中使用 `accessx.NewSkipPolicyResolver` 统一处理：

```go
func newIAMAccessResolver(security conf.SecurityConfig) mwaccess.Resolver {
    skipResolver := accessx.NewSkipPolicyResolver(accessx.AccessConfig{
        SkipOperations: security.Access.SkipOperations,
    })
    return func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
        if policy := skipResolver(operation); policy != accessx.SkipDefault {
            return accessx.Check{SkipPolicy: policy, AuditAction: resolveAuditAction(operation)}, true, nil
        }
        // 其他操作走生成的 resolver 链
        resolvers := []mwaccess.Resolver{
            v1.IAMAuthServiceKernelAccessResolver,
            // ...
        }
        for _, resolver := range resolvers {
            check, ok, err := resolver(ctx, operation, req)
            if err != nil || ok {
                return check, ok, err
            }
        }
        return accessx.Check{}, false, nil
    }
}
```

**旧方案（Deprecated）**：直接在 resolver 中返回 `accessx.Check{}, false, nil` 跳过授权检查。新代码应使用 `SkipPolicy`。

### 场景：手写 AccessResolver 未被使用

手写的 `AccessResolver`（如 `iam_auth_resolver.go` 中的 `IAMAuthServiceAccessResolver`）可能使用不同的授权模式（如 `CHECK_ONLY`），但如果 `iamAccessResolver` 中只引用了生成的 `KernelAccessResolver`，手写的 resolver 不会生效。

**解决方案**：确保 `iamAccessResolver` 的 resolver 列表包含所有需要的手写 resolver，或者将手写 resolver 的逻辑合并到 `iamAccessResolver` 中。

### 设计原则

| 端点级别 | 认证 | 授权 | 说明 |
|---------|------|------|------|
| `PUBLIC` | 不需要 | 不需要 | 登录 URL、code exchange |
| `AUTHENTICATED` | 需要 | 不需要 | GetMe、RefreshToken |
| `AUTHORIZED` | 需要 | 需要 | CRUD 操作 |
| `INTERNAL` | 服务认证 | 可选 | 内部服务调用 |

`AUTHENTICATED` 级别的端点虽然 proto 中声明了 `action` 和 `resource`，但生成的 `AccessResolver` 可能产生不必要的 SpiceDB 检查。需要在 `iamAccessResolver` 中根据实际情况跳过或修改。

## Agent 硬规则

1. 不允许业务组件直接 import Casdoor SDK、Casdoor object、SpiceDB SDK。
2. 不允许业务组件自己解析 token 或手写权限判断。
3. authn 实现必须接到 `authn.Provider` / `authn.ManagementProvider`。
4. authz 实现必须接到 `authz.Provider` / `authz.AdminProvider`。
5. 请求热路径必须通过 `accessx.Guard` 或 autowire/serverx 自动装配。
6. 用户、组织、组的默认管理后台是 Casdoor；Kernel 不默认自建用户中心。
7. 资源级权限默认走 Kernel authz 接口，生产优先 SpiceDB。
8. 包装 Kernel provider 时，必须检查被包装方法是否完整实现了所有参数传递（如 Casdoor SDK 的 `GetOAuthToken` 缺少 `RedirectURL`）。
9. `AUTHENTICATED` 级别的端点（如 `GetMe`）不需要 SpiceDB 授权检查，应使用 `SkipPolicy=SkipAuthz` 跳过。
10. 新代码应使用 `SkipPolicy` 替代 `AllowAll` 和 `AuthzMode`。旧字段标记为 Deprecated，保持向后兼容。
11. 配置驱动的跳过策略应使用 `security.access.skip_operations` 和 `security.access.public_operations`，而非硬编码在 resolver 中。
