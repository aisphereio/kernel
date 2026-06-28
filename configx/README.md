# configx

`configx` 是 Aisphere Kernel 的统一配置模块。它负责把 file、env、remote source 加载成一棵可查询、可 Scan、可 Watch 的配置树，是 Kernel 中**唯一的**运行时配置读取入口。

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 configx

在 configx 之前，业务系统通常同时使用本地 YAML/JSON、环境变量、Kubernetes ConfigMap、Nacos、Apollo、etcd、Consul 等配置来源。如果每个模块直接读取 `os.Getenv`、直接解析 YAML，最终会出现三类问题：启动顺序不一致、默认值规则不一致、热更新行为不一致。

`configx` 把这些差异收敛成一套 Source / Watcher / Value 契约。业务代码只依赖 `configx.Config`，不关心配置来自文件、环境变量还是远程配置中心。这样后续迁移配置来源时，只需要替换 Source，不需要改业务模块。

`configx` 的核心契约：**配置来源可以有很多个，但业务读取配置只能通过 Config / Value / Scan 这一套稳定接口。**

```text
configx Source(file/env/remote)
  ↓ Load() []*KeyValue
configx Reader merge + resolve
  ↓ stable tree
Config.Value / Config.Scan / Config.Watch
  ↓ logx/dbx/httpx/grpcx/authx/cachex 等模块消费结构化配置
```

`configx` 本身不负责校验所有业务字段、不启动服务、不记录审计、不绑定具体配置中心 SDK。它只负责加载、合并、解析占位符、类型读取、热更新通知。

---

## 2. 30 秒上手

```go
package bootstrap

import (
    "github.com/aisphereio/kernel/configx"
    "github.com/aisphereio/kernel/configx/file"
)

type ServerConfig struct {
    HTTP struct {
        Addr string `json:"addr"`
        Port int    `json:"port"`
    } `json:"http"`
}

func LoadServerConfig(path string) (ServerConfig, error) {
    cfg := configx.New(configx.WithSource(file.NewSource(path)))
    defer cfg.Close()

    if err := cfg.Load(); err != nil {
        return ServerConfig{}, err
    }

    var out ServerConfig
    if err := cfg.Value("server").Scan(&out); err != nil {
        return ServerConfig{}, err
    }
    return out, nil
}
```

配置示例：

```json
{
  "server": {
    "http": { "addr": "0.0.0.0", "port": 8000 }
  }
}
```

日常开发只需要记住：`New` 创建配置、`Load` 加载、`Value` 读取单项、`Scan` 映射结构体、`Watch` 监听变化。

---

## 3. 构造器与 helper 速查

| 场景 | API | 说明 | Example |
|---|---|---|---|
| 创建配置实例 | `configx.New(opts...)` | 创建 Config，默认启用 JSON/YAML/XML/Proto codec | `ExampleNew` |
| 读取强类型值 | `configx.Get[T](cfg, key)` | 读取并转换成 `T` | `ExampleGet` |
| 启动期必须存在 | `configx.MustGet[T](cfg, key)` | 不存在或类型错误时 panic | `ExampleMustGet` |
| 有默认值读取 | `configx.GetOrDefault[T](cfg, key, fallback)` | 缺失或转换失败时返回 fallback | `ExampleGetOrDefault` |

```go
cfg := configx.New(configx.WithSource(file.NewSource("configs/app.yaml")))
_ = cfg.Load()
addr := configx.MustGet[string](cfg, "server.http.addr")
port := configx.GetOrDefault[int](cfg, "server.http.port", 8000)
```

`Get[T]` 原生支持 `bool / int / int64 / float64 / string` 与任意带 `json` tag 的 struct。其他类型请用 `Value(key).Scan(&v)`。

---

## 4. Option 列表

构造时按需附加：

```go
configx.WithSource(src...)                 // 配置来源：file/env/remote source
configx.WithDecoder(decoder)               // 自定义 KeyValue 解码逻辑
configx.WithResolver(resolver)             // 自定义占位符解析逻辑
configx.WithResolveActualTypes(true)       // ${PORT} 解析为 int/bool/float 等实际类型
configx.WithMergeFunc(merge)               // 自定义多 source 合并策略
```

推荐顺序：基础文件 source 放前面，覆盖层 source 放后面。

```go
cfg := configx.New(configx.WithSource(
    file.NewSource("configs/common.yaml"),
    file.NewSource("configs/dev.yaml"),
    env.NewSource("KERNEL_"),
))
```

