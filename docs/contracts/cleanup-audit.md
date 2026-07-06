# Kernel 清理审计记录

本文记录 Kernel 主线代码中已经确认的删除项、废弃项、保留项和本轮安全修复项。后续 AI Agent 或人类开发者做清理时，先看本文，避免把已删除入口重新补回来。

## 1. 已确认处理的问题

| 文件 / 路径 | 发现 | 处理 |
|---|---|---|
| `layout/` | Kernel 仓库内置了一份服务模板，和独立 `aisphereio/kernel-layout` 仓库职责重复，CI 还会把它当作生成项目来源 | 从 Kernel 仓库删除；模板、生成项目 Makefile、layout 文档和 layout smoke tests 只放在 `kernel-layout` |
| `validation/` | 场景验证目录进入默认 `go test ./...` 包图，和 runtime tree 边界不一致 | 从 Kernel 仓库删除；场景验证放到业务仓库、layout 仓库或专用测试面 |
| `.github/workflows/iam-service-demo.yml` | 同时依赖 `validation/iamservice` 和仓库内 `layout/` | 改为 Kernel CLI/tooling smoke，不再引用 `validation/` 和仓库内模板目录 |
| `.github/workflows/governance-demo.yml` | 包含 `cd layout` 的生成项目 compile smoke | 删除 layout compile smoke，只保留 Kernel governance targeted tests |
| root `Makefile` | 混合了 Kernel runtime/tooling 与生成项目 layout 的职责，例如 root `make deploy` | 收敛为 Kernel 仓库自身门禁；生成项目的 `make deploy` 属于 `kernel-layout` |
| root `buf.gen.deploy.yaml` | 让 Kernel 仓库本身看起来承担服务部署清单生成职责 | 删除；deploy manifest 生成能力仍由 `protoc-gen-go-deploy` 提供，但生成项目模板放在 `kernel-layout` |
| `cmd/kernel` layout resolution | 会自动向上搜索本仓库或当前目录中的 `layout/` | 改为仅使用 `--repo`、`KERNEL_LAYOUT` 或默认远程 `https://github.com/aisphereio/kernel-layout.git` |

## 2. 已删除 / 不允许恢复的旧入口

| 路径或概念 | 状态 | 当前替代 |
|---|---:|---|
| Kernel 仓库内置 `layout/` | removed | `aisphereio/kernel-layout` |
| Kernel 仓库内置 `validation/` | removed | service/layout 专用测试面 |
| `middleware/ratelimit/` | removed | `ratelimitx` policy/provider |
| `internal/ratelimit/` | removed | `ratelimitx` providers |
| `github.com/aisphereio/kernel/errors` | removed | `errorx` |
| `core` / `httpx` / `contextx` 作为旧文档总入口 | obsolete wording | `serverx`、`transportx/http`、`requestx`、`accessx`、当前 `contextx` |
| `grpcgatewayx` | not mainline | upstream grpc-gateway generator + `protoc-gen-go-gateway` + `gatewayx` |
| `registryx` 文档写法 | wrong name | `registry` |

## 3. 保留但必须限制使用的包

| 包 | 为什么保留 | 使用边界 |
|---|---|---|
| `securityx` | Gateway 和后端都需要统一把 `config.yaml` 的 `security.*` 构造成 AuthN/AuthZ/Audit runtime | 它不是 middleware 装配层；装配入口仍是 `serverx` / `middleware/autowire` |
| `middleware/*` | 框架内部需要可组合的中间件实现 | 业务不要手写装配链；通过 `serverx` 消费 |
| `authn/casdoor` | Casdoor 是默认 authn provider 适配器 | 业务通过 `authn` 接口或 `securityx` 使用，不直接散落依赖 Casdoor SDK 对象 |
| `authn/oidcx` | Gateway 本地 JWT/JWKS 校验和 external OIDC 场景需要 provider-neutral verifier | 必须配置 issuer/audience/allowed_algs；不能接受任意 OIDC token |
| `authz/spicedb` | SpiceDB 是默认 authz provider 适配器 | 业务通过 `authz` / `accessx` 使用，不直接绕过 guard |
| `restx` / `grpcx` | transport 运行时辅助 | 优先通过 `transportx` / `serverx` 使用，避免业务自行拼底层链路 |

## 4. 当前主线边界

```text
proto contract
  -> buf-check-aisphere
  -> protoc generators
  -> requestx.Info
  -> securityx runtime material
  -> serverx/autowire
  -> gatewayx manifest / deploy HTTPRoute generator
  -> admissionx
  -> business service
  -> errorx negotiated response
```

Kernel 仓库只提供 runtime、contract、generator 和 CLI。生成项目模板、生成项目 Makefile、`make deploy` 示例和模板文档属于 `kernel-layout`。

## 5. 清理规则

1. 发现旧包名时，先检查 `docs/contracts/package-status.md` 和本文，不要新增兼容层。
2. 安全类依赖必须跑 `make vuln` 或 `govulncheck ./...`。
3. 如果某个包只被文档引用、没有主线 API 定位，优先删除文档引用；不要为了旧文档恢复代码。
4. 如果某个包仍被 `serverx` / `middleware/autowire` 消费，不要按“业务不可 import”误删；应在文档中标成 framework assembly/helper。
5. 任何生成器能力变化必须同步 `README.md`、`AGENTS.md`、`docs/contracts/runtime-api-boundary.md` 和 `docs/contracts/package-status.md`。
