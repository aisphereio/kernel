# configx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的配置处理规范。**只看本文件即可写对所有配置场景。**

---

## 0. 一句话规则

> 业务代码（handler/service/repository/worker）读取配置时，**必须**使用 `github.com/aisphereio/kernel/configx`。
> **禁止**使用 `os.Getenv` / `yaml.Unmarshal` / `json.Unmarshal` / `viper.Get` 等直接读取方式。

---

## 1. 速查：什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| 启动期加载配置 | `cfg.Load()` | 进程启动时调用一次 |
| 启动期必须值 | `configx.MustGet[T](cfg, key)` | 缺失即 panic，启动失败 |
| 启动期可选值 | `configx.GetOrDefault[T](cfg, key, fallback)` | 缺失用 fallback |
| 启动期强类型读取 | `configx.Get[T](cfg, key)` | 缺失返回 `ErrNotFound` |
| 模块整体配置 | `cfg.Value("module").Scan(&moduleCfg)` | 推荐方式 |
| 请求期单值读取 | `cfg.Value("a.b.c").String()` | 仅用于动态值 |
| 热更新监听 | `cfg.Watch("key", fn)` | 不阻塞，回调轻量 |
| 关闭配置 | `defer cfg.Close()` | 进程退出前 |

**关键原则**：启动期 Scan 到结构体，请求期使用结构体字段，不要每次请求都读 `cfg.Value`。

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 启动期加载 file + env 配置

```go
package bootstrap

import (
    "github.com/aisphereio/kernel/configx"
    "github.com/aisphereio/kernel/configx/env"
    "github.com/aisphereio/kernel/configx/file"
)

func LoadConfig(baseDir string) (configx.Config, error) {
    cfg := configx.New(configx.WithSource(
        file.NewSource(baseDir+"/configs/common.yaml"),
        file.NewSource(baseDir+"/configs/"+env()+".yaml"),
        env.NewSource("KERNEL_"),
    ))
    if err := cfg.Load(); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

后面的 source 覆盖前面的叶子值。`KERNEL_` 前缀的环境变量会自动剥离前缀并按 `_` 分割成路径。

### 2.2 启动期必需值

```go
secret := configx.MustGet[string](cfg, "jwt.secret")
if secret == "" {
    panic("jwt.secret must not be empty")  // MustGet 只保证 key 存在；空字符串仍需业务校验
}
```

`MustGet` 在 key 缺失或类型转换失败时 panic。**只**在 main / bootstrap 阶段使用，不要在 handler 里调用。

### 2.3 启动期可选值（带默认值）

```go
port := configx.GetOrDefault[int](cfg, "server.port", 8000)
maxConn := configx.GetOrDefault[int](cfg, "db.max_open_conns", 32)
```

缺失或类型转换失败都返回 fallback。

### 2.4 模块整体配置 Scan（推荐）

```go
type DBConfig struct {
    Driver          string `json:"driver"`
    DSN             string `json:"dsn"`
    MaxOpenConns    int    `json:"max_open_conns"`
    MaxIdleConns    int    `json:"max_idle_conns"`
    ConnMaxLifetime int    `json:"conn_max_lifetime_ns"`  // 纳秒
}

