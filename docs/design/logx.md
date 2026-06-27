# Aisphere Kernel 日志系统设计规范

文件路径：`docs/design/logx.md`
模块名：`logx`
版本：v0.1-draft
适用仓库：`github.com/aisphereio/kernel`

---

# 1. 模块定位

`logx` 是 Aisphere Kernel 的统一日志模块。

它不是简单封装 `log.Println`，也不是只为了打印调试信息。
它的目标是为所有业务模块、Kernel 内部模块、starter、adapter 提供一套统一、结构化、可追踪、可审计、AI 友好的日志能力。

在 Aisphere Kernel 中，日志系统必须服务于以下目标：

```text
1. 让 AI 不再自己创建 logger
2. 让所有服务日志格式统一
3. 让日志天然带 request_id / trace_id / user_id / org_id / project_id
4. 让错误日志可以和 errorx / tracingx / metricsx / auditx 串联
5. 让日志可以输出到 stdout、文件、Loki、ELK、OTel Collector 等后端
6. 让业务代码只关心“记录什么业务事件”，不关心“日志怎么初始化、怎么输出、怎么采集”
```

一句话：

> `logx` 负责日志边界，业务只负责日志语义。

---

# 2. 设计原则

## 2.1 业务层禁止直接使用底层日志库

业务代码禁止直接使用：

```go
log.Println(...)
fmt.Println(...)
slog.Info(...)
zap.New(...)
logrus.New(...)
```

业务只能通过 Kernel 提供的 logger 输出日志：

```go
ctx.Log().Info("skill created",
	logx.String("skill_id", skill.ID),
	logx.String("project_id", req.ProjectID),
)
```

或者在没有请求上下文的启动阶段使用：

```go
app.Logger().Info("module registered",
	logx.String("module", module.Name()),
)
```

---

## 2.2 日志必须结构化

日志不能以字符串拼接为主：

```go
// 禁止
ctx.Log().Info("create skill " + skill.ID + " success")
```

必须使用结构化字段：

```go
// 推荐
ctx.Log().Info("skill created",
	logx.String("skill_id", skill.ID),
	logx.String("skill_name", skill.Name),
)
```

原因：

```text
1. 方便 Loki / ELK / ClickHouse 检索
2. 方便按 user_id / org_id / project_id 过滤
3. 方便和 trace / metrics / audit 关联
4. 方便 AI 后续分析日志
```

---

## 2.3 日志必须自动携带上下文字段

业务代码不应该每次手动传：

```text
request_id
trace_id
service
module
env
version
user_id
org_id
project_id
```

这些字段应该由 Kernel 自动注入。

业务只补充业务字段，例如：

```text
skill_id
agent_id
workflow_id
model_id
duration_ms
provider
upstream_status
```

---

## 2.4 日志和审计必须分开

日志不是审计。

日志用于：

```text
调试
排障
性能分析
链路追踪
运行状态观察
```

审计用于：

```text
合规
安全追责
权限变更记录
敏感操作留痕
用户行为追踪
```

因此：

```go
ctx.Log().Info(...)
```

不能替代：

```go
ctx.Audit().Record(...)
```

敏感操作必须同时满足：

```text
1. 有业务日志
2. 有审计事件
3. 有权限校验
```

---

## 2.5 日志模块只暴露 Kernel 抽象

`logx` 可以底层使用 `slog`、`zap` 或其他日志库，但业务层不能感知。

业务代码只能依赖：

```go
github.com/aisphereio/kernel/logx
```

不能依赖：

```go
log/slog
go.uber.org/zap
github.com/sirupsen/logrus
```

---

# 3. 包结构设计

推荐目录：

```text
logx/
├── logger.go          # Logger 接口
├── level.go           # 日志级别
├── field.go           # 字段类型
├── config.go          # 日志配置
├── context.go         # 上下文字段注入
├── noop.go            # Noop Logger
├── std.go             # 标准库 slog 实现，可选
├── zap.go             # zap 实现，可选
├── redactor.go        # 敏感信息脱敏
├── sampler.go         # 日志采样
├── writer.go          # 输出目标抽象
├── middleware.go      # HTTP/gRPC 日志中间件
└── testlogger.go      # 测试 logger
```

第一版可以先实现：

```text
logx/
├── logger.go
├── level.go
├── field.go
├── config.go
├── context.go
├── noop.go
└── std.go
```