后面的 source 会覆盖前面的叶子值，但嵌套 map 会递归合并。

---

## 5. Source / Watcher 契约

`Source` 是配置输入适配器：

```go
type Source interface {
    Load() ([]*KeyValue, error)
    Watch() (Watcher, error)
}
```

`Watcher` 是热更新适配器：

```go
type Watcher interface {
    Next() ([]*KeyValue, error)
    Stop() error
}
```

内置 source：

| Source | 包 | 场景 |
|---|---|---|
| 文件 | `configx/file` | 本地 JSON/YAML/XML/Proto 配置 |
| 环境变量 | `configx/env` | 容器部署、CI/CD 注入 |
| Apollo | `contrib/config/apollo` | Apollo 配置中心 |
| Nacos | `contrib/config/nacos` | Nacos 配置中心 |
| etcd | `contrib/config/etcd` | etcd KV 配置 |
| Consul | `contrib/config/consul` | Consul KV 配置 |
| Kubernetes | `contrib/config/kubernetes` | ConfigMap / Secret |
| Polaris | `contrib/config/polaris` | Polaris 配置中心 |

每个 contrib source 都返回 `configx.Source`，所以可以无缝替换 file/env。

---

## 6. 合并规则

默认 merge 是"嵌套 map 递归合并，叶子值后者覆盖前者"：

```json
// common.json
{ "server": { "addr": "0.0.0.0", "port": 8000 } }

// prod.json
{ "server": { "port": 9000 } }
```

最终结果：

```json
{ "server": { "addr": "0.0.0.0", "port": 9000 } }
```

数组不是递归合并，而是整体覆盖。这样可以避免数组项按下标合并导致不可预测结果。

可以用 `WithMergeFunc` 替换默认合并策略，例如实现"深合并并保留数组顺序"或"按 key 合并数组"。

---

## 7. 占位符解析

默认占位符格式：

```text
${KEY}
${KEY:default}
${server.http.addr}
```

例子：

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

解析结果：

```json
{
  "server": {
    "addr": "127.0.0.1:8000",
    "mode": "dev"
  }
}
```

如果启用 `WithResolveActualTypes(true)`，当整个字段就是一个占位符时，`"${PORT}"` 会转换为 int，`"${ENABLED}"` 会转换为 bool，`"${RATIO}"` 会转换为 float。

---

## 8. Value 读取规则

`Value` 提供统一类型转换：

```go
v := cfg.Value("server.port")
port, err := v.Int()
addr, err := cfg.Value("server.addr").String()
enabled, err := cfg.Value("feature.enabled").Bool()
timeout, err := cfg.Value("upstream.timeout_ns").Duration()
```

| 方法 | 支持输入 | 输出 |
|---|---|---|
| `Bool()` | bool/string/number | bool |
| `Int()` | int/uint/float/string | int64 |
| `Float()` | int/uint/float/string | float64 |
| `String()` | string/number/bool/[]byte/Stringer | string |
| `Duration()` | int/string | time.Duration |
| `Slice()` | []any | []Value |
| `Map()` | map[string]any | map[string]Value |
| `Scan()` | any JSON/proto-compatible value | struct/proto |

`Value` 实现了原子 Load/Store，所以同一 `Value` 对象在 Watch 回调刷新后，引用它的代码会自动看到新值（不重新调用 `cfg.Value(key)`）。

---

## 9. Scan 结构体

推荐用 `Scan` 接收模块配置：

```go
type LogConfig struct {
    Level  string `json:"level"`
    Format string `json:"format"`
}

var logCfg LogConfig
if err := cfg.Value("log").Scan(&logCfg); err != nil {
    return err
}
```

大型模块建议每个模块定义自己的 Config 结构体，例如 `logx.Config`、`dbx.Config`、`httpx.Config`，启动时由 boot 层统一 Scan。

`Config.Scan(v)` 会把整棵配置树 marshal 成 JSON 再 unmarshal 进 `v`，所以 struct 必须有 `json` tag。

---

## 10. Watch 热更新

`Watch` 用于监听某个 key 的变化：

```go
_ = cfg.Watch("log.level", func(key string, value configx.Value) {
    level, _ := value.String()
    logger.SetLevel(level)
})
```

本版本优化了旧实现的两个问题：
1. 同一个 key 支持多个 observer。
2. Reload 后会刷新已缓存的 Value，即使新旧值类型发生变化也会更新。

注意：`Watch` 是配置变化通知，不是业务事件总线。回调里不要做耗时操作，不要访问网络，不要阻塞。

