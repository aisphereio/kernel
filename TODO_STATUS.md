# Kernel 长任务状态

本文件用于记录当前长任务的推进状态。每次完成较大改动，都必须同步更新。

## 当前目标

把 Kernel 从“能力包集合”收敛成“规范驱动框架”：

```text
contract -> check -> codegen -> requestx.Info -> serverx/autowire -> admissionx -> business -> validation
```

同时把 Kernel 核心代码和验证业务代码分离，文档改成中文主线，过期文档归档。

## 已完成

1. [x] 新增 `requestx.Info`，作为请求元信息中心。
2. [x] 新增 `middleware/requestinfo`，server/client 链路统一注入 request info。
3. [x] 新增 `admissionx`，支持 mutating / validating admission。
4. [x] `middleware/autowire` 接入 requestinfo 和 admission。
5. [x] `middleware/access` 优先读取 `requestx.Info`。
6. [x] `protoc-gen-go-authz` 生成 `RequestInfoResolver`。
7. [x] `bootx.ValidateGovernance` 增加启动治理校验。
8. [x] 新增 `serverx`，收敛 HTTP/gRPC transport、系统路由和治理校验。
9. [x] 新增 GitHub Actions 委托构建文档和 workflow。
10. [x] 新增 IAM + Gateway mock 验证场景。
11. [x] 修复 `serverx` 内 HTTP/gRPC transport 被覆盖的问题。
12. [x] 将 IAM + Gateway 验证代码从 `examples/` 移到 `validation/iamgateway/`，避免和 Kernel 核心边界混在一起。
13. [x] 新增 `docs/architecture/kernel-boundary.md`，明确 Kernel 与验证业务代码边界。
14. [x] 重写 `docs/README.md` 为中文文档入口。
15. [x] 重写 `AGENTS.md` 为中文 Agent 开发规范。
16. [x] 将过期/历史分析文档归档到 `docs/archive/legacy/`。
17. [x] 将关键 contract 文档改成中文：serverx、request-info、admission、rate-limit、downstream-policy、boot-governance-validation。

## 当前验证状态

本地执行：

```bash
go test ./serverx ./validation/iamgateway ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
```

结果：部分基础包通过，但当前沙箱缺少完整离线 Go module proxy，`GOPROXY=off` 下 admissionx/autowire 依赖无法解析。

已记录日志：

```text
/mnt/data/kernel-demo-lab/boundary-targeted-tests.log
```

这不是代码语义失败，而是依赖环境问题。完整构建应走 GitHub Actions：

```text
.github/workflows/governance-demo.yml
```

## 下一步

1. [ ] 将 `serverx.Clients()` 正式做成 client factory，业务不再碰 raw client。
2. [ ] 增加真正的 Gateway demo service，而不仅是 mock middleware 链路。
3. [ ] 增加 IAM demo service，验证 INTERNAL + service-auth + authz/audit。
4. [ ] GitHub Actions 产出 governance-demo 构建日志 artifact。
5. [ ] 必要时让 Actions 产出离线依赖包，回灌到本地验证。

## Casdoor -> Kernel IAM 抽象整改

- [x] 使用 GitHub Actions 拉取 Casdoor 源码学习包。
- [x] 梳理 Casdoor User/Organization/Group/Role/Permission 模型。
- [x] 新增 `iamx` provider-neutral IAM 领域模型。
- [x] 新增 `iamx/memory` 用于 demo 和 mock 集成测试。
- [x] 新增 `iamx/authnadapter`，把 `authn.IdentityAdmin` 映射为 `iamx.Directory`。
- [x] 新增 `validation/iam` 验证 Casdoor 风格组织/多级组/membership。
- [x] 更新中文文档和 AGENTS.md 开发硬规则。
- [ ] 后续补 DB-backed IAM directory。
- [ ] 后续补 Gateway/IAM demo 中的登录、token relay、业务服务 authz 三段验证。

## 方向修正：Casdoor/SpiceDB Provider 化

- [x] 明确业务只依赖 Kernel authn/authz/accessx 接口。
- [x] authn 默认实现收敛为 Casdoor provider。
- [x] authz 生产实现收敛为 SpiceDB provider。
- [x] Casdoor 作为用户、组织、多级组、角色管理后台，不再默认自建 IAM 中心。
- [x] iamx/db 标记为 experimental / 可选同步视图。
- [x] 文档新增 `docs/architecture/authn-authz-provider-boundary.md`。


## IAM 微服务端到端流程

- [x] 使用 `kernel new iam-service --repo ./layout` 生成服务骨架，日志见 `iam-kernel-new.log`。
- [x] 新增 IAM proto 示例：`validation/iamservice/api/iam/v1/iam.proto`。
- [x] 新增 `serverx.RuntimeProviders`，把 authn/authz/accessx/provider 装配收进框架。
- [x] 新增 IAM 微服务验证：未登录 401、无权限 403、有权限 200 + audit。
- [x] 中文文档：`docs/demos/iam-service-kernel-flow.md`。
- [ ] `make api` 完整生成和 full build 如遇沙箱超时/公网依赖，委托 GitHub Actions。

