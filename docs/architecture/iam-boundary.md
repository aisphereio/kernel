# IAM 边界：Casdoor 作为默认用户中心，Kernel 作为接口与治理层

## 结论

当前正式方向：

```text
用户、组织、多级组、角色、登录页面、OAuth/OIDC：交给 Casdoor
资源级授权：通过 Kernel authz 接口，生产优先 SpiceDB
业务服务：只依赖 Kernel authn/authz/accessx，不直接依赖 Casdoor/SpiceDB SDK
```

Kernel 不默认自建用户中心。前一阶段的 `iamx/db`、`iamx/memory` 只保留为 experimental / test / 本地同步视图，不作为默认生产路径。

## 分层

```text
Casdoor
  - 用户管理
  - 组织管理
  - 多级组/角色管理
  - OAuth/OIDC 登录、token、JWKS

Kernel authn
  - provider-neutral token/login/principal 接口
  - 默认实现：authn/casdoor

Kernel authz
  - provider-neutral permission/resource/relationship 接口
  - 默认生产实现：authz/spicedb
  - 可选过渡实现：Casdoor/Casbin/memory

Kernel accessx
  - 组合 authn + authz + audit
  - 由 autowire/serverx 放入请求链
```

## 为什么不把用户中心放进 Kernel

Casdoor 已经提供成熟的用户、组织、组、角色、OAuth/OIDC 和管理后台。Kernel 的目标是框架，不是再造一个 IAM 产品。继续在 Kernel 内维护完整用户组织 CRUD 会导致：

```text
框架边界变重
重复 Casdoor 能力
前端管理成本增加
业务 Agent 容易绕过统一 provider 接口
```

所以默认收敛为：Casdoor 管用户中心，Kernel 管接口规范、请求治理、资源授权和审计。

## iamx 定位

`iamx` 不是默认主存储，而是 Kernel 的 provider-neutral 用户/组织/组视图接口。

```text
iamx/casdoor 或 iamx/authnadapter：默认 Casdoor-backed view
iamx/memory：测试用
iamx/db：experimental，可用于未来脱离 Casdoor 或做本地缓存/同步视图
```

业务代码如果需要用户/组织/组信息，仍然不能直接 import Casdoor SDK，而是通过 Kernel view/service 接口。

## 参考文档

- `docs/architecture/authn-authz-provider-boundary.md`
- `docs/contracts/iam-domain.md`
- `docs/reference/casdoor-study.md`