var dbCfg DBConfig
if err := cfg.Value("database").Scan(&dbCfg); err != nil {
    return fmt.Errorf("scan database config: %w", err)
}
if dbCfg.Driver == "" {
    return errors.New("database.driver is required")
}
```

`Scan` 会把配置子树 marshal 成 JSON 再 unmarshal 到 struct，所以 struct 必须有 `json` tag。

### 2.5 文件 + 环境变量覆盖

```go
cfg := configx.New(configx.WithSource(
    file.NewSource("configs/common.yaml"),     // 基础
    file.NewSource("configs/prod.yaml"),        // 环境覆盖
    env.NewSource("KERNEL_"),                   // 部署时注入
))
```

**注意**：env source 产出的是扁平 key/value，不会自动把 `KERNEL_APP_ENV` 转成 `app.env` 路径。
- `KERNEL_APP_ENV=prod` → Key=`APP_ENV`，Value=`prod`（顶层 key，不是 `app.env`）
- 想覆盖嵌套路径，有三种做法：
  1. 在文件里写占位符：`"env": "${APP_ENV:dev}"`，然后 `KERNEL_APP_ENV=prod` 解析占位符
  2. 用 `WithResolveActualTypes(true)` 让 `"${SERVER_PORT}"` 解析为 int
  3. 自己实现一个 `Source`，把 `KERNEL_APP_ENV` 转成 `app.env` 路径（参考 `configx/env/env.go`）

参考 `examples/configx-env/main.go` 看占位符做法的完整示例。

### 2.6 占位符解析

```json
{
  "HOST": "127.0.0.1",
  "PORT": "8000",
  "server": {
    "addr": "${HOST}:${PORT}",
    "mode": "${MODE:dev}"
  }
}
```

`configx` 默认解析 `${KEY}` 和 `${KEY:default}`。`MODE` 缺失时使用 `dev`。

### 2.7 实际类型占位符

```go
cfg := configx.New(
    configx.WithSource(file.NewSource("configs/app.yaml")),
    configx.WithResolveActualTypes(true),
)
```

```json
{
  "PORT": "8080",
  "ENABLED": "true",
  "RATIO": "0.75",
  "server": {
    "port": "${PORT}",
    "enabled": "${ENABLED}",
    "ratio": "${RATIO}"
  }
}
```

启用 `WithResolveActualTypes(true)` 后，整个字段就是一个占位符时会转换为对应实际类型：int、bool、float。混合字符串仍为字符串。

### 2.8 Watch 热更新

```go
// 启动期注册回调，运行时配置变更会自动触发
err := cfg.Watch("log.level", func(key string, value configx.Value) {
    level, _ := value.String()
    if err := levelCtl.SetLevel(level); err != nil {
        logx.Error("failed to update log level", "error", err, "level", level)
    }
})
if err != nil {
    return fmt.Errorf("watch log.level: %w", err)
}
```

**铁律**：
- `Watch` 要求 key 在注册时已存在（否则返回 `ErrNotFound`）。
- 回调里不要做耗时操作、不要访问网络、不要阻塞。
- 回调里抛出 panic 会被 defer recover 兜住，但会导致后续 observer 不被调用——所以**不要 panic**。

### 2.9 校验必需配置

```go
requiredKeys := []string{
    "database.dsn",
    "jwt.secret",
    "server.http.addr",
}
for _, key := range requiredKeys {
    if _, err := configx.Get[string](cfg, key); err != nil {
        return fmt.Errorf("missing required config %s: %w", key, err)
    }
}
```

启动期统一校验所有必需 key，缺失任何一个都 fail-fast。

### 2.10 自定义 decoder（dotenv 等非标格式）

```go
decoder := func(kv *configx.KeyValue, target map[string]any) error {
    for _, line := range strings.Split(string(kv.Value), "\n") {
        key, value, ok := strings.Cut(line, "=")
        if ok {
            target[strings.TrimSpace(key)] = strings.TrimSpace(value)
        }
    }
    return nil
}

cfg := configx.New(
    configx.WithSource(file.NewSource(".env")),
    configx.WithDecoder(decoder),
)
```

`KeyValue.Format == ""` 时走自定义 decoder。`Format != ""` 时由 codec 按 format 解析（json/yaml/xml/proto）。

---

## 3. 配置 key 命名规则

格式：`{module}.{submodule}.{field}`，全小写蛇形。

```text
✅ database.dsn
✅ database.max_open_conns
✅ server.http.addr
✅ log.level
✅ jwt.secret

❌ DATABASE_DSN        （大写）
❌ database-dsn        （连字符）
❌ databaseDSN         （驼峰）
❌ dsn                 （过于宽泛）
```

环境变量映射规则：`KERNEL_APP_ENV` → 顶层 key `APP_ENV`（**不会**自动转成 `app.env` 路径）。

`env.NewSource("KERNEL_")` 会：
1. 找到所有以 `KERNEL_` 开头的环境变量
2. 剥离前缀和紧跟的下划线
3. 把剩下的字符串作为**完整 key**（`APP_ENV`），不做大小写转换、不做 `_` → `.` 转换

如果想用环境变量覆盖嵌套路径，请用占位符（推荐）或自己实现一个 Source 适配器。

---

## 4. 禁止模式

### 4.1 业务代码禁止

```go
// ❌ 直接读环境变量
dsn := os.Getenv("DATABASE_DSN")

// ❌ 自己解析 YAML/JSON
data, _ := os.ReadFile("config.yaml")
yaml.Unmarshal(data, &cfg)

// ❌ 用 viper 等第三方库
v := viper.New()
v.GetString("database.dsn")

// ❌ 在请求路径上每次读 cfg.Value
func (h *Handler) ServeHTTP(w, r) {
    addr := cfg.Value("server.addr").String()  // 配置不会变？启动期 Scan 到 struct
}

// ❌ 在 Watch 回调里做慢操作
cfg.Watch("x", func(k string, v configx.Value) {
    resp, _ := http.Get("https://config-center/reload")  // 阻塞回调
})

// ❌ 吞掉 Load 错误
if err := cfg.Load(); err != nil {
    log.Printf("warn: load failed: %v", err)  // 启动失败被吞
}
```

### 4.2 替代写法

```go
// ✅ 启动期 Scan 到结构体，请求期用 struct 字段
type ServerConfig struct {
    Addr string `json:"addr"`
    Port int    `json:"port"`
}
var serverCfg ServerConfig
_ = cfg.Value("server").Scan(&serverCfg)

