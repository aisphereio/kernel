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
  - token 签发、刷新、校验
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

type Provider interface {
    Authenticator
    TokenService
    LoginService
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

## Agent 硬规则

1. 不允许业务组件直接 import Casdoor SDK、Casdoor object、SpiceDB SDK。
2. 不允许业务组件自己解析 token 或手写权限判断。
3. authn 实现必须接到 `authn.Provider` / `authn.ManagementProvider`。
4. authz 实现必须接到 `authz.Provider` / `authz.AdminProvider`。
5. 请求热路径必须通过 `accessx.Guard` 或 autowire/serverx 自动装配。
6. 用户、组织、组的默认管理后台是 Casdoor；Kernel 不默认自建用户中心。
7. 资源级权限默认走 Kernel authz 接口，生产优先 SpiceDB。
