# Transport observability with logx, errorx and metricsx

This note describes the low-intrusion observability layer added to `transportx/http`
and `transportx/grpc`. It is intentionally implemented at the transport boundary so
business handlers do not need to change their logic.

## What is recorded

### HTTP server

`transportx/http.NewServer` now wires the built-in access log by default using
`logx.HTTPAccessLog` and the default `logx.AccessLogConfig`.

For handlers registered via `Route(...).GET/POST/...`, returned errors are logged
before the existing `ErrorEncoder` writes the response. The log includes:

- `event=handler_error`
- `method`, `path`, `route`
- `status`, `status_class`
- `code`, `grpc_code`, `retryable`
- structured `errorx` fields and redacted internal metadata

Optional HTTP metrics are enabled by passing a `metricsx.Manager`:

```go
logger, _, _ := logx.New(logx.DefaultConfig("dev"))
metrics := metricsx.NewPrometheusManager("aihub", "dev", logger)

srv := httpx.NewServer(
    httpx.Logger(logger),
    httpx.Metrics(metrics),
)
```

Registered metric names:

- `kernel_http_server_requests_total`
- `kernel_http_server_request_duration_seconds`
- `kernel_http_server_inflight_requests`
- `kernel_http_server_handler_errors_total`

Labels are intentionally low-cardinality: `method`, `route`, `status`,
`status_class`, and for handler errors also `code`.

### gRPC server

`transportx/grpc.NewServer` now records unary and stream access logs through
`logx.LogAccess`. Incoming metadata is copied into log context when present:
`x-request-id`, `request-id`, `x-trace-id`, `trace-id`, or W3C `traceparent`.

Optional gRPC metrics are enabled the same way:

```go
srv := grpcx.NewServer(
    grpcx.Logger(logger),
    grpcx.Metrics(metrics),
)
```

Registered metric names:

- `kernel_grpc_server_requests_total`
- `kernel_grpc_server_request_duration_seconds`
- `kernel_grpc_server_inflight_requests`

Labels are: `service`, `method`, `operation`, `grpc_code`, `code`.

## Error handling changes

HTTP error encoding now normalizes common transport errors into Kernel `errorx`:

- `context.Canceled` → `HTTP_CLIENT_CLOSED` / 499
- `context.DeadlineExceeded` → `HTTP_REQUEST_TIMEOUT` / 504

HTTP client transport failures are wrapped into Kernel errors:

- request context deadline → `HTTP_CLIENT_TIMEOUT`
- request context cancellation → `HTTP_CLIENT_CANCELED`
- other network failures → `HTTP_CLIENT_REQUEST_FAILED`

When a non-2xx HTTP response has no Kernel error body, the fallback code now
matches the HTTP status (`404 -> NOT_FOUND`, `429 -> TOO_MANY_REQUESTS`, etc.)
instead of always returning `INTERNAL_ERROR`.

## Disabling or overriding

Disable access log explicitly:

```go
srv := httpx.NewServer(
    httpx.AccessLog(logx.AccessLogConfig{Enabled: false}),
)
```

Override slow threshold / skip paths:

```go
cfg := logx.DefaultConfig("prod").AccessLog
cfg.SlowThreshold = 500 * time.Millisecond
cfg.SkipPaths = append(cfg.SkipPaths, "/internal/health")

srv := httpx.NewServer(httpx.AccessLog(cfg))
```

Business code should still return `errorx` errors. The transport layer only
observes and normalizes errors; it does not change handler branching or domain
logic.
