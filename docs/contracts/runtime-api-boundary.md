# Runtime API 边界

Kernel 仓库分为四类表面：runtime API、development tooling、external layout、examples。业务代码只能依赖 runtime API；服务 boot 代码可以依赖 runtime API 中的装配类包，例如 `serverx`、`securityx`、`bootx`。

## 1. Runtime API

业务代码、服务 boot 代码和生成代码可以 import：

```text
errorx logx configx metricsx serverx securityx bootx contextx
transportx/http transportx/grpc
requestx accessx authn authz auditx
gatewayx admissionx ratelimitx clientpolicyx
dbx cachex objectstorex dtmx
selectorx registry encodingx
```

### 1.1 装配边界

`securityx` 是安全配置和 provider-neutral runtime 构造层，不是 middleware 装配层。它负责把 `config.yaml` 中的 `security.*` 转换为 `serverx.RuntimeProviders` 可消费的 AuthN/AuthZ/Audit/skip-policy runtime。

`serverx` / `middleware/autowire` 仍是唯一的服务端中间件链装配入口。业务服务不应该自行组装 authn/authz/audit middleware。

## 2. Development tooling

以下包是工具，不是 runtime API。业务代码禁止 import：

```text
cmd/kernel
cmd/protoc-gen-go-http
cmd/protoc-gen-go-errors
cmd/protoc-gen-go-authz
cmd/protoc-gen-go-gateway
cmd/protoc-gen-go-deploy
cmd/protoc-gen-go-kernel
cmd/buf-check-aisphere
```

生成项目通过 `kernel-layout` 的 Makefile 或本仓库 `make tools` 安装这些工具。

## 3. External layout

生成服务模板、生成服务 Makefile 行为、deploy manifest 模板、layout 文档和 layout smoke tests 不在 Kernel 仓库内维护，归属独立仓库：

```text
https://github.com/aisphereio/kernel-layout
```

Kernel 的 `cmd/kernel` 只保留 layout 解析和拉取逻辑。默认解析顺序为：

```text
--repo -> KERNEL_LAYOUT -> https://github.com/aisphereio/kernel-layout.git
```

## 4. Examples

`examples/*` 只用于示例和 proto/generator 输入，不是 runtime API。没有明确 README 和运行目标的示例默认不承诺可独立运行。

## 5. 已移除验证树

`validation/*` 已从 Kernel runtime tree 移除。后续验证方式：

```text
1. 独立 validation 仓库
2. 生成项目自己的 tests
3. CI job 中临时生成测试工程
4. 显式 build tag 的外部验证包
```

## 6. 旧入口处理

```text
layout/ -> aisphereio/kernel-layout
validation/ -> external validation / generated service tests
buf.gen.deploy.yaml at Kernel root -> kernel-layout generated-service deploy template
middleware/ratelimit/ -> use ratelimitx
internal/ratelimit/ -> use ratelimitx providers
github.com/aisphereio/kernel/errors -> use errorx
core/httpx/contextx as docs wording -> use serverx/transportx/requestx/accessx
securityx.AuthnConfig -> deprecated alias; use securityx.AuthnBoundaryConfig
grpcgatewayx -> not mainline; use grpc-gateway generator + protoc-gen-go-gateway + gatewayx
```

## 7. Agent 规则

AI Agent 遇到不存在或旧入口时，不允许重新创建同名兼容层。必须改用当前主线包，或先更新本契约文档再实现新抽象。
