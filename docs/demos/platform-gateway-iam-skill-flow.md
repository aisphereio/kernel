# Gateway / IAM / SkillService 规范驱动流程

本文档描述当前验证业务的目标形态。真实业务组件必须走：

```text
kernel new <service>
  -> 写 proto
  -> 声明 google.api.http + aisphere.access.v1.policy
  -> make api
  -> 生成 RequestInfoResolver / AccessResolver / GatewayManifest
  -> serverx 装配 provider
  -> 写业务 handler
```

## 1. 登录流程

```text
POST /v1/iam/login
  -> Gateway route match
  -> route exposure=PUBLIC
  -> Gateway 不要求 Authorization
  -> 转发 IAMService Login
  -> IAMService 通过 authn/Casdoor-like provider 签发 token
  -> 返回 alice-token
```

## 2. 访问 Skill 流程

```text
GET /v1/skills/s1
  -> Gateway route match
  -> route exposure=AUTHORIZED
  -> Gateway passive authn：只检查 Authorization 是否存在
  -> token relay 到 SkillService
  -> SkillService authn：验证 token，得到 Principal
  -> SkillService requestx.Info：由 generated resolver 注入
  -> SkillService accessx：由 generated AccessResolver 得到 action=skill.get resource=skill:s1
  -> SkillService authz：调用 provider 判断权限
  -> accessx audit：记录 success/denied
  -> handler 返回 Skill
```

## 3. 三种结果

| 场景 | Gateway | SkillService | 结果 |
|---|---|---|---|
| 无 token | 401 | 不执行 | UNAUTHENTICATED |
| bob 有 token 无权限 | 放行 | authz deny，handler 不执行 | 403 |
| alice 有 token 有权限 | 放行 | authn/authz/audit/handler 全通过 | 200 Skill |

## 4. 当前验证实现

`validation/platformflow` 使用：

- `gatewayx.NewEtcdRegistry(gatewayx.NewMemoryKVStore(), ...)` 模拟 etcd route registry。
- `gatewayx.StaticHosts` 模拟 K8s Service DNS。
- `fakeAuthnProvider` 模拟 Casdoor authn provider。
- `authz.NewMemoryAuthorizer` 模拟 SpiceDB authz provider。
- `SkillServiceGatewayManifest` / `SkillServiceRequestInfoResolver` / `SkillServiceAccessResolver` 模拟生成器产物。

后续生产接入只替换 provider / registry / resolver 产物，不改变业务 handler 形态。

## 5. Gateway -> Service gRPC 调用

当前验证已经从 fake `UpstreamInvoker` 升级为 generated-equivalent invoker registry：

```text
Gateway Dispatcher
  -> gatewayx.InvokerRegistry
  -> IAMServiceGatewayBindLogin / SkillServiceGatewayBindGetSkill
  -> gatewayx.UnaryInvoker / gatewayx.GRPCUnaryInvoker
  -> Service Kernel server chain
```

完整生产形态：

```text
POST /v1/iam/login
  -> Gateway
  -> IAMServiceGatewayBindLogin
  -> IAMService gRPC client Login
  -> IAMService server chain / handler
  -> alice-token

GET /v1/skills/s1
  -> Gateway
  -> SkillServiceGatewayBindGetSkill: path id -> GetSkillRequest
  -> SkillService gRPC client GetSkill
  -> SkillService server chain
  -> authn(Casdoor provider)
  -> accessx/authz(SpiceDB provider)
  -> audit
  -> handler
  -> Skill
```

因此 authn/authz 的权威判断仍然在 SkillService 内部，Gateway 只是根据 route policy 做 passive authn 和 token relay。
