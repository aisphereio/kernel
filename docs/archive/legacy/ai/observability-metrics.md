# Kernel metrics readiness

Audit is intentionally out of scope for this pass. Metrics should be treated as a
first-class Kernel bootstrap dependency, not as scattered optional wiring inside
individual components.

## Startup order

Use this order in every service:

```text
1. Read minimal bootstrap config
2. Build logx logger and install it as the default logger
3. Build metricsx manager
4. Inject logger + metrics into kernel lifecycle context
5. Initialize dbx/cachex/objectstorex/authn/authz with the shared logger/metrics
6. Start business transports
7. Start admin metrics endpoint
```

## One-line Prometheus setup

```go
app := kernel.New(
    kernel.Name("aihub"),
    kernel.Version(version),
    kernel.LogxLogger(logger),
    kernel.PrometheusMetrics(":9090"), // GET /metrics
)
```

`PrometheusMetrics` creates one `metricsx.Manager`, registers Go runtime gauges,
injects the manager into lifecycle hook contexts, and starts a small admin HTTP
server when the address is non-empty.

The built-in admin server exposes only `/metrics` by default. Enable pprof
explicitly when needed:

```go
kernel.PrometheusMetrics("127.0.0.1:9090"),
kernel.MetricsPprof(true),
```

Change the path when a platform gateway requires it:

```go
kernel.PrometheusMetrics(":9090"),
kernel.MetricsPath("/internal/metrics"),
```

## Bring your own manager

If the service already owns an OpenTelemetry/Prometheus setup, pass it in:

```go
manager := metricsx.NewPrometheusManager("aihub", version, logger)

app := kernel.New(
    kernel.Name("aihub"),
    kernel.Version(version),
    kernel.LogxLogger(logger),
    kernel.Metrics(manager),
)
```

`kernel.Metrics(manager)` does not start an extra HTTP server. Mount the handler
wherever your admin server lives:

```go
adminMux.Handle("/metrics", metricsx.GetMetricsHandler(manager))
```

## Lifecycle hook wiring

Hooks should read the shared manager from context and pass it into component
configs. This keeps dbx/cachex/s3/authn/authz using the same metric registry.

```go
kernel.BeforeStart(func(ctx context.Context) error {
    manager := kernel.MetricsFromContext(ctx)
    logger := logx.FromContext(ctx)

    db, err := dbx.New(dbx.Config{
        Driver:         "postgres",
        DSN:            dsn,
        Logger:         logger,
        Metrics:        manager,
        MetricsEnabled: true,
    })
    if err != nil {
        return err
    }
    _ = db
    return nil
})
```

The same pattern applies to:

- `cachex.Config{Metrics: manager, MetricsEnabled: true}`
- `objectstorex.Config{Metrics: manager, MetricsEnabled: true}`
- `authn/casdoor.Config{Metrics: manager, MetricsEnabled: true}`
- `authz/spicedb.Config{Metrics: manager, MetricsEnabled: true}`

## Idempotent registration

`metricsx.RegisterSystemMetrics(manager)` and repeated `NewCounter/NewGauge/...`
calls are now safe to call more than once on the same manager. Duplicate
registrations are ignored at debug level instead of polluting error logs.

This matters because Kernel components can be initialized in multiple modules,
tests, or examples while sharing one manager.

## Endpoint policy

Production recommendation:

```text
business server: :8080
admin server:    127.0.0.1:9090/metrics or private subnet only
pprof:           disabled by default, enable temporarily only
```

Do not expose `/metrics` publicly. Metrics labels intentionally stay
low-cardinality: method, route, status, driver, operation, code, grpc_code.
Dynamic IDs, token values, request bodies, SQL text, object keys, and user input
belong in logx/tracing, not metrics labels.

## Smoke test

```powershell
go test ./metricsx
go test ./transportx/http ./transportx/grpc
go test ./dbx ./cachex ./objectstorex
go test ./authn/... ./authz/...
go test ./...
```

Runtime check:

```powershell
curl http://127.0.0.1:9090/metrics
```

Expected families include:

```text
app_go_goroutines
kernel_http_requests_total
kernel_grpc_requests_total
kernel_dbx_operations_total
kernel_cachex_operations_total
kernel_objectstorex_operations_total
kernel_authn_casdoor_requests_total
kernel_authz_spicedb_requests_total
```