`Watch` 要求 key 在注册时已存在；否则返回 `ErrNotFound`。这样能避免"启动期配置缺失但被静默忽略"的隐患。

---

## 11. 错误处理

常见错误：

| 错误 | 含义 | 处理方式 |
|---|---|---|
| `configx.ErrNotFound` | key 不存在 | 启动必选项直接失败，可选项用默认值 |
| `configx.ErrClosed` | Config 已关闭 | 不要复用已 Close 的 Config |
| `configx.ErrInvalidObserver` | Watch 传入 nil observer | 修复调用方 |
| `configx.ErrNilConfig` | Get/MustGet 传入 nil Config | 修复启动注入 |

业务层不要把配置错误包装成业务 errorx。配置错误一般是启动期错误，应在 boot 层直接返回并终止进程。请求路径上的配置缺失通常意味着代码 bug，应直接 panic 或 fail-fast，不应让 errorx 处理。

---

## 12. 禁止模式

```go
// ❌ 禁止：业务代码直接读环境变量
os.Getenv("DATABASE_DSN")

// ✅ 推荐：boot 层统一加载，业务拿结构体
cfg.Value("database.dsn").String()

// ❌ 禁止：每个模块自己解析 YAML
os.ReadFile("config.yaml")
yaml.Unmarshal(data, &cfg)

// ✅ 推荐：统一 Source + Scan
configx.New(configx.WithSource(file.NewSource("config.yaml")))

// ❌ 禁止：在 Watch 回调里做慢操作
cfg.Watch("x", func(string, configx.Value) { callRemoteAPI() })

// ✅ 推荐：回调只更新本地原子配置或发送轻量信号

// ❌ 禁止：把启动期 Load 错误吞掉
if err := cfg.Load(); err != nil {
    log.Printf("load failed: %v", err)  // 启动失败被吞
}

// ✅ 推荐：直接返回 / panic，让进程退出
if err := cfg.Load(); err != nil {
    return err
}
```

---

## 13. 测试建议

每个使用 configx 的模块至少要覆盖三类测试：

1. 默认配置可以 Scan 到模块 Config。
2. 缺失必选项能返回明确错误。
3. 覆盖层配置能覆盖默认值。

如果模块支持热更新，再增加 Watch 测试。

测试代码可以使用 `configx.New(configx.WithSource(exampleSource{...}))` 直接构造内存 source，不写临时文件。参考 `configx/example_test.go` 顶部的 `exampleSource` 实现。

---

## 14. 本次工程化优化点

本轮从 `config/` 迁移到 `configx/`，并做了这些工程化优化：

| 优化 | 旧问题 | 新行为 |
|---|---|---|
| 包本地化 | 目录仍叫 `config`，和 Go 常见变量名冲突 | 统一为 `configx` |
| 缓存刷新 | `Load()` 后已缓存 Value 可能还是旧值 | `Load()` / Watch 后刷新缓存 |
| 类型变化 | Watch 更新时新旧类型不同会跳过 | Value 可跨类型 Store |
| observer | 同一个 key 只能一个 observer | 支持多个 observer |
| Close | 重复 Close 可能返回底层错误 | Close 幂等 |
| clone | reader clone 依赖 gob | 使用递归 clone，减少类型脆弱性 |
| helper | 只有 `Get[T]` | 新增 `MustGet[T]`、`GetOrDefault[T]` |
| 文档 | README 太薄 | 按 skill 建立 README/doc/AI/examples/contracts |
| 测试 | 只覆盖基本路径 | 新增 contract / integration / coverage_edge / benchmark / fuzz |

---

## 15. 迁移指南：config → configx

旧代码（已删除）：

```go
import "github.com/aisphereio/kernel/config"
import "github.com/aisphereio/kernel/config/file"

cfg := config.New(config.WithSource(file.NewSource("app.yaml")))
```

新代码：

```go
import "github.com/aisphereio/kernel/configx"
import "github.com/aisphereio/kernel/configx/file"

cfg := configx.New(configx.WithSource(file.NewSource("app.yaml")))
```

如果是 contrib source，也全部返回 `configx.Source`：

```go
src := nacos.NewConfigSource(client, nacos.WithDataID("app.yaml"))
cfg := configx.New(configx.WithSource(src))
```

迁移检查清单：

