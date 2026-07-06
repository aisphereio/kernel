# Runtime API 边界

Kernel 仓库分为四类表面：runtime API、development tooling、layout、external validation。业务代码只能依赖 runtime API；服务 boot 代码可以依赖 runtime API 中的装配类包，例如 `serverx`、`securityx`、`bootx`。

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

生成项目通过 `make tools` 安装这些工具。

## 3. Layout

`layout/` 和独立 `kernel-layout` 仓库用于表达推荐工程结构，不是 runtime API。

layout 可以包含示例业务代码，但 Kernel 核心包不能反向依赖 layout。

## 4. External validation

场景检查、generated-shape 实验、IAM/Gateway/SkillService 联调验证，不进入 Kernel 主模块默认包图。

后续验证方式：

```text
1. 独立 validation 仓库
2. 生成项目自己的 tests
3. 显式 build tag
4. GitHub Actions 专用 job
```

## 5. 旧入口处理

```text
middleware/ratelimit/       -> use ratelimitx
internal/ratelimit/         -> use ratelimitx providers
github.com/aisphereio/kernel/errors -> use errorx
core/httpx/contextx as docs wording -> use serverx/transportx/requestx/accessx
securityx.AuthnConfig -> deprecated alias; use securityx.AuthnBoundaryConfig
grpcgatewayx -> not mainline; use grpc-gateway generator + protoc-gen-go-gateway + gatewayx
```

## 6. Agent 规则

AI Agent 遇到不存在或旧入口时，不允许重新创建同名兼容层。必须改用当前主线包，或先更新本契约文档再实现新抽象。
