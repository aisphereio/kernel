# metricsx Contract

模块：`github.com/aisphereio/kernel/metricsx`
验收等级目标：L3 可生产试用
版本：v0.1.0-alpha.1

## 1. Contract 目标

metricsx 是 Kernel 的指标包。它提供 Counter / UpDownCounter / Histogram /
Gauge 四种指标类型的注册和记录，nil 安全，带基数校验。

只要以下 contract 不被破坏，内部实现可以重构。

## 2. 不可破坏行为

### 2.1 Manager 接口

```go
type Manager interface {
    NewCounter(name, desc string)
    NewUpDownCounter(name, desc string)
    NewHistogram(name, desc string, buckets ...float64)
    NewGauge(name, desc string)

    IncrementCounter(ctx context.Context, name string, labels ...string)
    DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
    RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
    SetGauge(name string, value float64, labels ...string)
}
```

### 2.2 nil 安全

nil Manager 的所有方法必须是 no-op，不 panic：

```go
var m Manager  // nil
m.NewCounter("x", "y")                    // no-op
m.IncrementCounter(ctx, "x")              // no-op
m.DeltaUpDownCounter(ctx, "x", 1.0)       // no-op
m.RecordHistogram(ctx, "x", 0.5)          // no-op
m.SetGauge("x", 1.0)                      // no-op
```

`Noop()` 返回的 Manager 行为与 nil Manager 完全一致。

### 2.3 注册行为

- `NewCounter(name, desc)`：注册 int64 单调递增计数器
- `NewUpDownCounter(name, desc)`：注册 float64 可增可减计数器
- `NewHistogram(name, desc, buckets...)`：注册 float64 直方图，buckets 可选
- `NewGauge(name, desc)`：注册 float64 当前值 gauge

重复注册同名 metric 必须日志告警但不 panic（返回 `*ErrAlreadyRegistered`）。

### 2.4 记录行为

- `IncrementCounter(ctx, name, labels...)`：+1
- `DeltaUpDownCounter(ctx, name, value, labels...)`：+value（可为负）
- `RecordHistogram(ctx, name, value, labels...)`：观察 value
- `SetGauge(name, value, labels...)`：设置当前值（覆盖之前）

记录未注册的 metric 必须日志告警但不 panic（返回 `*ErrNotRegistered`）。

### 2.5 标签格式

labels 是 key-value 对的变参：`"k1", "v1", "k2", "v2"`。

- 奇数个 labels：日志告警，最后一个忽略
- 超过 20 个 labels：日志告警（基数风险），但仍然记录

### 2.6 Logger 接口

```go
type Logger = logx.Logger
```

复用 Kernel `logx.Logger` 契约。nil logger → 使用 `logx.Noop()`。

### 2.7 NewManager

```go
func NewManager(meter metric.Meter, logger Logger) Manager
```

- nil meter → 返回 nilManager（no-op）
- nil logger → 使用 `logx.Noop()`
- 非 nil meter → 返回 otelManager

### 2.8 NewPrometheusManager / NewPrometheusMeter

```go
func NewPrometheusMeter(appName, appVersion string) metric.Meter
func NewPrometheusManager(appName, appVersion string, logger Logger) Manager
```

- 创建 Prometheus exporter + otel MeterProvider
- 资源属性包含 `service.name` 和 `service.version`
- 失败时返回 nil（meter）或 nilManager（manager）

### 2.9 GetHandler

```go
func GetHandler(m Manager) http.Handler
```

返回的 handler 必须服务：
- `GET /metrics`：Prometheus 格式 + 自动 system metrics
- `GET /debug/pprof/cmdline` / `/profile` / `/symbol` / `/trace` / `/`：pprof endpoints

每次 `/metrics` 请求时自动记录：
- `app_go_goroutines`
- `app_sys_memory_alloc`
- `app_sys_total_alloc`
- `app_go_num_gc`
- `app_go_sys`

### 2.10 RegisterSystemMetrics

注册 5 个 system gauge metric 名字，使它们在第一次 `/metrics` scrape 前就存在。

### 2.11 DefaultBuckets

```go
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
```

用于延迟直方图的默认 bucket 边界（秒）。

### 2.12 Errors

```go
type ErrAlreadyRegistered struct{ Name string }
type ErrNotRegistered struct{ Name string }
```

`Error()` 输出格式：

```text
metricsx: metric "<name>" already registered
metricsx: metric "<name>" not registered
```

## 3. Breaking Change 判定

以下变更必须作为 breaking change 记录：

- 删除或修改 `Manager` 接口方法签名
- 修改 `NewManager` / `NewPrometheusManager` / `NewPrometheusMeter` 签名
- 修改 `Noop()` 返回类型
- 修改 `GetHandler` 返回的 endpoint 路径
- 修改 `DefaultBuckets` 的值（影响现有 dashboard 查询）
- 修改 `Logger` 类型别名或 `logx.Logger` 契约
- 修改 `ErrAlreadyRegistered` / `ErrNotRegistered` 的 `Error()` 输出格式
- 修改 nil Manager 的行为（从 no-op 改为 panic）
- 修改标签基数限制（从 20 改为其他值，会改变 warning 行为）

## 4. 验收命令

```bash
go test ./metricsx -v
go test ./metricsx -race
go test ./metricsx -cover
go test ./...
go vet ./...
go run ./examples/metricsx-basic
# then: curl http://localhost:9001/metrics
```