- [ ] 全仓库搜索 `kernel/config"` 与 `kernel/config/` 已无业务代码引用
- [ ] contrib/config/* 子模块的 import 路径全部更新
- [ ] `os.Getenv` 在 business 代码中已替换为 `cfg.Value`
- [ ] 启动期 `Load` 错误直接 fail-fast
- [ ] `defer cfg.Close()` 已添加

---

## 16. 文档地图

configx 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (configx/README.md)              ← 单一入口
├── configx/doc.go                          ← go doc 输出源
└── configx/example_test.go                 ← Go 标准示例（go test -v 可看输出）

深度规范（架构师/PR review 时看）
├── docs/design/configx.md                  ← 设计规范
└── docs/contracts/configx.md               ← 不可破坏契约

AI 编码指南（AI 写业务代码时看）
├── docs/ai/configx.md                      ← 合并版 AI 指南
└── AGENTS.md                               ← 项目级 AI 规则

验收与运维（CI/CD 时看）
└── docs/process/configx-acceptance-checklist.md

可运行示例
├── examples/configx-basic/                 ← 最小示例：file + Scan
├── examples/configx-env/                   ← env + file 覆盖示例
└── examples/configx-watch/                 ← Watch 热更新示例
```

**优先级**：日常开发只看本 README + `docs/ai/configx.md` 即可。其他文档按场景查阅，无需通读。

---

## 17. Examples 索引（按场景查找）

### 构造器与 helper 示例

| API | Example 函数 | 何时用 |
|---|---|---|
| `New` | `ExampleNew` | 创建配置实例 |
| `Get` | `ExampleGet` | 读取强类型值 |
| `MustGet` | `ExampleMustGet` | 启动期必需配置 |
| `GetOrDefault` | `ExampleGetOrDefault` | 可选配置默认值 |

### Option 示例

| API | Example 函数 | 何时用 |
|---|---|---|
| `WithSource` | `ExampleWithSource` | 组合 file/env/remote source |
| `WithDecoder` | `ExampleWithDecoder` | 自定义格式解析 |
| `WithResolver` | `ExampleWithResolver` | 自定义占位符解析 |
| `WithResolveActualTypes` | `ExampleWithResolveActualTypes` | 占位符转换为实际类型 |
| `WithMergeFunc` | `ExampleWithMergeFunc` | 自定义覆盖策略 |

### Config / Value 示例

| 场景 | Example 函数 |
|---|---|
| 读取单项 | `ExampleConfig_Value` |
| Scan 结构体 | `ExampleConfig_Scan` |
| Watch 热更新 | `ExampleConfig_Watch` |
| Bool 转换 | `ExampleValue_Bool` |
| Int 转换 | `ExampleValue_Int` |
| Float 转换 | `ExampleValue_Float` |
| String 转换 | `ExampleValue_String` |
| Slice 读取 | `ExampleValue_Slice` |
| Map 读取 | `ExampleValue_Map` |
| 局部 Scan | `ExampleValue_Scan` |

### 业务场景示例（10 个）

| 场景 | Example 函数 |
|---|---|
| 启动 HTTP 服务 | `Example_businessBootstrapHTTPServer` |
| 文件 + 环境覆盖 | `Example_businessLayeredFileAndEnv` |
| 数据库配置 Scan | `Example_businessScanDatabaseConfig` |
| Feature flag | `Example_businessFeatureFlag` |
| 上游 timeout | `Example_businessUpstreamTimeoutConfig` |
| 占位符解析 | `Example_businessResolvePlaceholder` |
| typed placeholder | `Example_businessActualTypedPlaceholder` |
| runtime change | `Example_businessObserveRuntimeChange` |
| 必需配置校验 | `Example_businessValidateRequiredConfig` |
| 自定义 dotenv decoder | `Example_businessCustomDecoderForDotEnv` |

---

## 18. 发版前检查

```bash
# 单元测试
go test ./configx ./configx/env ./configx/file

# 标准示例
go test ./configx -run=Example -v

# 模块文档完整性检查
./scripts/check-module-docs.sh configx

# 禁止模式扫描
./scripts/check-configx-usage.sh

# 种子模糊测试
go test ./configx -run=^$ -fuzz=FuzzConfigLoad -fuzztime=30s
```

Windows PowerShell 如果执行脚本被拦截，使用仓库里的 `.cmd` 包装脚本，或者用 `powershell -ExecutionPolicy Bypass -File ...` 临时绕过。

---

## 19. 设计哲学一句话

> 配置必须有 source。
> 配置必须可合并。
> 配置必须可观测。
> 配置必须可热更新。
> configx 只定义加载与读取语义，不负责校验、不负责服务、不负责审计。
