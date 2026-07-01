# Kernel 长任务状态

本文件记录当前主线状态。每次完成较大改动，都必须同步更新。

## 当前目标

把 Kernel 从“能力包集合”收敛成“规范驱动框架”：

```text
contract -> check -> codegen -> requestx.Info -> serverx/autowire -> admissionx -> business -> generated project tests / external validation
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
9. [x] `cmd/protoc-gen-go-gateway` 生成 Gateway Manifest / Binding / Invoker 注册。
10. [x] `cmd/protoc-gen-go-kernel` 生成 `<Service>KernelModule()`。
11. [x] `cmd/buf-check-aisphere` 执行 proto contract 检查。
12. [x] `gatewayx` 支持 Route Manifest、MemoryRegistry、KVStore/EtcdRegistry、Matcher、Dispatcher、InvokerRegistry、gRPC invoker 辅助。
13. [x] `ratelimitx` 成为限流唯一主线；旧 `middleware/ratelimit` 不再保留兼容入口。
14. [x] `dbx + migrationx + dbrepo` 形成数据开发范式：SQL migration 是数据库结构来源，proto 不承载 SQL。
15. [x] README、docs/README、AGENTS、package-status、runtime-api-boundary 改为中文主线。
16. [x] `validation/` 从 runtime API 和 CI 默认包图移除。

## 当前验证方式

根仓库本地门禁：

```bash
make tools
make api
make proto-check
make verify
```

治理链路 targeted tests：

```bash
go test ./cmd/protoc-gen-go-gateway ./cmd/protoc-gen-go-kernel ./cmd/buf-check-aisphere ./gatewayx ./serverx ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
```

生成项目 / layout 编译验证：

```bash
cd layout
go test ./api/todo/v1 -run TestNonExistent -count=0
go test ./cmd/fullflow-smoke -run TestNonExistent -count=0
```

CI 入口：

```text
.github/workflows/governance-demo.yml
```

## 当前边界规则

- 业务代码通过 `serverx` 启动服务，不手写 transport glue。
- 业务代码通过 `requestx.Info` 获取操作、资源、租户、调用方信息，不解析 raw path / raw grpc method。
- 业务代码通过 `accessx/authn/authz/auditx` 执行认证、授权和审计，不直接依赖 Casdoor、SpiceDB、Casbin、OPA SDK。
- Gateway 只做边界路由、边界准入和 token relay；资源级授权在业务服务内执行。
- 限流只走 `ratelimitx` policy/provider，不恢复旧 `middleware/ratelimit`。
- 服务间调用必须走 Kernel client governance chain，不直接裸用 `grpc.Dial`、`http.Client`、手写 retry/breaker/limiter。
- `registry` 是当前服务注册发现 runtime 包名，不再在文档里写成 `registryx`。
- 场景验证进入生成项目 tests、独立验证仓库、显式 build tag 或 GitHub Actions 专用 job，不回流到 Kernel runtime tree。

## 下一步

1. [ ] 完成 `serverx.Clients()` 正式 client factory，业务不再碰 raw client。
2. [ ] 用真实 `kernel new -> make api -> make verify` 生成物替换手写等价 demo 代码。
3. [ ] 为 Gateway 补 `/gateway/routes` snapshot 系统路由。
4. [ ] 为 `gatewayx.EtcdRegistry` 接入真实 etcd clientv3 adapter 或统一 `etcdx.Store`。
5. [ ] 补 IAM demo service 的登录、token relay、service-auth、authz、audit 三段验证。
6. [ ] 补 `kernel db migration create/up/status/down` CLI。
7. [ ] 补真实 PG/MySQL 集成验证，优先走 GitHub Actions。
8. [ ] 补 `kernel db inspect`，从表结构生成 Row/Repo skeleton。
