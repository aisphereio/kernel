# logx/otelx

`otelx` 是可选扩展，不参与默认构建。

启用方式：

```bash
go get go.opentelemetry.io/otel/trace
go test -tags otel ./...
```

接入方式：

```go
logger, levels, err := logx.New(cfg, logx.WithExtractor(otelx.TraceExtractor))
```

核心 `logx` 包保持标准库依赖，避免所有服务被强制绑定 OTel。
