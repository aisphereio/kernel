# Kernel 清理审计记录

本文记录 Kernel 主线代码中已经确认的删除项、废弃项、保留项和本轮安全修复项。后续 AI Agent 或人类开发者做清理时，先看本文，避免把已删除入口重新补回来。

## 1. 本轮必须处理的问题

| 文件 | 发现 | 处理 |
|---|---|---|
| `go.mod` | `github.com/golang-jwt/jwt/v4@v4.5.0` 命中 GO-2025-3553 / GO-2024-3250；代码直接 import 但被标成 `// indirect` | 升级到 `v4.5.2`，移动为直接依赖 |
| `authn/oidcx/verifier.go` | `Parser.ParseWithClaims` 调用缺少 parser-level signing method 白名单 | 改为 `jwt.NewParser(jwt.WithoutClaimsValidation(), jwt.WithValidMethods(...))`，并保留手动 issuer/audience/time/owner 校验 |
| `authn/casdoor/token.go` | `jwt.ParseWithClaims` 直接调用；Keyfunc 内部虽校验 alg，但 parser 层没有白名单 | 改为 `jwt.NewParser(jwt.WithValidMethods(...))`，统一 `allowedJWTAlgs()` |
| `authn/casdoor/token.go` | `Principal.Attributes` 中保存原始 `access_token`，可能进入日志、审计或下游上下文 | 删除该字段；token 只通过 `authn.TokenSet` 返回，不进入 Principal |
| `securityx/config.go` | `AuthnConfig` 只是旧别名，容易被新代码继续使用 | 标记 `Deprecated`，新代码必须使用 `AuthnBoundaryConfig` |
| `README.md` / `doc.go` / `AGENTS.md` / `docs/contracts/*.md` | runtime API 边界没有同步 `securityx`、`bootx`、`contextx` 的当前定位 | 同步 runtime API 表和废弃入口说明 |

## 2. 已删除 / 不允许恢复的旧入口

| 路径或概念 | 状态 | 当前替代 |
|---|---:|---|
| `validation/` | removed | 独立 validation 仓库、生成项目自己的 tests、显式 build tag 或 GitHub Actions 专用 job |
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
  -> admissionx
  -> business service
  -> errorx negotiated response
```

业务服务只写领域逻辑。认证、授权、审计、限流、重试、熔断、超时、错误协议转换、Gateway route manifest 和 deploy route manifest 都应由 Kernel 提供。

## 5. 清理规则

1. 发现旧包名时，先检查 `docs/contracts/package-status.md` 和本文，不要新增兼容层。
2. 安全类依赖必须跑 `make vuln` 或 `govulncheck ./...`。
3. 如果某个包只被文档引用、没有主线 API 定位，优先删除文档引用；不要为了旧文档恢复代码。
4. 如果某个包仍被 `serverx` / `middleware/autowire` 消费，不要按“业务不可 import”误删；应在文档中标成 framework assembly/helper。
5. 任何生成器能力变化必须同步 `README.md`、`AGENTS.md`、`docs/contracts/runtime-api-boundary.md` 和 `docs/contracts/package-status.md`。
