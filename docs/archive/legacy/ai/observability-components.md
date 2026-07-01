# Component observability with logx, errorx and metricsx

This note describes the low-intrusion observability layer for Kernel runtime
components. The goal is to keep business logic unchanged while making startup,
backend calls and close paths visible through the same `logx`, `errorx` and
`metricsx` contracts.

## Startup rule

Initialize the application logger before any component is created:

```go
logger, _, _ := logx.New(logx.DefaultConfig("dev"))
metrics := metricsx.NewPrometheusManager("aihub", "dev", logger)

app := kernel.New(
    kernel.Name("aihub"),
    kernel.LogxLogger(logger), // preferred
    // kernel.Logger(slogLogger), // also installs logx default
)
```

`kernel.New` now installs the supplied logger as the package default before
lifecycle hooks and servers start. Components created from `BeforeStart` hooks or
business bootstrap code can therefore write to the correct sink even when their
own config does not explicitly pass a logger.

For component configs, pass the same logger and metrics manager when available:

```go
db, err := dbx.New(dbx.Config{
    Driver: "postgres",
    DSN: dsn,
    Logger: logger,
    Metrics: metrics,
    MetricsEnabled: true,
})
```

## Covered components

### dbx

`dbx.New` logs open success/failure and returns an observed `DB` wrapper. Public
`DB` and `Tx` methods record latency, result status and normalized error codes.
Driver/sentinel errors are normalized into Kernel `errorx` values such as:

- `DBX_NO_ROWS`
- `DBX_DUPLICATE_KEY`
- `DBX_TIMEOUT`
- `DBX_SCHEMA_NOT_READY`
- `DBX_DATABASE_NOT_EXIST`
- `DBX_FOREIGN_KEY_VIOLATION`
- `DBX_CLOSED`
- `DBX_OPERATION_FAILED`

Business callback errors from `InTx` are intentionally returned as-is so existing
transaction semantics do not change.

Metrics:

- `kernel_dbx_operations_total`
- `kernel_dbx_operation_duration_seconds`

Labels: `driver`, `operation`, `status`, `code`.

### cachex

`cachex.New` logs open success/failure and returns an observed `Cache` wrapper.
Cache sentinel errors are normalized into Kernel `errorx` values such as:

- `CACHEX_NOT_FOUND`
- `CACHEX_INVALID_CONFIG`
- `CACHEX_UNKNOWN_DRIVER`
- `CACHEX_CLOSED`
- `CACHEX_TYPE_MISMATCH`
- `CACHEX_NIL_VALUE`
- `CACHEX_TIMEOUT`
- `CACHEX_OPERATION_FAILED`

`GetOrSet` callback errors are logged and measured but returned as-is, preserving
cache-through business semantics.

Metrics:

- `kernel_cachex_operations_total`
- `kernel_cachex_operation_duration_seconds`

Labels: `driver`, `operation`, `status`, `code`.

### objectstorex / S3-compatible storage

`objectstorex.New` logs open success/failure and returns an observed `Client`
wrapper. Object store sentinel errors are normalized into Kernel `errorx` values
such as:

- `OBJECTSTOREX_NOT_FOUND`
- `OBJECTSTOREX_INVALID_CONFIG`
- `OBJECTSTOREX_UNKNOWN_DRIVER`
- `OBJECTSTOREX_BUCKET_NOT_EXISTS`
- `OBJECTSTOREX_CLOSED`
- `OBJECTSTOREX_TIMEOUT`
- `OBJECTSTOREX_OPERATION_FAILED`

Metrics:

- `kernel_objectstorex_operations_total`
- `kernel_objectstorex_operation_duration_seconds`

Labels: `driver`, `operation`, `status`, `code`.

### authn/casdoor

The Casdoor adapter installs an instrumented SDK HTTP client. Backend HTTP calls
emit access-style logs with method, redacted URL path, status class and latency.
Query strings are not logged to avoid leaking tokens or authorization codes.

Metrics:

- `kernel_authn_casdoor_requests_total`
- `kernel_authn_casdoor_request_duration_seconds`

Labels: `method`, `path`, `status_class`, `code`.

### authz/spicedb

The SpiceDB adapter installs gRPC client interceptors. Backend calls emit
access-style logs with service, method, full gRPC operation, gRPC status and
latency.

Metrics:

- `kernel_authz_spicedb_requests_total`
- `kernel_authz_spicedb_request_duration_seconds`

Labels: `service`, `method`, `grpc_code`, `code`.

## Compatibility notes

- Observability is opt-in for metrics: set `MetricsEnabled: true` and pass a
  `metricsx.Manager`.
- Logging is always safe: when no logger is provided, components use
  `logx.DefaultLogger()`.
- Logs avoid high-cardinality labels. Request object keys, SQL statements and URL
  query strings are not used as metrics labels.
- Existing sentinel errors remain in the error chain through `errorx.Wrap`, so
  `errors.Is(err, dbx.ErrNoRows)` and similar checks continue to work.

## Metrics bootstrap update

Metrics bootstrap is now documented in `docs/ai/observability-metrics.md`. Prefer `kernel.PrometheusMetrics(":9090")` or `kernel.Metrics(manager)` at app creation, then use `kernel.MetricsFromContext(ctx)` inside lifecycle hooks to pass the shared manager into component configs.