---

# 4. Logger 接口设计

## 4.1 核心接口

```go
package logx

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)

	With(fields ...Field) Logger
	Named(name string) Logger
}
```

说明：

```text
Debug：开发调试信息，生产默认可关闭
Info：正常业务事件
Warn：异常但可恢复的问题
Error：失败事件，需要排查
With：追加上下文字段
Named：创建子 logger，例如 module 名称
```

---

## 4.2 暂不支持 Fatal / Panic

第一版不建议暴露：

```go
Fatal(...)
Panic(...)
```

原因：

```text
1. Kernel 应统一控制应用生命周期
2. 业务模块不应该直接终止进程
3. panic 应由 recover middleware 捕获并转 errorx
```

启动阶段遇到致命错误，应该返回 error，由 Kernel 决定退出。

---

# 5. 日志级别规范

## 5.1 Debug

用于开发调试和低频排障。

适合记录：

```text
配置加载细节
SQL 构建细节
缓存命中细节
外部请求原始摘要
内部状态变化
```

示例：

```go
ctx.Log().Debug("cache hit",
	logx.String("key", cacheKey),
	logx.String("resource_type", "skill"),
)
```

禁止在 Debug 中打印：

```text
密码
token
secret
完整请求体
完整响应体
大对象
用户隐私数据
```

---

## 5.2 Info

用于正常业务事件和关键流程节点。

适合记录：

```text
服务启动
模块注册
请求完成
资源创建成功
任务开始/完成
外部服务调用成功
```

示例：

```go
ctx.Log().Info("skill created",
	logx.String("skill_id", skill.ID),
	logx.String("project_id", skill.ProjectID),
)
```

---

## 5.3 Warn

用于异常但可恢复的问题。

适合记录：

```text
权限被拒绝
参数异常
外部服务短暂失败但已重试
缓存不可用但已降级
请求过慢
配置项使用默认值
```

示例：

```go
ctx.Log().Warn("authz denied",
	logx.String("action", "aihub.skill.publish"),
	logx.String("resource", "aihub:skill:"+skillID),
	logx.String("reason", decision.Reason),
)
```

---

## 5.4 Error

用于真实失败。

适合记录：

```text
数据库写入失败
外部服务调用失败
任务执行失败
消息处理失败
系统内部错误
无法恢复的业务失败
```

示例：

```go
ctx.Log().Error("create skill failed",
	logx.Error(err),
	logx.String("project_id", req.ProjectID),
)
```

---

# 6. Field 设计

## 6.1 基础字段类型

```go
package logx

type Field struct {
	Key   string
	Value any
}

func String(key string, value string) Field
func Int(key string, value int) Field
func Int64(key string, value int64) Field
func Bool(key string, value bool) Field
func Float64(key string, value float64) Field
func Duration(key string, value time.Duration) Field
func Time(key string, value time.Time) Field
func Any(key string, value any) Field
func Error(err error) Field
```

---

## 6.2 Error 字段规范

错误字段统一使用：

```go
logx.Error(err)
```

输出字段建议包含：

```json
{
  "error": "database timeout",
  "error_type": "*net.OpError",
  "error_code": "AIHUB_SKILL_CREATE_FAILED"
}
```

当错误来自 `errorx.Error` 时，必须自动提取：

```text
error_code
http_status
retryable
```

---

## 6.3 字段命名规范

字段统一使用 snake_case：

```text
request_id
trace_id
user_id
org_id
project_id
skill_id
model_id
duration_ms
error_code
```

禁止混用：

```text
requestId
RequestID
request-id
```

---

# 7. 标准日志字段

## 7.1 Kernel 自动字段

每条业务请求日志应该尽可能自动携带：

```text
timestamp
level
message
service
module
env
version
request_id
trace_id
span_id
user_id
org_id
project_id
tenant_id
caller
```

---

## 7.2 HTTP 访问日志字段

HTTP access log 应包含：

```text
method
path
route
status
latency_ms
remote_ip
user_agent
request_size
response_size
error_code
```

示例 JSON：

```json
{
  "timestamp": "2026-06-27T10:00:00.000Z",
  "level": "info",
  "message": "http request completed",
  "service": "aihub",
  "module": "httpx",
  "env": "dev",
  "version": "v0.1.0",
  "request_id": "req_123",
  "trace_id": "trace_abc",
  "user_id": "user_001",
  "org_id": "aisphere",
  "method": "POST",
  "path": "/v1/skills",
  "route": "/v1/skills",
  "status": 201,
  "latency_ms": 18,
  "error_code": ""
}
```

