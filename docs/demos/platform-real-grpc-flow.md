# Gateway -> IAMService -> SkillService 真实 gRPC 验证链路

本 demo 验证 Gateway 到内部服务的默认生产路径：HTTP 入口、Gateway 路由、gRPC client 调 service。

```text
POST /v1/iam/login
  -> Gateway HTTP
  -> route manifest
  -> generated IAMService gRPC client
  -> IAMService gRPC server
  -> Login
  -> alice-token

GET /v1/skills/s1
  -> Gateway HTTP
  -> passive authn / token relay
  -> generated SkillService gRPC client
  -> SkillService gRPC server
  -> Kernel server chain
  -> authn provider 验 token
  -> access resolver
  -> authz provider 查权限
  -> accessx audit
  -> handler 返回 Skill
```

## 权威判断点

- Gateway 只判断是否需要 Authorization header，不做资源级授权。
- SkillService 内部执行权威 authn/authz/audit。
- Casdoor 是 authn provider 默认实现。
- SpiceDB 是 authz provider 默认实现。
- validation 中使用 fake Casdoor-like provider 和 memory SpiceDB-like authorizer，只验证框架链路。

## 本地验证

`validation/platformflow` 使用 `bufconn` 启动真实 gRPC server/client，不占用端口；生产环境替换为真实 gRPC listener 和 K8s Service DNS。
