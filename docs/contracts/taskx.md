# taskx 后台任务与运行时任务中心契约

## 1. 定位

`taskx` 提供两层能力：

1. **Managed Runtime**：由外部运行时持久化 Job、协调多副本并通过 gRPC 回调业务服务。生产默认 provider 是 `taskx/dapr`。
2. **Local Scheduler**：进程内周期调度、Redis lease、重试和指标。仅用于单机、本地开发、测试和无法部署 Dapr 的降级场景。

业务代码只依赖 `taskx.Runtime`、`taskx.ManagedJob` 和 `taskx.EventHandler`，不得直接依赖 Dapr SDK 类型。

典型场景：

- Grant、订阅、租约、会话等到期回收；
- 数据库控制面事实与 SpiceDB、搜索索引等外部投影之间的 reconciliation；
- 周期清理、补偿和健康巡检；
- 延时执行的一次性任务；
- 需要跨服务状态机、等待、补偿或人工审批时，升级为 Dapr Workflow，而不是把逻辑继续堆进单个 Job handler。

所有 provider 均按 **at-least-once** 设计，Handler 必须幂等。

## 2. 为什么生产主线选择 Dapr Jobs

Dapr Jobs 由独立 Scheduler control-plane 服务管理：

- Job 定义持久化在 Scheduler 的 etcd；
- Scheduler 多副本之间分配触发事件，不依赖 IAM 自己选主；
- 同一 app-id 的 Job 到期后，只会路由到其中一个可用 sidecar/应用副本；
- 应用暂时不可用时，Job 会进入 Scheduler staging queue，待 sidecar 可用后再次投递；
- 支持 schedule、due time、repeats、TTL、overwrite 和失败重试策略；
- 管理面可通过 Dapr CLI list/get/delete/export/import；
- gRPC 是生产推荐协议，避免 HTTP JSON 编解码开销。

Dapr Jobs 是“调度中心”，不是执行器。真正的数据库扫描、SpiceDB 删除和审计写入仍在 IAM handler 内执行。

## 3. 方案对比

| 方案 | 持久化与 HA | 任务执行模型 | 适用范围 | Kernel 结论 |
|---|---|---|---|---|
| `time.Ticker` / `robfig/cron` | 无 | 当前进程执行 | 单机脚本 | 不进入生产主线 |
| Kernel local scheduler + Redis lease | 调度定义不持久化；Redis 只做互斥 | 当前服务副本执行 | 本地、测试、降级 | 保留为 local provider |
| `go-co-op/gocron` | 依赖外部 elector/locker | 当前进程执行 | 进程内 cron | 不作为运行时中心 |
| Asynq | Redis 队列持久化，worker 拉取 | 离散 payload 队列 | 邮件、转码、文件处理 | 未来可作为 queue provider，不替代 Jobs |
| River | PostgreSQL 持久化，可事务入队 | worker 拉取 | PG 主导的离散异步任务 | 可选 provider，不强制所有服务绑定 PG |
| **Dapr Jobs** | Scheduler + etcd，K8s HA | Scheduler 到期后 gRPC 回调一个 app 副本 | 周期任务、一次性延时任务、reconciliation | **生产默认** |
| Dapr Workflow | 状态持久化、长时间运行、恢复与补偿 | Workflow/Activity | 跨服务业务流程 | 复杂流程升级目标 |

## 4. Provider-neutral API

### 4.1 Runtime

```go
type Runtime interface {
    Schedule(context.Context, ManagedJob) error
    Get(context.Context, string) (ManagedJob, error)
    Delete(context.Context, string) error
    RegisterHandler(string, EventHandler) error
}
```

### 4.2 ManagedJob

```go
type ManagedJob struct {
    Name          string
    Schedule      string
    DueTime       string
    Repeats       *uint32
    TTL           string
    Data          []byte
    DataTypeURL   string
    Overwrite     bool
    FailurePolicy *DeliveryFailurePolicy
}
```

Kernel 采用的 canonical schedule 格式与 Dapr Jobs 一致：

- `@every 5m`
- `@hourly` / `@daily` / `@weekly` / `@monthly` / `@yearly`
- 六字段 cron：`秒 分 时 日 月 星期`
- `DueTime`、`TTL` 支持 RFC3339 或 Go duration 字符串

### 4.3 Delivery failure policy

Delivery policy 只描述“Dapr Scheduler 向应用回调失败后的投递重试”，不等于业务内部逐条 Grant 重试。

- `constant`：固定间隔重试，可配置 max retries；
- `drop`：首次回调失败后放弃；
- 未设置时使用 Dapr 运行时默认策略。

安全敏感任务不建议使用 `drop`。

## 5. gRPC 集成模式

Dapr sidecar 通过应用 gRPC `AppCallback` 服务投递 Job。Kernel 提供两种模式。

### 5.1 独立 callback Server：生产默认

IAM 主 gRPC Server 通常带有用户认证、内部调用认证和资源授权中间件。Dapr sidecar callback 不应伪装成普通业务调用，也不应因为缺少用户 JWT 被拒绝。因此默认使用独立 callback 端口：

```go
runtime, callbackServer, err := taskxdapr.NewStandalone(":9001")
if err != nil {
    return err
}
defer runtime.Close()

// callbackServer implements transportx.Server.
app := kernel.New(
    kernel.Server(httpServer, grpcServer, callbackServer),
)
```

该模式下：