## Gateway/IAM/SkillService 自动路由验证

- [x] 新增 gatewayx Route Manifest / MemoryRegistry / Matcher / StaticHosts / Dispatcher。
- [x] 用 StaticHosts 替代 K8s Service DNS，跑通本地 Gateway -> IAM/SkillService 验证。
- [x] 新增 `validation/platformflow`，覆盖 login public、无 token 401、有 token无权限403、有权限200。
- [x] 更新中文文档和 AGENTS 规则。
- [ ] 后续补 `gatewayx.EtcdRegistry` 和 `protoc-gen-go-gateway` 生成 Route Manifest。

### Gateway Route Manifest / Registry 推进

- [x] 新增 `cmd/protoc-gen-go-gateway`，从 `google.api.http + aisphere.access.v1.policy` 生成 `<Service>GatewayManifest()`。
- [x] 新增 `gatewayx.KVStore` / `gatewayx.EtcdRegistry`，用 etcd-shaped KV 接口承载 route registry；本地使用 `MemoryKVStore` 模拟。
- [x] 新增 `serverx.RegisterGatewayRoutes`，服务启动/部署阶段统一注册 Gateway Manifest。
- [x] `validation/platformflow` 改为 `EtcdRegistry + MemoryKVStore + generated-equivalent manifest/resolver`，不再把 route table 写死在测试主体里。
- [x] 保留 `StaticHosts` 替代 K8s Service DNS，暂不切换真实 K8s Service。
- [x] 明确 authn/authz 完整链路：Gateway passive token relay；SkillService 内部执行 authn provider、authz provider、accessx audit。
- [ ] 后续把 `validation/platformflow` 的 generated-equivalent 文件替换为真实 `make api` 生成产物。
- [ ] 后续为 `EtcdRegistry` 接入真实 etcd clientv3 adapter 或统一 `etcdx.Store`。


## Gateway gRPC Invoker 进度

- [x] 增加 gatewayx.InvokerRegistry。
- [x] 增加 gatewayx.UnaryInvoker / GRPCUnaryInvoker。
- [x] protoc-gen-go-gateway 增强为生成 Manifest + Binding + RegisterInvokers。
- [x] validation/platformflow 改为 generated-equivalent invoker registry。
- [ ] 下一步接真实 gRPC bufconn / ClientConn 验证。
- [ ] 下一步补 Gateway /gateway/routes snapshot 系统路由。


## 已完成：真实 gRPC 与配置化 transport

- serverx 新增配置加载：`serverx.LoadConfigFile` / `DecodeFileConfig`。
- 服务端 HTTP/gRPC 可通过 `server.http.enabled`、`server.grpc.enabled` 开关控制。
- 增加 IAM/Skill/Gateway 配置样例。
- platformflow 增加 bufconn 真实 gRPC client/server 链路验证。
- Gateway 到内部服务主路径明确为 generated gRPC client。

## ServiceModule autoload

- [x] 新增 `serverx.ServiceModule`，承载生成式服务元信息。
- [x] 新增 `serverx.BuildServiceFromFactory` / `ServiceDeps`，根据 config + module + provider factory 自动装配 middleware/transport，并把 DB/Data/Providers 注入业务。
- [x] 新增 `protoc-gen-go-kernel` 生成 `<Service>KernelModule()`；`protoc-gen-go-gateway` 回归只生成 Gateway manifest/binding/invoker。
- [x] validation/platformflow 改用 `RegisterServiceGatewayRoutes` 和 generated-equivalent KernelModule。
- [ ] 在公网 GitHub Actions 中跑完整 `kernel new -> make api`，用真实生成物替换 validation 手写等价代码。

## DBX 数据开发范式

- [x] 明确不把 SQL 放进 proto，SQL migration 是数据库结构唯一真实来源。
- [x] 新增 `migrationx`，支持 goose-compatible SQL 目录发现、checksum、apply/validate 模式。
- [x] `serverx.Config` 增加 `database` 配置，支持自动打开 dbx.DB、执行/校验 migration、应用关闭时释放连接。
- [x] 新增 `dbrepo.ResourceRepository[T]`，提供 tenant/owner/soft-delete/timestamps/filter/sort/pagination 的傻瓜式 CRUD；补 fail-closed 和 patch 字段保护。
- [x] 增加 `validation/dbflow` migration 示例和文档。
- [ ] 后续补 `kernel db migration create/up/status/down` CLI。
- [ ] 后续补真实 PG/MySQL 集成验证，优先在 GitHub Actions 公网环境执行。
- [ ] 后续补从表结构生成 Row/Repo skeleton 的 `kernel db inspect`。
