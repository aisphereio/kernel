# configx 契约

本契约对 Kernel 及业务模块是强制性要求。

## 包使用规则

`github.com/aisphereio/kernel/configx` 是唯一的 Kernel 配置运行时入口。

在 Kernel/业务 API 配置读取路径中禁止以下导入：

```go
import "github.com/aisphereio/kernel/config"   // 该包已删除
import "github.com/spf13/viper"                  // 第三方配置库
```

业务代码也禁止直接调用 `os.Getenv` / `os.LookupEnv` / `yaml.Unmarshal` / `json.Unmarshal` 读取配置。这些只能在 boot 层与测试代码中使用。

## 公共 API

以下符号保持 source 兼容，不可以在不升 major 版本的情况下破坏：

```go
type Config interface {
    Load() error
    Scan(v any) error
    Value(key string) Value
    Watch(key string, o Observer) error
    Close() error
}

type Source interface {
    Load() ([]*KeyValue, error)
    Watch() (Watcher, error)
}

type Watcher interface {
    Next() ([]*KeyValue, error)
    Stop() error
}

type Value interface {
    Bool() (bool, error)
    Int() (int64, error)
    Float() (float64, error)
    String() (string, error)
    Duration() (time.Duration, error)
    Slice() ([]Value, error)
    Map() (map[string]Value, error)
    Scan(any) error
    Load() any
    Store(any)
}

type KeyValue struct {
    Key    string
    Value  []byte
    Format string
}

type Observer func(string, Value)

func New(opts ...Option) Config
func Get[T any](c Config, key string) (T, error)
func MustGet[T any](c Config, key string) T
func GetOrDefault[T any](c Config, key string, fallback T) T

func WithSource(s ...Source) Option
func WithDecoder(d Decoder) Option
func WithResolver(r Resolver) Option
func WithResolveActualTypes(enable bool) Option
func WithMergeFunc(m Merge) Option
```

## 合并契约

Sources 按 `WithSource` 传入顺序合并：

- 后面的 source 覆盖前面的叶子值
- 嵌套 `map[string]any` 递归合并
- 数组（`[]any`）整体替换，不按下标合并
- `map[any]any`（YAML 解析常见）会先转成 `map[string]any` 再合并

## 占位符契约

`defaultResolver` 支持以下占位符格式：

```text
${KEY}               引用同棵配置树中的 key
${KEY:default}       缺失时使用 default
${server.http.addr}  支持 dot path
```

缺失值且无 default 时，占位符解析为空字符串。

`WithResolveActualTypes(true)` 仅当整个字段就是一个占位符时，会尝试把结果转为 bool / int64 / float64。混合字符串中的占位符仍按字符串替换。

## 错误契约

| 错误 | 触发条件 |
|---|---|
| `ErrNotFound` | `Value(key)` 找不到 key，或 `Get[T]` 在 key 不存在时返回 |
| `ErrClosed` | 在已 Close 的 Config 上调用 `Load` / `Watch` |
| `ErrInvalidObserver` | `Watch` 传入 nil observer |
| `ErrNilConfig` | `Get` / `MustGet` / `GetOrDefault` 传入 nil Config |

`MustGet` 在以上任一错误发生时 panic。

## Watch 契约

- `Watch(key, observer)` 要求 key 在注册时已存在；否则返回 `ErrNotFound`
- 同一 key 支持多个 observer，按注册顺序调用
- observer 回调在 configx 内部 goroutine 中执行，调用方需自行保证线程安全
- observer 不应阻塞；阻塞会延迟后续 observer 与下一次 reload
- observer panic 会被 recover，但会导致同次 reload 的后续 observer 不被调用

## Close 契约

- `Close` 是幂等的，多次调用返回 nil
- `Close` 后所有 Watcher 被停止
- `Close` 后 `Load` 返回 `ErrClosed`
- `Close` 后 `Value` 仍可读取已缓存的值（不清空缓存）

## 类型转换契约

`Value.Int()` 接受 `int* / uint* / float* / 数字字符串`，返回 `int64`。
`Value.Bool()` 接受 `bool / "true" / "false" / 数字`（0 为 false，非 0 为 true）。
`Value.Float()` 接受 `int* / uint* / float* / 数字字符串`，返回 `float64`。
`Value.String()` 接受任意类型，对 `string / 数字 / bool / []byte / fmt.Stringer` 直接转换，其他类型返回 error。
`Value.Duration()` 仅接受 `int*`（视为纳秒）与可被 `time.ParseDuration` 解析的字符串。

类型不匹配时返回 error，不 panic，不返回零值。

## 迁移契约

- 旧 `config/` 包已从根模块删除，不可重新创建
- 旧 `config/file` / `config/env` 子包已删除
- contrib/config/* 子模块全部返回 `configx.Source`
- 所有 `config.New` 调用替换为 `configx.New`
- 所有 `config.WithSource` 调用替换为 `configx.WithSource`

## 与 errorx 的边界

配置错误不应该被业务代码包装成 errorx 错误。理由：

- 配置错误一般在启动期发生，应直接 fail-fast 让进程退出
- errorx 主要面向请求期错误（用户可见的 API 错误）
- 把 `ErrNotFound` 包成 `errorx.NotFound` 会让"启动期缺配置"和"请求期资源不存在"语义混淆

启动期发现配置缺失时，推荐做法：

```go
if err := cfg.Load(); err != nil {
    log.Fatalf("load config failed: %v", err)  // 或 return err 给 main
}
```

请求期不读配置（启动期 Scan 到 struct，请求期用 struct 字段）。