---

## 7.3 gRPC 访问日志字段

gRPC access log 应包含：

```text
grpc_service
grpc_method
grpc_code
latency_ms
peer_addr
error_code
```

---

## 7.4 外部调用日志字段

调用外部服务时建议包含：

```text
provider
endpoint
operation
status
latency_ms
retry_count
timeout_ms
error_code
```

示例：

```go
ctx.Log().Info("llm upstream call completed",
	logx.String("provider", "deepseek"),
	logx.String("operation", "chat.completions"),
	logx.Int("status", 200),
	logx.Duration("latency", elapsed),
)
```

---

# 8. 配置设计

## 8.1 配置结构

```go
package logx

type Config struct {
	Level       string `json:"level" yaml:"level"`
	Format      string `json:"format" yaml:"format"`
	Output      string `json:"output" yaml:"output"`
	AddSource   bool   `json:"add_source" yaml:"add_source"`

	ServiceName string `json:"service_name" yaml:"service_name"`
	Env         string `json:"env" yaml:"env"`
	Version     string `json:"version" yaml:"version"`

	Redact      RedactConfig  `json:"redact" yaml:"redact"`
	Sampling    SamplingConfig `json:"sampling" yaml:"sampling"`
	AccessLog   AccessLogConfig `json:"access_log" yaml:"access_log"`
}

type RedactConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Keys    []string `json:"keys" yaml:"keys"`
}

type SamplingConfig struct {
	Enabled    bool `json:"enabled" yaml:"enabled"`
	First      int  `json:"first" yaml:"first"`
	Thereafter int  `json:"thereafter" yaml:"thereafter"`
}

type AccessLogConfig struct {
	Enabled          bool `json:"enabled" yaml:"enabled"`
	SlowThresholdMS int  `json:"slow_threshold_ms" yaml:"slow_threshold_ms"`
}
```

---

## 8.2 YAML 示例

```yaml
log:
  level: info
  format: json
  output: stdout
  add_source: true

  service_name: aihub
  env: dev
  version: v0.1.0

  redact:
    enabled: true
    keys:
      - password
      - passwd
      - token
      - access_token
      - refresh_token
      - secret
      - client_secret
      - authorization
      - cookie

  sampling:
    enabled: false
    first: 100
    thereafter: 100

  access_log:
    enabled: true
    slow_threshold_ms: 1000
```

---

## 8.3 默认配置

开发环境默认：

```yaml
log:
  level: debug
  format: console
  output: stdout
  add_source: true
```

生产环境默认：

```yaml
log:
  level: info
  format: json
  output: stdout
  add_source: false
```

容器化部署优先输出到 stdout，由外部日志系统采集。

---

# 9. 输出格式

## 9.1 JSON 格式

生产环境必须使用 JSON。

示例：

```json
{
  "timestamp": "2026-06-27T10:00:00.000Z",
  "level": "info",
  "message": "skill created",
  "service": "aihub",
  "module": "aihub.skill",
  "env": "prod",
  "version": "v0.1.0",
  "request_id": "req_123",
  "trace_id": "trace_abc",
  "user_id": "user_001",
  "org_id": "aisphere",
  "project_id": "project_001",
  "skill_id": "skill_001"
}
```

---

## 9.2 Console 格式

本地开发可以使用 console 格式。

示例：

```text
2026-06-27T10:00:00.000Z INFO  skill created service=aihub module=aihub.skill request_id=req_123 skill_id=skill_001
```

---

# 10. Context 集成

## 10.1 App Logger

用于启动阶段和无请求上下文场景：

```go
app.Logger().Info("kernel starting",
	logx.String("service", cfg.App.Name),
	logx.String("version", cfg.App.Version),
)
```

---

## 10.2 Request Logger

每个请求进入 Kernel 后，middleware 必须创建 request-scoped logger：

```go
reqLogger := app.Logger().With(
	logx.String("request_id", requestID),
	logx.String("trace_id", traceID),
	logx.String("user_id", principal.SubjectID),
	logx.String("org_id", principal.OrgID),
)
```

业务层通过：

```go
ctx.Log()
```

获取带上下文的 logger。

---