- `DAPR_GRPC_ENDPOINT` 或 `DAPR_GRPC_PORT` 用于 IAM 调用本 Pod sidecar；
- `:9001` 用于 sidecar 回调 IAM；
- callback Server 由 Kernel 与 HTTP/gRPC Server 一起启动、优雅停止；
- Dapr SDK 使用 `APP_API_TOKEN` / `dapr-api-token` 验证 sidecar callback；
- NetworkPolicy 只允许同 Pod sidecar 或本地回环访问 callback 端口。

### 5.2 挂载现有 Kernel gRPC Server：可选

只有当服务的全局中间件明确允许 Dapr callback 方法，并且已配置可信 sidecar 校验时，才使用同端口挂载：

```go
runtime, err := taskxdapr.AttachTransport(grpcServer)
if err != nil {
    return err
}
defer runtime.Close()
```

`AttachTransport` 把 Dapr AppCallback 服务注册到 `transportx/grpc.Server` 内嵌的 `grpc.Server`。必须在 `Start/Serve` 之前调用；同一个 gRPC Server 只能注册一次 Dapr AppCallback。

禁止为了省一个端口而直接绕过 IAM 全局认证。无法证明 callback 中间件策略正确时，使用独立 callback Server。

## 6. GRANT-006 推荐实现

### 6.1 Boot 注册

```go
const grantExpirationJob = "grant-expiration-reconciler"

runtime, callbackServer, err := taskxdapr.NewStandalone(":9001")
if err != nil {
    return err
}
defer runtime.Close()

err = runtime.RegisterHandler(grantExpirationJob,
    func(ctx context.Context, event taskx.TriggerEvent) error {
        return grantService.ExpireDueGrants(ctx)
    },
)
if err != nil {
    return err
}

maxRetries := uint32(5)
err = runtime.Schedule(ctx, taskx.ManagedJob{
    Name:        grantExpirationJob,
    Schedule:    "@every 5m",
    Overwrite:   true,
    Data:        []byte(`{"batch_size":100}`),
    DataTypeURL: "application/json",
    FailurePolicy: &taskx.DeliveryFailurePolicy{
        Mode:       taskx.DeliveryFailureConstant,
        MaxRetries: &maxRetries,
        Interval:   5 * time.Second,
    },
})
if err != nil {
    return err
}

app := kernel.New(
    kernel.Server(httpServer, grpcServer, callbackServer),
)
```

所有 IAM 副本都可以在启动时用同一个 name 注册 Job，并设置 `Overwrite=true`。Scheduler 只保存一份 app-id/name 对应的定义，触发时只选择一个 IAM 副本。

### 6.2 Handler 内部状态机

Dapr 解决的是调度持久化和回调分发，不能替代数据库级幂等。Grant handler 必须：

1. 分页查找 `expires_at <= now AND status IN (active, expiring)`；
2. 通过行锁、claim token 或条件更新把一批 Grant 原子推进到 `expiring`；
3. 删除 SpiceDB relationship；relationship 不存在视为成功；
4. 更新 Grant 为 `expired`；
5. 写审计记录；
6. 单条失败保留可重试状态，不阻断整批；
7. handler 只有在本轮存在不可接受的系统性失败时返回 error，让 Dapr 重新投递。

Job 可能重复投递，因此不能依赖“该任务只执行一次”。

## 7. Local Scheduler 边界

现有 `NewScheduler`、`Every`、`At`、`RedisLocker` 和 `PrometheusObserver` 保留，但重新定位为：

- 单元测试和集成测试；
- 本地无 Dapr 环境；
- 单机工具；
- 应急降级。

生产 IAM 不再同时启用 Dapr Runtime 和 local scheduler 执行同一个 Job，否则会产生双重触发。

## 8. Kubernetes 部署要求

独立 callback Server 模式下，Dapr `app-port` 指向 callback 端口，而 IAM 对外 gRPC 端口保持不变：

```yaml
metadata:
  annotations:
    dapr.io/enabled: "true"
    dapr.io/app-id: "aisphere-iam"
    dapr.io/app-port: "9001"
    dapr.io/app-protocol: "grpc"
```

容器仍可同时暴露：

- `8000`：IAM HTTP API；
- `9000`：IAM 业务 gRPC API；
- `9001`：Dapr callback gRPC，仅供 sidecar 使用。

生产建议：

- Dapr control plane 使用 HA 安装；
- Scheduler 数据卷使用高可靠块存储；
- 需要自由扩缩 Scheduler 副本时使用 external etcd；嵌入式 etcd 模式下不要随意修改 Scheduler replica 数量；
- 定期执行 `dapr scheduler export -k`，并按 RPO 保存备份；
- 监控 Scheduler etcd metrics、Job callback 错误率、IAM handler 时延及最后成功时间；
- 设置 `APP_API_TOKEN`，启用 Dapr mTLS，并用 NetworkPolicy 限制 callback 端口；
- readiness 必须覆盖 sidecar 与 callback Server 是否可用。

## 9. 版本基线

- Kernel Go：`1.26.4`
- Dapr Go SDK：`v1.15.0`
- Dapr Runtime / Scheduler：`v1.18.x`

该组合使用稳定 `ScheduleJob/GetJob/DeleteJob` gRPC RPC，并在 SDK 内兼容回退到旧 Alpha RPC。禁止新代码直接调用 `ScheduleJobAlpha1`。
