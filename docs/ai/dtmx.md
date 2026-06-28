# dtmx：Kernel 分布式事务封装

`dtmx` 是 Kernel 的分布式事务抽象层。业务组件只依赖 `github.com/aisphereio/kernel/dtmx`，不要直接依赖 `github.com/dtm-labs/client`。当前生产实现是 `dtmx/dtm`，底层使用 `github.com/dtm-labs/client/dtmcli` 的 HTTP Saga 能力。

DTM 是独立的事务协调服务。Kernel 不把 DTM server 嵌入进业务进程；业务服务通过 `dtmx.Manager` 向外部 DTM server 提交事务。DTM server 生产建议用 PostgreSQL 存储事务状态。

## DTM server 的部署边界

Kernel 不管理 DTM server 的存储后端，也不生成 DTM server 配置。DTM server 是独立基础设施；你可以在部署层让它使用 PostgreSQL、MySQL 或其他 DTM 支持的存储。应用侧只关心 DTM HTTP API 地址，例如：

```yaml
dtm:
  enabled: true
  driver: dtm
  protocol: http
  server: http://127.0.0.1:36789/api/dtmsvr
  service_base_url: http://127.0.0.1:18001
  branch_prefix: /internal/dtm
  branch_secret: ${DTM_BRANCH_SECRET}
  wait_result: true
  timeout_ns: 10000000000
  metrics_enabled: true
```

DTM 官方示例中 HTTP 服务默认监听 36789，gRPC 默认监听 36790。生产环境里，DTM 的数据库、迁移、账号和高可用属于 DTM 服务部署，不属于 kernel 配置模型。

## Kernel 侧初始化

```go
import (
    "github.com/aisphereio/kernel/dtmx"
    _ "github.com/aisphereio/kernel/dtmx/dtm"
)

manager, err := dtmx.New(dtmx.Config{
    Enabled:        true,
    Driver:         "dtm",
    Protocol:       "http",
    Server:         "http://127.0.0.1:36789/api/dtmsvr",
    ServiceBaseURL: "http://127.0.0.1:18001",
    BranchPrefix:   "/internal/dtm",
    BranchSecret:   os.Getenv("DTM_BRANCH_SECRET"),
    WaitResult:     true,
    Logger:         logger,
    Metrics:        metrics,
    MetricsEnabled: true,
})
if err != nil {
    return err
}
```

生命周期上下文可直接交给 `kernel.New` 注入：

```go
app := kernel.New(
    kernel.Name("aihub"),
    kernel.LogxLogger(logger),
    kernel.Metrics(metrics),
    kernel.DTM(manager),
)

manager = kernel.DTMFromContext(ctx)
```

底层仍然是 `dtmx.Inject` / `dtmx.FromContext`；业务 bootstrap 代码优先使用 `kernel.DTMFromContext(ctx)`，普通库代码可以继续使用 `dtmx.FromContext(ctx)`。

## Saga 建模

跨 S3、PG、缓存、权限等资源时，优先用 Saga：

```go
gid, err := manager.NewGID(ctx)
if err != nil { return err }

saga := dtmx.NewSaga(gid, "skill.upload", dtmx.WithSagaWaitResult(true)).
    AddHTTP(
        "promote-object",
        manager.BranchURL("skill/object/promote"),
        manager.BranchURL("skill/object/promote_compensate"),
        payload,
    ).
    AddHTTP(
        "write-metadata",
        manager.BranchURL("skill/metadata/write"),
        manager.BranchURL("skill/metadata/write_compensate"),
        payload,
    )

_, err = manager.SubmitSaga(ctx, saga)
```

分支接口必须满足：

1. action 幂等。
2. compensate 幂等。
3. payload 不放大对象，只放 object key、sha256、size、业务 ID。
4. branch endpoint 不要暴露到公网；建议内网端口或 mTLS。
5. 如果使用 HTTP Saga，配置 `branch_secret`，dtmx/dtm 会通过 DTM 的 `branch_headers` 传递 `X-Kernel-DTM-Branch-Token`，服务端用 `dtmx.ValidateBranchRequest` 或 `dtmx.BranchAuthMiddleware` 校验。

## 通信与认证

应用向 DTM server 提交事务使用 DTM 官方 Go SDK 的 `dtmcli` HTTP client。DTM server 再回调业务服务的 action/compensate URL。`dtmcli` 的 Saga `Add` 会把 action/compensate URL 和 payload 注册给 DTM；DTM 的 `TransOptions` 支持 `BranchHeaders`，并在回调 branch 时设置这些 headers。kernel 的 `dtmx/dtm` 会在 `Config.BranchSecret` 非空时自动设置：

```text
X-Kernel-DTM-Branch-Token: <branch_secret>
```

业务服务内部回调接口应校验：

```go
if err := dtmx.ValidateBranchRequest(r, cfg.DTM.BranchSecret); err != nil {
    // return 401
}
```

或者直接包一层：

```go
handler = dtmx.BranchAuthMiddleware(cfg.DTM.BranchSecret, handler)
```

这不是替代 mTLS / 内网隔离，而是应用层兜底。它只能保护 DTM -> 业务服务的 branch callback endpoint，不能阻止别人直接访问 DTM server。生产建议同时使用内网地址、网络策略、网关 ACL 或 mTLS，避免 DTM server 和 `/internal/dtm/*` 暴露到公网。

## 指标

`metrics_enabled=true` 时注册：

- `kernel_dtmx_operations_total`
- `kernel_dtmx_operation_duration_seconds`

低基数标签：

- `driver`
- `protocol`
- `operation`
- `status`
- `code`

## 错误码

- `DTMX_DISABLED`
- `DTMX_INVALID_CONFIG`
- `DTMX_UNKNOWN_DRIVER`
- `DTMX_INVALID_SAGA`
- `DTMX_UNSUPPORTED_PROTOCOL`
- `DTMX_SUBMIT_FAILED`
- `DTMX_NEW_GID_FAILED`
- `DTMX_TIMEOUT`
- `DTMX_CANCELED`

## 边界

这一版只封装 Kernel 分布式事务能力，不改 hub skill 生命周期。后续 hub skill 应基于 `dtmx.Manager` 重新设计 draft、package、manifest、delete、retention、GC/reconcile 等完整流程。
