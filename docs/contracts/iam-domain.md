# IAM 领域视图规范

## 当前结论

Kernel 不默认自建用户中心。用户、组织、多级组、角色和登录管理默认交给 Casdoor。

Kernel 保留 `iamx` 的目的不是取代 Casdoor，而是给业务和框架提供 provider-neutral 的用户/组织/组视图，避免业务代码直接依赖 Casdoor SDK 或 Casdoor object。

```text
Casdoor：用户中心、组织中心、多级组/角色管理、OAuth/OIDC
Kernel iamx：统一视图接口、测试实现、可选同步/缓存实现
Kernel authn：认证接口，默认 authn/casdoor
Kernel authz：授权接口，默认生产 authz/spicedb
```

## 核心对象

`iamx.User`、`iamx.Organization`、`iamx.Group`、`iamx.Membership` 是 Kernel 的稳定投影模型。

这些对象可以来自：

```text
Casdoor adapter
memory test store
experimental DB store
未来其他 IdP / LDAP / 企业微信 / 飞书 adapter
```

业务不得直接使用 Casdoor object。

## Group 多级视图

`iamx.Group.ParentID` 表示多级 group。`Group.Path` 由 Kernel adapter/view 维护，业务不得手写。

如果用户加入子组，读取有效归属时应使用：

```go
ListEffectiveMemberships(ctx, orgID, userID)
ListUserGroups(ctx, UserGroupQuery{IncludeInherited: true})
```

## CRUD 语义

`iamx.Directory` 提供完整 CRUD，但默认生产实现应优先 backed by Casdoor。

```text
Create*  明确创建，已存在返回 conflict
Update*  明确更新，不存在返回 not found
Upsert*  仅用于同步/provisioning/幂等初始化
Delete*  按 provider 能力执行；Casdoor-backed 实现可降级为禁用
```

## Provider 策略

默认：

```text
iamx/casdoor 或 iamx/authnadapter -> Casdoor-backed view
```

测试：

```text
iamx/memory.Store
```

实验：

```text
iamx/db.Store
```

`iamx/db.Store` 不作为默认生产 IAM 中心。只有在明确要做本地缓存、离线同步视图、或未来脱离 Casdoor 时才启用。

## 与 authn/authz 的关系

```text
authn：token/session/login/principal，默认实现 Casdoor
iamx：用户/组织/组视图，默认 backed by Casdoor
authz：subject/action/resource 决策，默认生产实现 SpiceDB
accessx：请求热路径组合 authn + authz + audit
```

Redis 黑名单、session、token version 属于 authn/token state。

资源权限判断属于 authz/accessx，不属于 iamx。

## Agent 规则

1. 不得直接依赖 Casdoor SDK、Casdoor object 或 provider-specific 用户结构。
2. 用户/组织/组视图必须通过 Kernel iamx/authn provider 接口。
3. 生产默认用户中心是 Casdoor，不允许业务组件私自建用户表替代。
4. `iamx/db` 是 experimental，不得作为默认生产 IAM 中心，除非架构文档明确批准。
5. 资源级权限不得塞进 iamx，必须通过 authz/accessx。
