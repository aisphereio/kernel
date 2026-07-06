# Aisphere Kernel

Aisphere Kernel 是 Aisphere 项目的规范驱动微服务基础框架。

核心原则：业务声明契约，Kernel 负责检查、生成、装配、治理和验证。业务项目优先写 proto contract 和领域逻辑，不手写 transport glue、错误协议转换、访问控制、审计、限流、Gateway 分发或部署路由清单。

## Quick start

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel version
kernel new todo-service
cd todo-service
make tools
make api
make deploy
make proto-check
make verify
make run
```

最小可运行服务：

```bash
kernel new todo-service --mvp
```

裁剪能力：

```bash
kernel new todo-service --disable iam,gateway,dtmx
```

`--repo` 只作为高级覆盖项，用于本地模板、私有 layout 或测试 layout 分支。

## Runtime API

业务代码和生成代码可以 import：

```text
errorx logx configx metricsx serverx
transportx/http transportx/grpc
requestx accessx authn authz auditx
gatewayx admissionx ratelimitx clientpolicyx
dbx cachex objectstorex dtmx
selectorx registry encodingx
```

## Development tooling

这些是工具，不是 runtime API。业务代码不能 import：

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

生成项目通过 `make tools` 安装工具链。

## Removed / not mainline

```text
validation/                 removed; scenario checks must not live in runtime tree
middleware/ratelimit/       removed; use ratelimitx
internal/ratelimit/         removed; use ratelimitx providers
github.com/aisphereio/kernel/errors removed; use errorx
core/httpx/contextx as main docs wording obsolete; use serverx/transportx/requestx/accessx
grpcgatewayx                not mainline; use grpc-gateway generator, protoc-gen-go-gateway and gatewayx
```

## Proto-first development

```text
proto contract
  -> buf-check-aisphere
  -> protoc generators
  -> requestx.Info / accessx / gatewayx / deploy HTTPRoute manifests / serverx
  -> business service implementation
```

Full profile 的外部 RPC 必须声明 `google.api.http` 和 `aisphere.access.v1.policy`。`--mvp` 只用于最小可运行骨架。

## Deploy route generation

`protoc-gen-go-deploy` 从 `google.api.http` 和 `aisphere.access.v1.policy` 生成 Kubernetes Gateway API `HTTPRoute` 清单。生成文件按路由暴露等级写入：

```text
deploy/generated/gateway/public/          PUBLIC 路由
deploy/generated/gateway/authenticated/   AUTHENTICATED / AUTHORIZED 路由
deploy/generated/gateway/internal/        INTERNAL / SYSTEM 路由
```

生成器会把 upstream gRPC operation、暴露等级、鉴权模式和授权资源声明写入 `RequestHeaderModifier`，由 Gateway Controller 或边缘鉴权中间件消费。业务服务只维护 proto contract，不手写 Gateway 路由 YAML。

## Verify

```bash
make tools
make api
make test
make test-cmd
make vet
```

完整本地门禁：

```bash
make verify
```

## Documentation

- [docs/getting-started.md](docs/getting-started.md)
- [docs/README.md](docs/README.md)
- [AGENTS.md](AGENTS.md)
- [docs/contracts/package-status.md](docs/contracts/package-status.md)
- [docs/contracts/runtime-api-boundary.md](docs/contracts/runtime-api-boundary.md)
- [docs/contracts/proto-capability-matrix.md](docs/contracts/proto-capability-matrix.md)