// 在 handler 里直接用 serverCfg.Addr / serverCfg.Port

// ✅ Watch 回调只更新原子变量
var logLevel atomic.Value
_ = cfg.Watch("log.level", func(_ string, v configx.Value) {
    level, _ := v.String()
    logLevel.Store(level)
})

// ✅ Load 失败 fail-fast
if err := cfg.Load(); err != nil {
    return err  // 或 panic(err)
}
```

### 4.3 允许的例外

测试代码（`*_test.go`）可以使用 `os.Setenv` 来构造 env source 测试场景：

```go
func TestEnvOverride(t *testing.T) {
    os.Setenv("KERNEL_DATABASE_DSN", "postgres://test")  // ✅ 测试代码允许
    defer os.Unsetenv("KERNEL_DATABASE_DSN")

    cfg := configx.New(configx.WithSource(env.NewSource("KERNEL_")))
    _ = cfg.Load()
    // ...
}
```

---

## 5. Source / Watcher 契约

### 5.1 自定义 Source

如果你的配置来自非内置来源（例如自研配置中心），实现 `configx.Source` 接口即可：

```go
type mySource struct {
    endpoint string
}

func (s *mySource) Load() ([]*configx.KeyValue, error) {
    resp, err := http.Get(s.endpoint + "/config")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return []*configx.KeyValue{{
        Key:    "remote.json",
        Format: "json",
        Value:  data,
    }}, nil
}

func (s *mySource) Watch() (configx.Watcher, error) {
    // 不支持热更新就返回阻塞 watcher
    return &blockingWatcher{...}, nil
}
```

`KeyValue.Format` 决定 codec：`json` / `yaml` / `xml` / `proto` / `""`（裸 key=value）。

### 5.2 自定义 Watcher

```go
type myWatcher struct {
    ch chan []*configx.KeyValue
}

func (w *myWatcher) Next() ([]*configx.KeyValue, error) {
    kvs, ok := <-w.ch
    if !ok {
        return nil, errors.New("watcher closed")
    }
    return kvs, nil
}

func (w *myWatcher) Stop() error {
    close(w.ch)
    return nil
}
```

`Next` 应阻塞直到有新数据。返回 error 后 configx 会重试（默认 1 秒间隔），所以临时错误不要 panic。

---

## 6. Value 类型转换规则

`Value` 提供统一的类型转换，所有方法都返回 `(T, error)`：

| 方法 | 接受输入 | 输出 | 转换失败时 |
|---|---|---|---|
| `Bool()` | `bool`, `"true"/"false"`, 数字 | `bool` | 返回 error |
| `Int()` | `int*`, `uint*`, `float*`, 数字字符串 | `int64` | 返回 error |
| `Float()` | `int*`, `uint*`, `float*`, 数字字符串 | `float64` | 返回 error |
| `String()` | 任意类型（包括 `Stringer`） | `string` | 几乎不失败 |
| `Duration()` | `int*`（纳秒）, 时间字符串 | `time.Duration` | 返回 error |
| `Slice()` | `[]any` | `[]Value` | 类型不匹配返回 error |
| `Map()` | `map[string]any` | `map[string]Value` | 类型不匹配返回 error |
| `Scan(obj)` | 任意 JSON/proto 兼容值 | struct/proto | unmarshal 失败返回 error |

注意：`Duration()` 优先按 int 解释为纳秒。如果你想要 `"5s"` 这样的字符串，需要在业务层自己 `time.ParseDuration`。

---

## 7. 调试技巧

```go
// 查看整棵配置树（JSON 格式）
data, _ := cfg.(*config).reader.Source()
fmt.Println(string(data))

// 检查 key 是否存在
v := cfg.Value("maybe.missing")
if v.Load() == nil {
    fmt.Println("key missing")
}

// 测试时直接构造内存 source
src := exampleSource{format: "json", data: `{"app":{"name":"test"}}`}
cfg := configx.New(configx.WithSource(src))
```

---

## 8. 与其他模块的关系

```text
configx (只加载/合并/解析配置)
  ↓ Config / Value / Scan