## 10.3 Module Logger

模块注册时可以创建 module logger：

```go
moduleLogger := app.Logger().Named("aihub.skill").With(
	logx.String("module", "aihub.skill"),
)
```

---

# 11. HTTP Middleware 设计

## 11.1 Access Log Middleware

每个请求完成后输出一条 access log。

伪代码：

```go
func AccessLogMiddleware(logger logx.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx contextx.Context) error {
			start := time.Now()

			err := next(ctx)

			status := httpx.StatusFromError(err)
			duration := time.Since(start)

			fields := []logx.Field{
				logx.String("method", ctx.Request().Method),
				logx.String("path", ctx.Request().URL.Path),
				logx.Int("status", status),
				logx.Duration("latency", duration),
			}

			if err != nil {
				fields = append(fields, logx.Error(err))
				ctx.Log().Warn("http request completed with error", fields...)
			} else {
				ctx.Log().Info("http request completed", fields...)
			}

			return err
		}
	}
}
```

---

## 11.2 Slow Request Log

超过阈值的请求必须输出 warn 日志：

```go
if duration > slowThreshold {
	ctx.Log().Warn("slow http request",
		logx.Duration("latency", duration),
		logx.String("route", route),
	)
}
```

---

# 12. 脱敏设计

## 12.1 必须脱敏的字段

默认必须脱敏：

```text
password
passwd
token
access_token
refresh_token
id_token
secret
client_secret
authorization
cookie
set_cookie
api_key
private_key
```

---

## 12.2 脱敏输出

原始字段：

```json
{
  "access_token": "abcdefg123456"
}
```

脱敏后：

```json
{
  "access_token": "***REDACTED***"
}
```

---

## 12.3 业务禁止打印完整请求体

禁止：

```go
ctx.Log().Info("request body", logx.Any("body", req))
```

除非 req 已经明确实现安全脱敏。

推荐：

```go
ctx.Log().Info("create skill request received",
	logx.String("project_id", req.ProjectID),
	logx.String("skill_name", req.Name),
)
```

---

# 13. 采样设计

第一版可以不实现复杂采样，但接口要预留。

场景：

```text
高频 debug 日志
大量重复错误
健康检查请求
静态资源请求
```

建议后续支持：

```go
type Sampler interface {
	ShouldLog(level Level, msg string, fields []Field) bool
}
```

---

# 14. 与 errorx 集成

`logx.Error(err)` 必须识别 `errorx.Error`。

如果 err 是 Kernel 标准错误，应提取：

```text
error_code
error_message
http_status
grpc_code
retryable
```

示例：

```go
err := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "no permission")

ctx.Log().Warn("delete skill denied",
	logx.Error(err),
	logx.String("skill_id", skillID),
)
```

输出：

```json
{
  "level": "warn",
  "message": "delete skill denied",
  "error": "no permission",
  "error_code": "AIHUB_SKILL_DELETE_DENIED",
  "http_status": 403,
  "retryable": false,
  "skill_id": "skill_001"
}
```

---

# 15. 与 tracingx 集成

日志必须可以和 trace 串联。

当请求存在 trace 时，日志自动带：

```text
trace_id
span_id
```

业务代码不需要手动传 trace 字段。

外部调用、DB、Redis、S3、AuthZ 等模块的日志也必须带相同 trace_id。

---

# 16. 与 metricsx 集成

日志不替代 metrics。

例如请求耗时既可以出现在日志里，也必须进入 metrics：

```text
http_request_duration_seconds
db_query_duration_seconds
authz_check_duration_seconds
```

日志用于查具体请求，metrics 用于看整体趋势。

---

# 17. 与 auditx 集成

日志不替代审计。

例如删除 skill：

```go
ctx.Log().Info("skill deleted",
	logx.String("skill_id", skill.ID),
)

ctx.Audit().Record(ctx.GoContext(), auditx.Event{
	Action:   "aihub.skill.delete",
	Resource: "aihub:skill:" + skill.ID,
	Actor:    ctx.Principal().SubjectID,
	Result:   "success",
})
```

必须同时存在。

---

# 18. AI 编码规则

## 18.1 AI 必须遵守

AI 写业务代码时：

