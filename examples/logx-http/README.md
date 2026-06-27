# logx HTTP example

完整 HTTP 服务器示例，展示 logx 在真实业务场景下的端到端用法：access log
中间件、panic recovery、request-scoped fields、动态级别控制。

## 运行

```bash
go run ./examples/logx-http
```

服务器监听 `:18080`。

## 测试不同场景

打开另一个终端，依次执行：

```bash
# 200 OK — INFO 级 access log
curl -i 'http://localhost:18080/?status=200'

# 404 Not Found — WARN 级 access log
curl -i 'http://localhost:18080/?status=404'

# 500 Internal — ERROR 级 access log
curl -i 'http://localhost:18080/?status=500'

# Panic — recovered，ERROR 级日志
curl -i 'http://localhost:18080/panic'

# Health check — 不记 access log（在 SkipPaths 中）
curl -i 'http://localhost:18080/healthz'

# 动态调整日志级别到 debug
curl -i 'http://localhost:18080/admin/log-level?level=debug'

# 再发请求，现在能看到 debug 级日志
curl -i 'http://localhost:18080/?status=200'

# 调回 info
curl -i 'http://localhost:18080/admin/log-level?level=info'
```

## 场景 → logx API → 日志级别对应表

| # | curl 场景 | logx API | 日志级别 | event |
|---|---|---|---|---|
| 1 | `?status=200` | `HTTPAccessLog` middleware | INFO | access |
| 2 | `?status=404` | `HTTPAccessLog` middleware | WARN | access |
| 3 | `?status=500` | `HTTPAccessLog` middleware | ERROR | access |
| 4 | `/panic` | `Recovery` middleware | ERROR | panic_recovered |
| 5 | `/healthz` | （跳过 access log） | — | — |
| 6 | `/admin/log-level?level=debug` | `LevelHTTPHandler` | — | — |
| 7 | `?status=200` (after level=debug) | `FromContext(ctx).Debug` | DEBUG | processing |

每个请求会产生多条结构化日志：

```json
{
  "timestamp": "2026-...",
  "level": "info",
  "message": "request started",
  "service": "demo-http",
  "env": "prod",
  "version": "v1.0.0",
  "request_id": "req_1234567890",
  "route": "/",
  "method": "GET"
}
{
  "timestamp": "2026-...",
  "level": "info",
  "message": "/ completed",
  "service": "demo-http",
  "event": "access",
  "side": "server",
  "protocol": "http",
  "operation": "GET /",
  "method": "GET",
  "path": "/?status=200",
  "status": 200,
  "latency": 1200000,
  "latency_ms": 1
}
```

## 学到什么

这个示例展示了 logx 的核心用法：

1. **Bootstrap**：`logx.New(cfg)` 返回 Logger + LevelController
2. **DefaultConfig**：prod 模式用 JSON 格式，AddSource=false
3. **HTTP access log 中间件**：`logx.HTTPAccessLog(logger, cfg)` 自动按状态码选级别
4. **Panic recovery 中间件**：`logx.Recovery(logger)` 捕获 panic 并记 ERROR
5. **Request-scoped fields**：`logx.Inject(ctx, logger, ...)` + `logx.FromContext(ctx)`
6. **SkipPaths**：health check 等健康端点不污染 access log
7. **SlowThreshold**：超过阈值的请求在 access log 中标记（未来扩展）
8. **动态级别控制**：`LevelHTTPHandler(levelCtl)` 暴露 HTTP endpoint
9. **结构化字段**：所有日志都是 JSON，便于 Loki/ELK/ClickHouse 索引
10. **event 字段**：`event=access` / `event=panic_recovered` 便于聚合查询

## 在真实项目中

- 把 `logx.HTTPAccessLog` + `logx.Recovery` 放到所有 HTTP server 的中间件链最外层
- 在 handler 入口调用 `logx.Inject(ctx, logger, request_id, trace_id, subject_id, route)`
- 下游 service / repository / worker 全部用 `logx.FromContext(ctx)` 取 logger
- 通过 `/admin/log-level` endpoint 在生产环境临时开 debug 排障

## 相关文档与示例

| 想学什么 | 看哪里 |
|---|---|
| logx 完整指南 | `logx/README.md` |
| AI 编码食谱（10 场景） | `docs/ai/logx.md` |
| 所有 API 的 Example | `logx/example_test.go` |
| 完整业务场景 Example | `logx/example_business_test.go` |
| 最小可运行示例 | `examples/logx-basic/` |
| 本 HTTP 示例 | `examples/logx-http/`（当前目录） |
| 设计规范 | `docs/design/logx.md` |
| 不可破坏契约 | `docs/contracts/logx.md` |
| 验收清单 | `docs/process/logx-acceptance-checklist.md` |
