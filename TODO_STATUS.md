# Kernel 长任务状态

本文件记录当前主线状态。每次完成较大改动，都必须同步更新。

## 当前目标

把 Kernel 从“能力包集合”收敛成“规范驱动框架”：

```text
kernel new
  -> proto contract
  -> check
  -> codegen
  -> generated ServiceModule
  -> serverx/autowire
  -> authn/authz/accessx/audit
  -> dbx/migrationx/dbrepo
  -> admissionx
  -> business handler
  -> generated project / CI validation
```

Kernel runtime tree 只保留框架能力。场景验证、generated-shape 实验、IAM/Gateway/SkillService 联调验证，不再放入 Kernel 默认包图。

## 已完成主线

1. [x] `requestx.Info` 成为请求元信息中心。
2. [x] `middleware/requestinfo` 在 server/client 链路统一注入 request info。
3. [x] `admissionx` 支持 mutating / validating admission。
4. [x] `middleware/autowire` 接入 requestinfo、authn、access、admission、ratelimitx、client policy middleware。
5. [x] `middleware/access` 优先读取 `requestx.Info`。
6. [x] `protoc-gen-go-authz` 生成 `RequestInfoResolver`。
7. [x] `bootx.ValidateGovernance` 支持启动治理校验。
8. [x] `serverx` 收敛 HTTP/gRPC transport、系统路由、治理校验和运行时 provider 装配。
9. [x] `serverx.BuildServiceFromFactory + ServiceDeps` 将 DB/Data/Providers 真正注入业务实例。
10. [x] `cmd/protoc-gen-go-gateway` 生成 Gateway Manifest / Binding / Invoker 注册。
11. [x] `cmd/protoc-gen-go-kernel` 独立生成 `<Service>KernelModule()`，不再把 ServiceModule 职责塞给 gateway generator。
12. [x] `cmd/buf-check-aisphere` 执行 proto contract 检查。
13. [x] `gatewayx` 支持 Route Manifest、MemoryRegistry、KVStore/EtcdRegistry、Matcher、Dispatcher、InvokerRegistry、gRPC invoker 辅助。
14. [x] `ratelimitx` 成为限流唯一主线；旧 `middleware/ratelimit` 不再保留兼容入口。
15. [x] `dbx + migrationx + dbrepo` 形成数据开发范式：SQL migration 是数据库结构来源，proto 不承载 SQL。
16. [x] `migrationx` 默认接入真实 `pressly/goose` provider；Kernel 自己的 `kernel_sql` parser 仅保留 legacy/test，不作为生产复杂 SQL 引擎。
17. [x] `dbrepo` tenant/owner scope fail-closed，Patch 保护字段/白名单生效，GORM/driver error 经 `dbx` 归一化后再进入业务边界。
18. [x] 无明确 migration locker 前，`replicas > 1 + apply/dev_apply` 强制 fail-closed。
19. [x] `make proto-check` 只检查 canonical `api/` contract，兼容 wrapper/examples/testdata 不再污染正式发布门禁。
20. [x] Goose 依赖和 `go.mod/go.sum` 已正式提交；CI 通过 `go mod tidy + git diff --exit-code` 阻止模块依赖漂移。
21. [x] README、docs/README、AGENTS、package-status、runtime-api-boundary 改为中文主线。
22. [x] `validation/` 从 runtime API 和 CI 默认包图移除。
23. [x] 真实 `kernel new -> make api -> make verify` 生成项目成为 release gate，不再只依赖 generated-equivalent demo。
24. [x] `serverx` 对缺失 generated gRPC/HTTP registrar fail-closed，避免服务启动但业务 RPC 未注册。

## 2026-08-13 框架边界硬化验收

PR：`#39 Harden framework boundaries and DB migration path`

专用 hardening gate 已真实通过：

