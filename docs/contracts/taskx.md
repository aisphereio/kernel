# taskx 后台任务与周期协调契约

## 1. 定位

`taskx` 是 Kernel 的后台任务生命周期与周期协调能力，适用于：

- 数据库事实与外部投影之间的周期 reconciliation；
- 权限、订阅、租约、会话等到期状态回收；
- 周期清理、补偿、健康巡检和小规模后台维护任务；
- 需要在服务启动时立即补偿一次、之后按固定间隔运行的任务。

`taskx` **不是**消息队列，也不承诺 exactly-once。Handler 必须幂等，执行语义是 at-least-once。

## 2. 为什么不直接在业务中启动 goroutine

业务自行启动 `time.Ticker` 或 goroutine 会产生以下问题：

1. 多副本服务会重复执行；
2. 服务停机期间错过的工作不会在恢复后补偿；
3. 缺少统一超时、重试、panic recovery 和优雅停机；
4. 缺少统一指标和执行事件；
5. 分布式锁容易出现误释放其他实例锁的安全问题。

`taskx` 统一解决这些横切能力。业务只注册 Job 和 Handler。

## 3. Go 生态选型

| 方案 | 特点 | Kernel 结论 |
|---|---|---|
| `robfig/cron/v3` | 成熟、轻量、进程内 cron | 不提供持久化或分布式协调，不足以单独解决多副本安全任务 |
| `go-co-op/gocron/v2` | 调度 API 完整，支持 elector/locker、监听器与监控扩展 | 适合作为未来 cron provider；不直接暴露给业务，避免 Kernel API 与第三方绑定 |
| `hibiken/asynq` | Redis 持久化队列、重试、定时任务、Web UI | 适合邮件、文件处理等离散异步任务；当前仍是 v0，且部分 Lua 对 Redis Cluster 有兼容限制 |
| `riverqueue/river` | PostgreSQL 持久化、事务入队、唯一任务、强类型 worker | 可靠性强，适合 PG 主导的异步任务；不应让所有 Kernel 服务强制依赖 PostgreSQL |
| Kernel `taskx` | provider-neutral Job/Schedule/Locker/Observer；内置轻量 scheduler 与 Redis lease | 当前主线，优先解决 reconciliation 和到期回收；后续可增加 Asynq/River provider |

## 4. 核心语义

### 4.1 周期

- `Every(duration)`：固定周期；服务停机或运行过慢造成的历史 tick 会被跳过，不会形成追赶风暴。
- `At(time)`：单次执行。
- `ScheduleFunc`：允许框架 provider 或业务基础设施层扩展 cron 等日历调度。
- `RunOnStart`：服务启动后立即执行一次，用于补偿停机期间已到期的数据。

### 4.2 并发

- 默认同一进程内不允许同一 Job 重叠执行；重叠 tick 被跳过。
- `AllowOverlap=true` 只适合明确允许并行的无共享状态任务。
- `Lease.Enabled=true` 后，通过 `Locker` 保证多个服务副本中最多一个实例执行该次任务。

### 4.3 Redis lease

`RedisLocker` 使用：

- `SET key token NX PX ttl` 获取租约；
- 所有权 token 校验后的 Lua `PEXPIRE` 续期；
- 所有权 token 校验后的 Lua `DEL` 释放。

不得使用无 token 的 `DEL key`，否则旧实例可能误删新实例已获得的租约。

### 4.4 重试和超时

- `Timeout` 是每次 attempt 的超时；
- `RetryPolicy.MaxAttempts` 包含首次执行；
- 默认不重试；启用重试后使用有上限的指数退避；
- panic 会转换为 error 并进入相同重试路径；
- 分布式租约在整个 run（包括重试退避）期间持有并续期。

### 4.5 可观测性

`Observer` 接收 scheduled、started、retrying、succeeded、failed、skipped 事件。

Kernel 内置 `PrometheusObserver`，导出：

- `kernel_taskx_runs_total`；
- `kernel_taskx_skips_total`；
- `kernel_taskx_retries_total`；
- `kernel_taskx_run_duration_seconds`；
- `kernel_taskx_last_success_unixtime`。

业务可通过组合 Observer 把同一事件写入 `logx`、OTel trace 或 `auditx`。

## 5. GRANT-006 推荐实现

Grant 数据库记录是控制面事实，SpiceDB relationship 是查询投影。过期任务必须按以下顺序实现幂等 reconciliation：

1. 分页领取 `expires_at <= now AND status = active` 的 Grant；
2. 将 Grant 原子推进到 `expiring`，或使用数据库行锁/claim token 防止重复处理；
3. 删除 SpiceDB relationship；删除不存在的 relationship 也视为成功；
4. 将 Grant 更新为 `expired`；
5. 写入审计记录；
6. 单条失败保留为可重试状态，不能阻断整批其他 Grant。

仅使用 Redis lease 不能替代数据库级 claim。Redis lease 负责“同一周期任务尽量只有一个副本运行”，数据库状态机负责进程崩溃后的恢复和逐条幂等。

```go
redisLocker := taskx.NewRedisLocker(redisClient, "iam:task:")
metricsObserver, err := taskx.NewPrometheusObserver(prometheus.DefaultRegisterer)
if err != nil {
    return err
}

scheduler := taskx.NewScheduler(
    taskx.WithLocker(redisLocker),
    taskx.WithObserver(metricsObserver),
)

err = scheduler.Register(taskx.Job{
    Name:       "grant-expiration-reconciler",
    Schedule:   taskx.Every(5 * time.Minute),
    RunOnStart: true,
    Timeout:    2 * time.Minute,
    Retry: taskx.RetryPolicy{
        MaxAttempts:    3,
        InitialBackoff: 2 * time.Second,
        MaxBackoff:     15 * time.Second,
    },
    Lease: taskx.LeaseOptions{
        Enabled: true,
        Key:     "grant-expiration-reconciler",
        TTL:     3 * time.Minute,
    },
    Handler: grantService.ExpireDueGrants,
})
if err != nil {
    return err
}

if err := scheduler.Start(appContext); err != nil {
    return err
}
```

服务退出时必须调用：

```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = scheduler.Shutdown(shutdownCtx)
```

## 6. 部署边界

- IAM 多副本场景必须启用 Redis lease；
- Redis 故障时任务本次执行失败，不允许无锁降级运行；
- `RunOnStart=true` 保证服务恢复后立即扫描所有已到期 Grant；
- 对安全敏感到期任务，应针对 `last_success_unixtime` 设置告警；
- 如果未来任务量变为大量离散任务、需要持久化 payload、死信队列或独立扩缩容，应切换到 Asynq/River provider 或独立 worker deployment，而不是继续扩大进程内 scheduler。