```text
1. 只能通过 ctx.Log() 或 app.Logger() 打日志
2. 不允许直接 import log / slog / zap / logrus
3. 不允许 fmt.Println 调试
4. 不允许把日志当审计
5. 不允许打印 token / secret / password
6. 不允许拼接字符串作为主要日志内容
7. 必须使用结构化字段
8. 错误日志必须包含 logx.Error(err)
9. 权限拒绝建议用 Warn
10. 系统失败使用 Error
```

---

## 18.2 kernel-lint 规则

`kernel-lint` 必须检查业务目录中的 forbidden imports：

```text
log
log/slog
go.uber.org/zap
github.com/sirupsen/logrus
```

必须检查 forbidden calls：

```text
log.Println(
fmt.Println(
slog.Info(
slog.Error(
zap.New(
zap.L(
```

必须提示替代写法：

```text
Use ctx.Log().Info(...) or app.Logger().Info(...) instead.
```

---

# 19. 测试规范

## 19.1 Test Logger

提供测试 logger：

```go
logger := logx.NewTestLogger(t)
```

用于测试中断言日志字段。

---

## 19.2 Noop Logger

提供空 logger：

```go
logger := logx.Noop()
```

用于不关心日志的单元测试。

---

## 19.3 日志测试示例

```go
func TestCreateSkillLog(t *testing.T) {
	logger := logx.NewTestLogger(t)

	ctx := contextx.NewTestContext(
		contextx.WithLogger(logger),
		contextx.WithRequestID("req_test"),
	)

	_, err := svc.Create(ctx, CreateSkillRequest{Name: "demo"})
	require.NoError(t, err)

	logger.AssertLogged(t, "skill created",
		logx.String("request_id", "req_test"),
		logx.String("skill_name", "demo"),
	)
}
```

---

# 20. 第一版实现计划

## v0.1 必须完成

```text
Logger interface
Level 类型
Field 类型
String / Int / Bool / Duration / Error 等字段构造函数
Config 结构
Noop Logger
基于 slog 的默认实现
With(fields...)
Named(name)
JSON 输出
Console 输出
基础脱敏
App Logger 注入
Context Logger 注入
HTTP access log middleware
```

---

## v0.2 再做

```text
zap adapter
日志采样
TestLogger
gRPC access log
errorx 深度集成
trace_id / span_id 自动注入
慢请求日志
```

---

## v0.3 再做

```text
Loki writer
OTel log exporter
文件输出
日志动态级别调整
按模块设置日志级别
敏感字段策略增强
```

---

# 21. 最小可用代码示例

## 21.1 Logger 接口

```go
package logx

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)

	With(fields ...Field) Logger
	Named(name string) Logger
}
```

---

## 21.2 Field

```go
package logx

import "time"

type Field struct {
	Key   string
	Value any
}

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
}
```

---

## 21.3 Noop Logger

```go
package logx

type noopLogger struct{}

func Noop() Logger {
	return noopLogger{}
}

func (noopLogger) Debug(string, ...Field) {}
func (noopLogger) Info(string, ...Field)  {}
func (noopLogger) Warn(string, ...Field)  {}
func (noopLogger) Error(string, ...Field) {}

func (l noopLogger) With(...Field) Logger {
	return l
}

func (l noopLogger) Named(string) Logger {
	return l
}
```

---

# 22. 验收标准

`logx` v0.1 完成后必须满足：

```text
1. 业务代码可以通过 ctx.Log() 打日志
2. 启动阶段可以通过 app.Logger() 打日志
3. 日志支持 JSON 和 console
4. 日志支持 Debug / Info / Warn / Error
5. 日志支持结构化字段
6. 日志支持 With 和 Named
7. 日志自动携带 service / env / version
8. HTTP 请求日志自动携带 request_id / trace_id
9. 错误日志可以使用 logx.Error(err)
10. token / password / secret 等字段会被脱敏
11. kernel-lint 能禁止业务直接使用 log/slog/zap
12. examples/hello-module 使用 logx，而不是标准库 log
```

---

# 23. 总结

`logx` 是 Aisphere Kernel 最先应该打稳的模块之一。

它的价值不是“打印日志”，而是建立一条工程边界：

> 业务不决定日志怎么打。
> AI 不发明日志系统。
> Kernel 统一日志格式、上下文、脱敏、输出和采集。

最终目标：

```text
AI 写业务事件
logx 管日志工程
tracingx 管链路
metricsx 管指标
auditx 管审计
errorx 管错误
```

这几个模块必须协同，而不是各写各的。