logx     → cfg.Scan(&logx.Config)
dbx      → cfg.Scan(&dbx.Config)
httpx    → cfg.Scan(&httpx.Config)
grpcx    → cfg.Scan(&grpcx.Config)
authx    → cfg.Scan(&authx.Config)
metricsx → cfg.Scan(&metricsx.Config)
workerx  → cfg.Scan(&workerx.Config)
```

每个模块定义自己的 Config 结构体，boot 层统一 Scan。这样：
- 模块之间不互相依赖配置 key
- 启动期一次性校验所有必需字段
- 配置缺失在启动期就被发现，而不是请求期

### 标准模块 Config 模板

```go
// internal/config/config.go
type Config struct {
    App      AppConfig      `json:"app"`
    Server   ServerConfig   `json:"server"`
    Database DatabaseConfig `json:"database"`
    Log      LogConfig      `json:"log"`
    JWT      JWTConfig      `json:"jwt"`
}

type AppConfig struct {
    Name    string `json:"name"`
    Env     string `json:"env"`
    Version string `json:"version"`
}

type DatabaseConfig struct {
    Driver          string `json:"driver"`           // postgres | mysql
    DSN             string `json:"dsn"`
    MaxOpenConns    int    `json:"max_open_conns"`
    MaxIdleConns    int    `json:"max_idle_conns"`
    ConnMaxLifetime int    `json:"conn_max_lifetime_ns"`
}

// boot/boot.go
func Load(path string) (*Config, error) {
    cfg := configx.New(configx.WithSource(
        file.NewSource(path),
        env.NewSource("KERNEL_"),
    ))
    defer cfg.Close()

    if err := cfg.Load(); err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }

    var out Config
    if err := cfg.Scan(&out); err != nil {
        return nil, fmt.Errorf("scan config: %w", err)
    }

    // 启动期校验
    if out.Database.DSN == "" {
        return nil, errors.New("database.dsn is required")
    }
    if out.JWT.Secret == "" {
        return nil, errors.New("jwt.secret is required")
    }

    return &out, nil
}
```

---

## 9. 完整 handler 示例

```go
package handler

import (
    "context"
    "net/http"

    "github.com/aisphereio/kernel/errorx"
    "github.com/aisphereio/kernel/logx"
)

type SkillHandler struct {
    logger logx.Logger
    svc    SkillService
    cfg    *SkillHandlerConfig  // 启动期 Scan 到 struct
}

type SkillHandlerConfig struct {
    MaxNameLength int `json:"max_name_length"`
    DefaultPage   int `json:"default_page"`
}

func NewSkillHandler(logger logx.Logger, svc SkillService, cfg *SkillHandlerConfig) *SkillHandler {
    return &SkillHandler{logger: logger, svc: svc, cfg: cfg}
}

func (h *SkillHandler) Create(ctx context.Context, req *CreateSkillRequest) (*Skill, error) {
    // 请求期直接用 h.cfg 字段，不调 cfg.Value
    if len(req.Name) > h.cfg.MaxNameLength {
        return nil, errorx.BadRequest("AIHUB_SKILL_NAME_TOO_LONG", "技能名称过长",
            errorx.WithPublicMetadata("max", h.cfg.MaxNameLength),
        )
    }
    return h.svc.Create(ctx, &Skill{Name: req.Name})
}
```

启动期装配：

```go
// boot
var handlerCfg SkillHandlerConfig
if err := cfg.Value("handlers.skill").Scan(&handlerCfg); err != nil {
    return fmt.Errorf("scan handlers.skill: %w", err)
}
handler := handler.NewSkillHandler(logger, svc, &handlerCfg)
```

---

## 10. 验收清单（写完 configx 代码后自检）

- [ ] 所有 `os.Getenv` 都换成 `cfg.Value` 或启动期 `Scan`
- [ ] 启动期必需 key 用 `MustGet` 或启动期校验
- [ ] 业务代码不直接 import `yaml` / `json` / `viper`
- [ ] Watch 回调只更新本地原子变量或发送轻量信号
- [ ] `defer cfg.Close()` 已添加
- [ ] Load 错误直接 fail-fast，不被吞掉
- [ ] 测试代码 `go test ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `./scripts/check-configx-usage.sh` 通过

---

## 11. 相关文档

- `configx/README.md` — 单一入口用户指南
- `configx/doc.go` — `go doc configx` 输出源
- `configx/example_test.go` — 所有 API 的 Go 标准 Example
- `configx/example_business_test.go` — 10 个完整业务场景
- `configx/contract_test.go` — 不可破坏契约测试
- `configx/integration_test.go` — 跨模块集成测试
- `configx/coverage_edge_test.go` — 边界情况测试
- `configx/benchmark_test.go` — 性能基准
- `configx/fuzz_test.go` — 模糊测试
- `docs/design/configx.md` — 完整设计规范（深度参考）
- `docs/contracts/configx.md` — 不可破坏契约
- `docs/process/configx-acceptance-checklist.md` — CI 验收清单
- `examples/configx-basic/` — 最小可运行示例
- `examples/configx-env/` — env 覆盖示例
- `examples/configx-watch/` — Watch 热更新示例
