# Validation 验证代码

这里存放用于验证 Kernel 开发范式的业务样例和集成测试。

这些代码不是 Kernel 核心能力包，不能被 `serverx`、`requestx`、`middleware` 等核心包反向依赖。

当前验证场景：

| 目录 | 说明 | 命令 |
|---|---|---|
| `iamgateway/` | Gateway 入口治理 + 服务间调用 IAM mock | `go test ./validation/iamgateway` |
