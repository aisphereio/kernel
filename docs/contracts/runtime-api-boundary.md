# Runtime API 边界

Kernel 仓库分为三类表面：runtime API、development tooling、external layout repository。业务代码只能依赖 runtime API；服务 boot 代码可以依赖 runtime API 中的装配类包，例如 `serverx`、`securityx`、`bootx`。

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

`cmd/kernel` 默认从独立 `https://github.com/aisphereio/kernel-layout.git` 拉取服务模板。Kernel 仓库不再内置服务模板目录。

## 3. External layout repository

服务模板、生成项目 Makefile、生成项目 `make deploy`、layout 文档、layout smoke tests 都属于独立仓库：

```text
https://github.com/aisphereio/kernel-layout
```

Kernel 仓库只保留 runtime 包、proto option、代码生成器和 CLI。模板能力不要复制回 Kernel 仓库。

## 4. Removed / not mainline

```text
layout tree in Kernel repo     -> moved to aisphereio/kernel-layout
validation tree in Kernel repo -> removed; scenario validation belongs to service/layout test surfaces
middleware/ratelimit/          -> use ratelimitx
internal/ratelimit/            -> use ratelimitx providers
github.com/aisphereio/kernel/errors -> use errorx
core/httpx/contextx as docs wording -> use serverx/transportx/requestx/accessx
securityx.AuthnConfig -> deprecated alias; use securityx.AuthnBoundaryConfig
grpcgatewayx -> not mainline; use grpc-gateway generator + protoc-gen-go-gateway + gatewayx
```

## 5. Agent 规则

AI Agent 遇到不存在或旧入口时，不允许重新创建同名兼容层。必须改用当前主线包，或先更新本契约文档再实现新抽象。
