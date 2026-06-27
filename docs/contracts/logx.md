# logx Contract

模块：`github.com/aisphereio/kernel/logx`
验收等级目标：L3 可生产试用
版本：v0.1.0-alpha.1

## 1. Contract 目标

logx 是 Kernel 的统一日志包。它只负责结构化日志的创建和输出，不负责
审计级记录、不调用 os.Exit、不写网络。其他模块（errorx / httpx / grpcx /
auditx / metricsx）通过稳定的 Logger 接口 + Field 类型消费 logx。

只要以下 contract 不被破坏，内部实现可以重构。

## 2. 不可破坏行为

### 2.1 Logger 接口

`logx.Logger` 接口的方法签名不可变：

```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    With(fields ...Field) Logger
    Named(name string) Logger
    WithContext(ctx context.Context) Logger
    Enabled(level LogLevel) bool
    Sync() error
}
```

### 2.2 Field 类型

`logx.Field` 是 struct（不是 slog.Attr 别名），字段为 `Key string` +
`Value any`。这避免了 logx ABI 泄漏 slog 引擎。

所有 Field 构造器签名不可变：

```go
String(key, value string) Field
Int(key string, value int) Field
Int64(key string, value int64) Field
Uint64(key string, value uint64) Field
Bool(key string, value bool) Field
Float64(key string, value float64) Field
Duration(key string, value time.Duration) Field
Time(key string, value time.Time) Field
Any(key string, value any) Field
Event(name string) Field
Err(err error) Field
Group(key string, fields ...Field) Field
```

### 2.3 Context 集成

| 输入 | 行为 |
|---|---|
| `FromContext(nil)` | 返回 `Noop()` |
| `FromContext(ctx)` 无 logger | 返回 `Noop()` |
| `FromContext(ctx)` 有 logger | 返回注入的 logger |
| `FromContextOr(nil, fallback)` | 返回 `fallback`（或 Noop if fallback nil） |
| `Inject(nil, logger, ...)` | 等价于 `Inject(context.Background(), ...)` |
| `Inject(ctx, nil, ...)` | 使用 `Noop()` 作为 logger |

### 2.4 Noop 行为

`Noop()` 返回的 logger：
- 所有 Debug/Info/Warn/Error 调用为 no-op
- `With` / `Named` / `WithContext` 返回自身
- `Enabled(level)` 永远返回 `false`
- `Sync()` 返回 `nil`

### 2.5 Redaction 默认 key

`DefaultRedactKeys()` 必须包含（小写、归一化后匹配）：

```text
password, passwd, pwd, token, access_token, refresh_token, id_token,
secret, client_secret, authorization, cookie, set_cookie,
api_key, private_key, ak, sk
```

`NewRedactor(cfg)` 在 `cfg.Enabled=true` 时脱敏所有匹配 key 的 field。

### 2.6 logx.Err 字段提取

`logx.Err(err)` 通过鸭子类型（errors.As）提取以下字段，**不 import errorx**：

| 接口 | 提取字段 |
|---|---|
| `Code() string` | `error_code` |
| `ErrorCode() string` | `error_code` |
| `Reason() string` | `error_reason` |
| `HTTPStatus() int` | `http_status` |
| `StatusCode() int` | `http_status` |
| `Retryable() bool` | `retryable` |
| `GRPCCode() string` | `grpc_code` |
| `GRPCCode() int` | `grpc_code`（int 形式） |

nil error → 不附加 error 字段。

### 2.7 日志级别

`LogLevel` 类型为 string，可选值：`debug` / `info` / `warn` / `error`。

| 输入 | ParseLogLevel 返回 |
|---|---|
| `""` / `"info"` | `InfoLevel, nil` |
| `"debug"` | `DebugLevel, nil` |
| `"warn"` / `"warning"` | `WarnLevel, nil` |
| `"error"` | `ErrorLevel, nil` |
| 其他 | `"", error` |

`LevelController.SetLevel` 必须支持运行时动态调整，且线程安全。

### 2.8 Access log / External call 自动级别

`LogAccess` 自动按状态码选级别：

| 条件 | 级别 |
|---|---|
| `Err != nil` 或 `StatusCode >= 500` | Error |
| `StatusCode >= 400` | Warn |
| 其他 | Info |

`LogExternalCall` 同样规则。

### 2.9 Duration 字段

`logx.Duration(key, value)` 必须同时输出两个属性：

- `<key>`: 原始 duration（slog.Duration 格式）
- `<key>_ms`: 毫秒整数（方便 Loki/ELK 聚合）

### 2.10 Format 常量

| 常量 | 值 | 行为 |
|---|---|---|
| `FormatJSON` | `"json"` | slog.NewJSONHandler |
| `FormatText` | `"console"` | slog.NewTextHandler |
| `FormatConsole` | `"console"` | FormatText 别名 |

`DefaultConfig("dev")` 返回 `FormatConsole`；其他返回 `FormatJSON`。

### 2.11 DefaultConfig 行为

| env | format | level | addSource | sampling |
|---|---|---|---|---|
| `dev` / `local` | console | info | true | off |
| `staging` / `prod` / 其他 | json | info | false | on (cfg.Sampling.Enabled=false，但配置就绪) |

`DefaultConfig("")` 等价于 `DefaultConfig("dev")`。

## 3. Breaking Change 判定

以下变更必须作为 breaking change 记录：

- 删除或修改 `Logger` 接口方法签名
- 修改 `Field` 类型（struct → 别名，或字段顺序）
- 删除任何 Field 构造器
- 修改 `DefaultRedactKeys()` 返回的 key 列表（减少 key 是 breaking）
- 修改 `logx.Err` 提取的字段名（如把 `error_code` 改成 `code`）
- 修改 `LogLevel` 可选值
- 修改 `DefaultConfig` 的默认 format / level / addSource
- 修改 `FromContext(nil)` / `Inject(nil, ...)` 的行为
- 修改 `Noop()` 的 `Enabled` 返回值

## 4. 验收命令

```bash
go test ./logx -v
go test ./logx -race
go test ./logx -cover
go test ./logx -bench=.

go test ./...                # 全量测试不破坏
go vet ./...
go run ./examples/logx-basic
go run ./examples/logx-http
```

可选 benchmark 检查：

```bash
go test ./logx -bench=BenchmarkInfo -benchmem
```