```text
Go 1.26.4
module graph committed / no tidy drift
DB runtime targeted tests
serverx tests
gateway/authz/http/kernel generator tests
make tools
make api
make proto-check
generated contract compile
kernel new boundary-smoke
生成项目 make tools-local
生成项目 make verify
source artifact packaging
```

同时通过现有：

```text
verify
dbflow
platform-real-grpc
governance-demo
kernel-cli-smoke
taskx
```

真实生成项目 gate 同时发现并修复了 `kernel-layout` 中的 proto contract 漂移、废弃 config generator、业务测试旧生成类型和提交 generated artifacts 不同步问题。Kernel #39 联调期间只对 PR #39 使用 companion layout branch；后续 Kernel PR 自动回到 `kernel-layout/main`，不留下长期 feature-branch 依赖。

## 当前验证方式

根仓库门禁：

```bash
make tools
make api
make proto-check
make verify
```

数据/边界 targeted tests：

```bash
go test ./dbx ./dbrepo ./migrationx ./serverx
go test ./cmd/protoc-gen-go-gateway ./cmd/protoc-gen-go-kernel ./cmd/protoc-gen-go-authz ./cmd/protoc-gen-go-http ./cmd/buf-check-aisphere
```

治理链路 targeted tests：

```bash
go test ./gatewayx ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
```

## 当前边界规则

- 业务代码通过 `serverx` 启动服务，不手写 transport glue。
- 业务实例通过 `ServiceFactory/ServiceDeps` 获得 DB/Data/Providers，不在 handler 内临时打开基础设施连接。
- 业务代码通过 `requestx.Info` 获取操作、资源、租户、调用方信息，不解析 raw path / raw grpc method。
- 业务代码通过 `accessx/authn/authz/auditx` 执行认证、授权和审计，不直接依赖 Casdoor、SpiceDB、Casbin、OPA SDK。
- Gateway 只做边界路由、边界准入和 token relay；资源级授权在业务服务内执行。
- 限流只走 `ratelimitx` policy/provider，不恢复旧 `middleware/ratelimit`。
- 服务间调用必须走 Kernel client governance chain，不直接裸用 `grpc.Dial`、`http.Client`、手写 retry/breaker/limiter。
- SQL migration 是 schema 的唯一真实来源；生产 migration 默认委托真实 goose。
- tenant/owner scoped repository 缺 scope 必须 fail-closed。
- `api/` 是 canonical proto contract gate；examples/testdata 不定义公共契约边界。
- `registry` 是当前服务注册发现 runtime 包名，不再在文档里写成 `registryx`。
- 场景验证进入生成项目 tests、独立验证仓库、显式 build tag 或 GitHub Actions 专用 job，不回流到 Kernel runtime tree。

## 下一步（P1/P2，不再混入本轮 P0）

1. [ ] 完成 `serverx.Clients()` 正式 client factory，业务不再碰 raw client。
2. [ ] 为 Kernel Gateway 增加 route watcher + immutable/atomic route snapshot，避免请求热路径重复构建 matcher。
3. [ ] 为 Gateway 补 `/gateway/routes`、`/gateway/snapshot`、reload status 系统路由。
4. [ ] 为 `gatewayx.EtcdRegistry` 接入真实 etcd clientv3 adapter 或统一 `etcdx.Store`。
5. [ ] 补 IAM demo service 的登录、token relay、service-auth、authz、audit 三段真实联调验证。
6. [ ] 补 `kernel db migration create/up/status/down` CLI。
7. [ ] 为生产 migration 设计显式 `MigrationLocker` contract；在实现 PG/MySQL 锁后再允许多副本 startup apply。
8. [ ] 补真实 PG/MySQL migration + CRUD 集成验证，优先走 GitHub Actions/Testcontainers。
9. [ ] 补 `kernel db inspect`，从表结构生成 Row/Repo skeleton。
10. [ ] 把默认 layout 的 cache/objectstore/DTM/logger/metrics 进一步收敛为统一 RuntimeDeps/ResourceFactory，使 `cmd/server/main.go` 更薄。
