# IAM 微服务验证用例

这个目录不是 Kernel 核心包，而是验证业务代码，用来证明 IAM 服务可以按照 Kernel 开发范式落地：

1. `kernel new iam-service --repo ./layout`
2. 编写 `api/iam/v1/iam.proto`
3. `make api` 生成 HTTP/gRPC/authz/request-info glue code
4. 业务服务只依赖 Kernel 的 `authn`、`authz`、`accessx`、`serverx`
5. authn 默认实现是 Casdoor provider，authz 默认实现是 SpiceDB provider

当前测试使用 fake Casdoor token provider + memory authorizer 模拟真实链路，覆盖：

- 未登录：authn middleware 拒绝，业务 handler 不执行
- 已登录但无权限：accessx/authz 拒绝，业务 handler 不执行
- 已登录且有权限：业务 handler 执行，并写入 audit 记录

真实生产路径中，fake provider 替换为：

- `authn/casdoor.Client`
- `authz/spicedb.Client`
