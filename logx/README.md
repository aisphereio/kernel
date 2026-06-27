# logx

`logx` is the Aisphere Kernel logging package. It keeps slog as the internal
engine, but exposes a Kernel-facing API for business code:

- `logx.Logger` and `logx.Field` for service/application code.
- `logx.NewHandler` / `logx.NewLogger` for framework adapters that still need
  direct slog compatibility.
- `logx.ContextWithFields` / `logx.Inject` for request-scoped fields.
- built-in redaction, sampling, access events, external-call events and test
  logger support.

## Business logger

```go
cfg := logx.DefaultConfig("dev")
cfg.ServiceName = "aihub"
logger, levelCtl, err := logx.New(cfg)
if err != nil {
    panic(err)
}
defer logger.Sync()

logger.Info("service started",
    logx.String("addr", ":8000"),
    logx.String("version", "v0.1.0"),
)

_ = levelCtl.SetLevel("debug")
```

## Context fields

```go
ctx = logx.ContextWithFields(ctx,
    logx.String("request_id", requestID),
    logx.String("trace_id", traceID),
    logx.String("subject_id", subjectID),
)

logx.FromContextOr(ctx, logger).Info("request accepted")
```

## Framework slog compatibility

Kernel internals can still use slog-compatible helpers while migrating:

```go
handler := logx.NewHandler(
    logx.WithWriter(os.Stdout),
    logx.WithFormat(logx.FormatJSON),
    logx.WithFilter(logx.FilterKey("password", "token")),
)
logx.SetDefault(slog.New(handler))
logx.Info("listening", "addr", addr)
```

## Error logs

```go
logger.Error("create skill failed",
    logx.String("operation", "aihub.skill.create"),
    logx.Err(err),
)
```

`logx.Err` extracts common weak error interfaces such as `ErrorCode`,
`HTTPStatus`, `StatusCode`, `GRPCCode`, and `Retryable`, so it works with
`errorx` without importing `errorx` and creating cycles.

## Access logs

```go
logx.LogAccess(logger, logx.AccessEvent{
    Side:       "server",
    Protocol:   "http",
    Operation:  "POST /v1/skills",
    StatusCode: 201,
    Latency:    latency,
})
```

## OpenTelemetry extractor

The optional `logx/otelx` package is behind the `otel` build tag and can extract
`trace_id` / `span_id` from OpenTelemetry contexts without making core `logx`
depend on OTel.
