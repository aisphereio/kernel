# Aisphere Kernel

Aisphere Kernel 是 Aisphere 项目的规范驱动微服务基础框架。

核心原则：业务声明契约，Kernel 负责检查、生成、装配、治理和验证。业务项目优先写 proto contract 和领域逻辑，不手写 transport glue、错误协议转换、访问控制、审计、限流或 Gateway 分发 glue。

## Quick start

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel version
kernel new todo-service
```

`kernel new` 默认从独立仓库 `https://github.com/aisphereio/kernel-layout.git` 拉取生成服务模板。需要本地或私有模板时使用 `--repo` 或 `KERNEL_LAYOUT`。

Kernel 仓库自身验证：

```bash
make tools
make api
make proto-check
make verify
```

生成服务里的 `make deploy`、layout smoke test、deploy 模板和业务工程 Makefile 归属 `aisphereio/kernel-layout`，不再放在 Kernel 仓库内。

## Package index

新业务、生成代码和 AI Agent 先看 [PACKAGE_INDEX.md](PACKAGE_INDEX.md)。它是当前包边界、状态和替代路径的总入口。

| 问题 | 结论 |
|---|---|
| `authz` vs `accessx` | `authz` 是 provider contract；`accessx` 是 request-time guard/orchestrator |
| `authn.IdentityAdmin` vs `iamx.Directory` | `IdentityAdmin` 管外部 IdP 投影；`Directory` 管 Kernel IAM 控制面事实 |
| `resourcex` vs `authz` | `resourcex.Grant` 是控制面事实；`authz.Relationship` 是查询投影 |
| `taskx` vs 持久化任务队列 | `taskx` 管周期 reconciliation；大量离散异步任务使用 Asynq/River provider 或独立 worker |
| `layout/` | 已移到 `aisphereio/kernel-layout` |
| `validation/` | 已从 runtime tree 移除；场景验证应放到独立验证仓库、CI 临时生成目录或 generated service tests |

## Runtime API

业务代码、服务 boot 代码和生成代码可以 import：

```text
errorx logx configx metricsx serverx securityx bootx contextx
transportx/http transportx/grpc
requestx accessx authn authz auditx
gatewayx admissionx ratelimitx clientpolicyx
dbx cachex objectstorex dtmx taskx
selectorx registry encodingx
```

`securityx` 负责把 `config.yaml` 的安全配置构造成 provider-neutral runtime，不是新的 middleware 装配层；中间件装配仍统一走 `serverx` / `middleware/autowire`。

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

生成项目通过 `make tools` 或 layout 仓库的 Makefile 安装工具链。

## External layout

Kernel runtime、proto options、generators、contract checkers 和 CLI 行为留在本仓库。生成服务模板、生成服务 Makefile 行为、deploy manifest 模板、layout 文档和 layout smoke tests 归属独立仓库：

```text
https://github.com/aisphereio/kernel-layout
```

## Removed / not mainline

```text
layout/                     moved to aisphereio/kernel-layout
validation/                 removed from Kernel runtime tree
buf.gen.deploy.yaml         removed from Kernel root; generated-service deploy template belongs to kernel-layout
middleware/ratelimit/       removed; use ratelimitx
internal/ratelimit/         removed; use ratelimitx providers
github.com/aisphereio/kernel/errors removed; use errorx
third_party/errors/errors.proto removed; orphan proto
core/httpx/contextx as main docs wording obsolete; use serverx/transportx/requestx/accessx
securityx.AuthnConfig       deprecated alias; use securityx.AuthnBoundaryConfig
grpcgatewayx                not mainline; use grpc-gateway generator, protoc-gen-go-gateway and gatewayx
aisphere/access/v1/access.proto compat wrapper; new proto imports api/aisphere/access/v1/access.proto
```

## Proto-first development

```text
proto contract
  -> buf-check-aisphere
  -> protoc generators
  -> requestx.Info / accessx / gatewayx / serverx
  -> business service implementation
```

`cmd/protoc-gen-go-deploy` 仍保留在 Kernel，作为生成器能力；生成服务如何调用 deploy generation 由 `kernel-layout` 的 Makefile 和模板负责。

## Documentation

- [PACKAGE_INDEX.md](PACKAGE_INDEX.md)
- [docs/README.md](docs/README.md)
- [docs/getting-started.md](docs/getting-started.md)
- [AGENTS.md](AGENTS.md)
- [docs/contracts/package-status.md](docs/contracts/package-status.md)
- [docs/contracts/runtime-api-boundary.md](docs/contracts/runtime-api-boundary.md)
- [docs/contracts/proto-capability-matrix.md](docs/contracts/proto-capability-matrix.md)
- [docs/contracts/cleanup-audit.md](docs/contracts/cleanup-audit.md)
- [docs/contracts/taskx.md](docs/contracts/taskx.md)
