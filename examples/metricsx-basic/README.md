# metricsx basic example

最小可运行示例，展示 metricsx 的端到端用法：注册指标、记录业务事件、暴露 /metrics endpoint。

## 运行

```bash
go run ./examples/metricsx-basic
```

服务器启动后会：
1. 监听 `:9001` 暴露 `/metrics` 和 `/debug/pprof/*`
2. 每 500ms 模拟一次业务活动（计数 + 延迟 + 会话）

## 查看 metrics

打开另一个终端：

```bash
# 查看所有 metrics
curl http://localhost:9001/metrics

# 过滤业务 metrics
curl -s http://localhost:9001/metrics | grep -E 'skill_create_total|request_seconds|active_sessions|errors_total'

# 查看 system metrics
curl -s http://localhost:9001/metrics | grep -E 'app_go_goroutines|app_sys_memory'
```

## 预期输出（业务 metrics 部分）

```text
# HELP skill_create_total Total skills created
# TYPE skill_create_total counter
skill_create_total{tenant="t_acme",visibility="public"} 42

# HELP request_seconds Request latency
# TYPE request_seconds histogram
request_seconds_bucket{operation="create_skill",le="0.005"} 42
request_seconds_bucket{operation="create_skill",le="0.01"} 42
...
request_seconds_sum{operation="create_skill"} 0.001
request_seconds_count{operation="create_skill"} 42

# HELP active_sessions Active sessions
# TYPE active_sessions gauge
active_sessions{tenant="t_acme"} 42

# HELP errors_total Total errors by code
# TYPE errors_total counter
errors_total{error_code="AIHUB_SKILL_NOT_FOUND",http_status="404"} 8
```

## 这个示例展示了什么

1. **NewPrometheusManager**：一行创建 Prometheus-backed Manager
2. **RegisterSystemMetrics**：自动注册 goroutines/memory/GC gauges
3. **NewCounter / NewHistogram / NewGauge / NewUpDownCounter**：注册 4 种指标
4. **IncrementCounter**：计数业务事件（带低基数标签）
5. **RecordHistogram**：记录请求延迟
6. **SetGauge**：设置当前会话数
7. **DeltaUpDownCounter**：增减队列大小
8. **GetHandler**：暴露 `/metrics` + `/debug/pprof/*`
9. **错误计数**：按 error_code 标签统计错误
10. **多租户标签**：按 tenant 维度分段

## 学到什么

- 启动时注册所有指标，业务代码只记录
- 标签只用低基数维度（tenant / operation / status / error_code）
- nil Manager 安全（虽然这里不是 nil）
- `/metrics` 自动包含 system metrics

## 相关文档

- `metricsx/README.md` — 完整用户指南
- `docs/ai/metricsx.md` — AI 编码食谱（10 场景）
- `metricsx/example_test.go` — 所有 API 的 Go 标准 Example
- `metricsx/example_business_test.go` — 10 个完整业务场景
