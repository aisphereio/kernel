# IAM 微服务：Kernel 规范驱动开发流程

本文档说明 IAM 微服务应该如何使用 Kernel 开发框架落地。目标不是让业务直接 import Casdoor 或 SpiceDB，而是让业务只依赖 Kernel 接口。

## 一、标准开发流程

```bash
kernel new iam-service --repo ./layout
cd iam-service
```

然后编写业务 proto，例如：

```text
api/iam/v1/iam.proto
```

proto 必须声明：

- `google.api.http`：外部 HTTP 契约
- `aisphere.access.v1.policy`：访问策略

示例：

```proto
rpc GetUser(GetUserRequest) returns (User) {
  option (google.api.http) = { get: "/v1/iam/users/{id}" };
  option (aisphere.access.v1.policy) = {
    exposure: AUTHORIZED
    authz: {
      action: "iam.user.get"
      resource: "iam:user:{id}"
      audience: "iam-service"
      mode: CHECK_ONLY
    }
    audit: { enabled: true event: "iam.user.get" risk: "low" }
  };
}
```

生成代码：

```bash
make api
make proto-check
```

生成器应该产出：

- HTTP/gRPC glue code
- AccessResolver
- RequestInfoResolver
- errorx 映射
- Gateway route manifest（后续阶段）

## 二、业务服务依赖边界

业务服务只依赖 Kernel：

```text
authn.Provider / authn.Authenticator
authz.Provider / authz.Authorizer
accessx.Guard
serverx.RuntimeProviders
requestx.Info
```

业务代码不得直接依赖：

```text
Casdoor SDK
SpiceDB/Authzed SDK
原始 grpc.Dial/http.Client
手写 authz resolver
手写 token parsing
```

## 三、默认 provider

```text
authn 默认实现：authn/casdoor.Client
authz 默认实现：authz/spicedb.Client
```

Casdoor 负责用户、组织、多级组、角色、OAuth/OIDC 登录和 token 管理。

SpiceDB 负责资源级授权和 ReBAC 关系判断。

Kernel 负责 provider-neutral 接口、ctx 注入、requestx.Info、accessx.Guard、audit、serverx 装配、proto 生成和治理校验。

## 四、验证场景

`validation/iamservice` 覆盖三个核心场景：

1. 未登录访问需要鉴权的接口：Kernel authn 拒绝，业务 handler 不执行。
2. 已登录但无权限：Kernel accessx/authz 拒绝，业务 handler 不执行。
3. 已登录且有权限：业务 handler 执行，并写入 audit。

这正是 Spring Cloud Resource Server 范式在 Kernel 中的对应实现：

```text
Gateway token relay / passive authn
  -> IAM/业务服务作为 Resource Server
  -> Kernel authn provider 校验 token
  -> Kernel accessx + authz provider 做资源授权
```


## 五、构建委托

本地沙箱如果缺少 Buf 远程依赖或公网 Go module，`make api` 可能失败。这个失败不代表业务范式失败。完整链路由 GitHub Actions `iam-service-demo.yml` 验证：

```text
kernel new iam-service
buf dep update
make api
make proto-check
go test ./validation/iamservice
```

本地只保留 targeted tests，完整生成和构建交给 Actions。
