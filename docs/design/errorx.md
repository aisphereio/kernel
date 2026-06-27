# Aisphere Kernel 错误系统设计规范

文件路径：`docs/design/errorx.md`
模块名：`errorx`
版本：v0.1-draft
适用仓库：`github.com/aisphereio/kernel`

---

# 1. 模块定位

`errorx` 是 Aisphere Kernel 的统一错误模块。

它负责定义 Kernel 内部、业务模块、HTTP、gRPC、日志、审计、指标之间可共享的标准错误模型。

在 Aisphere Kernel 中，错误不是简单字符串。

错误必须携带：

```text
error_code
message
http_status
grpc_code
retryable
cause
metadata
request_id
trace_id
```

一句话：

> `errorx` 负责错误语义，其他模块只消费错误语义。

---

# 2. 为什么 errorx 要先做

`errorx` 是 Kernel 的底层基础模块。

它会被这些模块依赖：

```text
logx      → 提取 error_code / retryable / http_status
httpx     → 映射 HTTP response
grpcx     → 映射 gRPC status
auditx    → 记录失败原因和 error_code
metricsx  → 按 error_code 统计失败次数
workerx   → 判断是否可重试
```

所以开发顺序应该是：

```text
errorx → configx → logx → contextx → core → httpx
```

重要依赖规则：

```text
errorx 不能依赖 logx
errorx 不能依赖 httpx
errorx 不能依赖 grpcx
errorx 不能依赖 contextx
errorx 只能依赖 Go 标准库
```

---

# 3. 设计目标

`errorx` 第一版要解决这些问题：

```text
1. 禁止业务返回裸 fmt.Errorf / errors.New
2. 所有业务错误必须有稳定 error_code
3. HTTP / gRPC 响应可以从 errorx 自动映射
4. 日志可以从 errorx 自动提取错误字段
5. worker 可以根据 retryable 判断是否重试
6. audit 可以记录标准错误原因
7. AI 写业务代码时有明确错误规范
```

---

# 4. 核心原则

## 4.1 错误码必须稳定

错误码是系统契约，不是临时字符串。

错误码用于：

```text
前端展示
API 响应
日志检索
审计记录
指标聚合
告警规则
AI 排障
```

错误码一旦发布，不应随意修改。

---

## 4.2 错误码使用大写 snake case

推荐格式：

```text
DOMAIN_RESOURCE_REASON
```

示例：

```text
AIHUB_SKILL_NOT_FOUND
AIHUB_SKILL_CREATE_FAILED
AIHUB_SKILL_PUBLISH_DENIED
IAM_TOKEN_INVALID
IAM_AUTHZ_CHECK_FAILED
MODEL_UPSTREAM_TIMEOUT
DB_TRANSACTION_FAILED
```

禁止：

```text
skillNotFound
skill-not-found
not_found
error_001
500
failed
```

---

## 4.3 message 面向人，code 面向系统

错误码是稳定机器语义：

```text
AIHUB_SKILL_NOT_FOUND
```

message 是可读说明：

```text
技能不存在
```

业务逻辑、前端分支、指标聚合、告警规则必须依赖 `code`，不能依赖 `message`。

---

## 4.4 错误必须可包装

底层错误不能丢。

例如数据库失败：

```go
return nil, errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
	errorx.WithMessage("创建技能失败"),
	errorx.WithHTTPStatus(500),
	errorx.WithRetryable(true),
)
```

这样既保留底层 cause，又有业务语义。

---

## 4.5 errorx 不负责打印日志

`errorx` 只创建和描述错误。

禁止在 `errorx` 中：

```go
log.Println(...)
slog.Error(...)
zap.L().Error(...)
```

日志由 `logx` 负责。

---

# 5. 包结构设计

推荐目录：

```text
errorx/
├── error.go          # Error 主结构
├── code.go           # Code 类型和内置错误码
├── option.go         # Option 模式
├── constructors.go   # New / Wrap / BadRequest 等构造函数
├── inspect.go        # CodeOf / StatusOf / RetryableOf
├── http.go           # HTTP status 映射
├── grpc.go           # gRPC code 映射，第一版可选
├── metadata.go       # metadata 支持
├── category.go       # 错误分类
├── stack.go          # 堆栈，第一版可选
└── errors_test.go
```

第一版建议只实现：

```text
error.go
code.go
option.go
constructors.go
inspect.go
http.go
```

---

# 6. 核心类型设计

## 6.1 Code 类型

```go
package errorx

type Code string

func (c Code) String() string {
	return string(c)
}
```

内置通用错误码：

```go
const (
	CodeOK Code = "OK"

	CodeBadRequest      Code = "BAD_REQUEST"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeInternal        Code = "INTERNAL_ERROR"
	CodeUnavailable     Code = "SERVICE_UNAVAILABLE"
	CodeTimeout         Code = "TIMEOUT"
)
```

业务模块应定义自己的错误码：

```go
const (
	ErrSkillNotFound     errorx.Code = "AIHUB_SKILL_NOT_FOUND"
	ErrSkillCreateFailed errorx.Code = "AIHUB_SKILL_CREATE_FAILED"
	ErrSkillPublishDenied errorx.Code = "AIHUB_SKILL_PUBLISH_DENIED"
)
```

---

## 6.2 Error 主结构

```go
package errorx

type Error struct {
	code       Code
	message    string
	httpStatus int
	grpcCode   string
	retryable  bool
	cause      error
	metadata   map[string]any
}
```

对外通过方法访问：

```go
func (e *Error) Error() string
func (e *Error) Unwrap() error
func (e *Error) Code() Code
func (e *Error) Message() string
func (e *Error) HTTPStatus() int
func (e *Error) GRPCCode() string
func (e *Error) Retryable() bool
func (e *Error) Metadata() map[string]any
```

不建议直接导出字段，避免后续兼容性问题。

---

## 6.3 CodedError 接口

`logx`、`httpx`、`auditx` 不应该强依赖 `*errorx.Error`，而是依赖一个轻量接口：

```go
type CodedError interface {
	error
	Code() Code
	Message() string
	HTTPStatus() int
	Retryable() bool
}
```

这样未来第三方错误类型也能适配 Kernel。

---

# 7. Option 设计

使用 Option 模式创建错误。

```go
type Option func(*Error)

func WithMessage(message string) Option
func WithHTTPStatus(status int) Option
func WithGRPCCode(code string) Option
func WithRetryable(retryable bool) Option
func WithMetadata(key string, value any) Option
func WithCause(cause error) Option
```

示例：

```go
return errorx.New("AIHUB_SKILL_NOT_FOUND",
	errorx.WithMessage("技能不存在"),
	errorx.WithHTTPStatus(404),
	errorx.WithRetryable(false),
)
```

---

# 8. 构造函数设计

## 8.1 New

```go
func New(code Code, opts ...Option) error
```

示例：

```go
return errorx.New("AIHUB_SKILL_NAME_REQUIRED",
	errorx.WithMessage("技能名称不能为空"),
	errorx.WithHTTPStatus(400),
)
```

---

## 8.2 Wrap

```go
func Wrap(cause error, code Code, opts ...Option) error
```

示例：

```go
skill, err := repo.Get(ctx, id)
if err != nil {
	return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
		errorx.WithMessage("查询技能失败"),
		errorx.WithHTTPStatus(500),
		errorx.WithRetryable(true),
	)
}
```

如果 `cause == nil`，建议返回 `nil`：

```go
func Wrap(cause error, code Code, opts ...Option) error {
	if cause == nil {
		return nil
	}
	// ...
}
```

这样调用方更安全。

---

## 8.3 快捷构造函数

提供常用快捷方法：

```go
func BadRequest(code Code, message string, opts ...Option) error
func Unauthorized(code Code, message string, opts ...Option) error
func Forbidden(code Code, message string, opts ...Option) error
func NotFound(code Code, message string, opts ...Option) error
func Conflict(code Code, message string, opts ...Option) error
func TooManyRequests(code Code, message string, opts ...Option) error
func Internal(code Code, message string, opts ...Option) error
func Unavailable(code Code, message string, opts ...Option) error
func Timeout(code Code, message string, opts ...Option) error
```

示例：

```go
return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
```

权限错误：

```go
return nil, errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除技能的权限")
```

内部错误：

```go
return nil, errorx.Internal("AIHUB_SKILL_CREATE_FAILED", "创建技能失败",
	errorx.WithCause(err),
	errorx.WithRetryable(true),
)
```

---

# 9. Inspect 工具函数

其他模块不应该直接类型断言太多。

`errorx` 提供统一检查函数。

```go
func IsKernelError(err error) bool
func As(err error) (*Error, bool)

func CodeOf(err error) Code
func MessageOf(err error) string
func HTTPStatusOf(err error) int
func GRPCCodeOf(err error) string
func RetryableOf(err error) bool
func MetadataOf(err error) map[string]any
```

默认规则：

```text
nil error        → CodeOK / 200
unknown error    → INTERNAL_ERROR / 500
errorx.Error     → 使用自身字段
```

示例：

```go
code := errorx.CodeOf(err)
status := errorx.HTTPStatusOf(err)
retryable := errorx.RetryableOf(err)
```

---

# 10. HTTP 映射规范

## 10.1 默认 HTTP status

```text
BAD_REQUEST           → 400
UNAUTHORIZED          → 401
FORBIDDEN             → 403
NOT_FOUND             → 404
CONFLICT              → 409
TOO_MANY_REQUESTS     → 429
TIMEOUT               → 504
SERVICE_UNAVAILABLE   → 503
INTERNAL_ERROR        → 500
```

业务自定义错误可以通过 `WithHTTPStatus` 指定。

---

## 10.2 HTTP 响应格式

`httpx` 应基于 `errorx` 返回统一格式：

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "技能不存在",
  "request_id": "req_123",
  "trace_id": "trace_abc"
}
```

生产环境默认不返回底层 cause。

开发环境可以返回 debug 信息：

```json
{
  "code": "AIHUB_SKILL_QUERY_FAILED",
  "message": "查询技能失败",
  "debug": "pq: connection refused",
  "request_id": "req_123"
}
```

---

# 11. gRPC 映射规范

第一版可以先预留接口，不强制实现。

推荐映射：

```text
BAD_REQUEST           → InvalidArgument
UNAUTHORIZED          → Unauthenticated
FORBIDDEN             → PermissionDenied
NOT_FOUND             → NotFound
CONFLICT              → AlreadyExists / Aborted
TOO_MANY_REQUESTS     → ResourceExhausted
TIMEOUT               → DeadlineExceeded
SERVICE_UNAVAILABLE   → Unavailable
INTERNAL_ERROR        → Internal
```

接口：

```go
func GRPCCodeOf(err error) string
```

第一版如果不引入 gRPC 依赖，可以先用 string 保存：

```go
grpcCode string
```

后续 `grpcx` adapter 再转换成真实 `codes.Code`。

---

# 12. Retryable 规范

`retryable` 用于 worker、外部调用、任务重试判断。

默认建议：

```text
BadRequest      false
Unauthorized    false
Forbidden       false
NotFound        false
Conflict        false
TooManyRequests true
Timeout         true
Unavailable     true
Internal        false，除非明确指定
```

示例：

```go
return errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型服务不可用",
	errorx.WithRetryable(true),
)
```

任务系统示例：

```go
if errorx.RetryableOf(err) {
	return workerx.Retry(err)
}
return workerx.Fail(err)
```

---

# 13. Metadata 规范

`metadata` 用于给错误附加机器可读上下文。

示例：

```go
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
	errorx.WithMetadata("skill_id", skillID),
	errorx.WithMetadata("project_id", projectID),
)
```

禁止在 metadata 中放：

```text
password
token
secret
authorization
cookie
完整请求体
大对象
隐私数据
```

metadata 可以被 `logx` 使用，但 HTTP 响应默认不返回 metadata。
是否返回由 `httpx` 配置控制。

---

# 14. 错误分类

第一版可以预留 Category。

```go
type Category string

const (
	CategoryValidation Category = "validation"
	CategoryAuth       Category = "auth"
	CategoryPermission Category = "permission"
	CategoryNotFound   Category = "not_found"
	CategoryConflict   Category = "conflict"
	CategoryDependency Category = "dependency"
	CategoryInternal   Category = "internal"
)
```

后续用于：

```text
metrics 聚合
告警分类
日志筛选
AI 排障分类
```

---

# 15. 错误码命名规范

## 15.1 通用格式

```text
{DOMAIN}_{RESOURCE}_{REASON}
```

示例：

```text
AIHUB_SKILL_NOT_FOUND
AIHUB_SKILL_CREATE_FAILED
AIHUB_SKILL_NAME_REQUIRED
AIHUB_SKILL_PUBLISH_DENIED
IAM_TOKEN_INVALID
IAM_USER_NOT_FOUND
MODEL_PROVIDER_TIMEOUT
RUNTIME_TOOL_CALL_DENIED
```

---

## 15.2 常用 REASON

```text
REQUIRED
INVALID
NOT_FOUND
ALREADY_EXISTS
DENIED
UNAUTHORIZED
FORBIDDEN
CONFLICT
FAILED
TIMEOUT
UNAVAILABLE
RATE_LIMITED
EXPIRED
DISABLED
```

---

## 15.3 不推荐的错误码

禁止过于宽泛：

```text
ERROR
FAILED
INTERNAL
BAD
UNKNOWN
```

禁止包含动态 ID：

```text
SKILL_123_NOT_FOUND
USER_TOM_DENIED
```

错误码必须稳定，动态信息放 metadata。

---

# 16. 与 logx 集成

`logx.Error(err)` 应提取：

```text
error
error_code
http_status
retryable
```

示例：

```go
err := errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除权限")

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
  "error": "没有删除权限",
  "error_code": "AIHUB_SKILL_DELETE_DENIED",
  "http_status": 403,
  "retryable": false,
  "skill_id": "skill_001"
}
```

---

# 17. 与 httpx 集成

`httpx` 不应该自己定义错误码。
它只负责把 `errorx` 映射成响应。

伪代码：

```go
func WriteError(w http.ResponseWriter, err error, ctx contextx.Context) {
	status := errorx.HTTPStatusOf(err)

	resp := ErrorResponse{
		Code:      errorx.CodeOf(err).String(),
		Message:   errorx.MessageOf(err),
		RequestID: ctx.RequestID(),
		TraceID:   ctx.TraceID(),
	}

	writeJSON(w, status, resp)
}
```

---

# 18. 与 auditx 集成

审计记录失败时，应保存：

```text
error_code
error_message
result = fail / deny
```

示例：

```go
ctx.Audit().Record(ctx.GoContext(), auditx.Event{
	Action:    "aihub.skill.delete",
	Resource:  "aihub:skill:" + skillID,
	Actor:     ctx.Principal().SubjectID,
	Result:    "deny",
	Reason:    errorx.CodeOf(err).String(),
	ErrorCode: errorx.CodeOf(err).String(),
})
```

---

# 19. 与 metricsx 集成

指标应按错误码聚合：

```text
http_requests_total{code="AIHUB_SKILL_NOT_FOUND",status="404"}
worker_jobs_failed_total{code="MODEL_UPSTREAM_TIMEOUT",retryable="true"}
authz_checks_total{code="FORBIDDEN"}
```

因此错误码不能随意生成，必须低基数、稳定。

禁止把动态 ID 放进错误码，否则会造成 metrics 爆炸。

---

# 20. AI 编码规则

## 20.1 AI 必须遵守

AI 写业务代码时必须：

```text
1. 业务错误使用 errorx
2. 参数错误使用 errorx.BadRequest
3. 未登录使用 errorx.Unauthorized
4. 无权限使用 errorx.Forbidden
5. 资源不存在使用 errorx.NotFound
6. 外部依赖失败使用 errorx.Unavailable 或 errorx.Timeout
7. 数据库等内部失败使用 errorx.Internal 或 errorx.Wrap
8. 返回错误时保留 cause
9. 错误码使用大写 snake case
10. 动态信息放 metadata，不放 code
```

---

## 20.2 AI 禁止写法

禁止：

```go
return errors.New("skill not found")
return fmt.Errorf("create skill failed: %w", err)
return nil, err
panic(err)
```

替代：

```go
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
	errorx.WithMetadata("skill_id", skillID),
)
```

包装：

```go
return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
	errorx.WithMessage("创建技能失败"),
	errorx.WithHTTPStatus(500),
)
```

---

# 21. kernel-lint 规则

`kernel-lint` 应在业务目录检查：

## 21.1 禁止 imports

```text
fmt
errors
```

注意：不是完全禁止 import `fmt`，因为格式化字符串有时合理。
但在 service / handler / repository 层，必须禁止：

```text
fmt.Errorf
errors.New
```

## 21.2 禁止调用

```text
fmt.Errorf(
errors.New(
panic(
```

## 21.3 允许场景

测试代码中可以允许：

```text
errors.New
fmt.Errorf
```

但业务代码不允许。

路径豁免：

```text
*_test.go
internal/test/*
tools/*
```

---

# 22. 测试规范

## 22.1 构造错误测试

```go
func TestNotFound(t *testing.T) {
	err := errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")

	require.Equal(t, errorx.Code("AIHUB_SKILL_NOT_FOUND"), errorx.CodeOf(err))
	require.Equal(t, 404, errorx.HTTPStatusOf(err))
	require.False(t, errorx.RetryableOf(err))
}
```

---

## 22.2 Wrap 测试

```go
func TestWrap(t *testing.T) {
	cause := errors.New("db timeout")

	err := errorx.Wrap(cause, "AIHUB_SKILL_QUERY_FAILED",
		errorx.WithMessage("查询技能失败"),
		errorx.WithRetryable(true),
	)

	require.True(t, errors.Is(err, cause))
	require.Equal(t, errorx.Code("AIHUB_SKILL_QUERY_FAILED"), errorx.CodeOf(err))
	require.True(t, errorx.RetryableOf(err))
}
```

---

## 22.3 Unknown error 测试

```go
func TestUnknownError(t *testing.T) {
	err := errors.New("unknown")

	require.Equal(t, errorx.CodeInternal, errorx.CodeOf(err))
	require.Equal(t, 500, errorx.HTTPStatusOf(err))
	require.False(t, errorx.RetryableOf(err))
}
```

---

# 23. 第一版实现计划

## v0.1 必须完成

```text
Code 类型
Error 主结构
CodedError 接口
New
Wrap
BadRequest
Unauthorized
Forbidden
NotFound
Conflict
Internal
Unavailable
Timeout
WithMessage
WithHTTPStatus
WithRetryable
WithMetadata
WithCause
CodeOf
MessageOf
HTTPStatusOf
RetryableOf
IsKernelError
As
单元测试
```

---

## v0.2 再做

```text
GRPCCodeOf
Category
Stack trace
错误注册表
错误码唯一性检查
错误码文档生成
```

---

## v0.3 再做

```text
多语言 message
前端错误文案映射
错误码中心
错误码 OpenAPI 导出
错误码 metrics cardinality 检查
```

---

# 24. 最小代码草案

## 24.1 error.go

```go
package errorx

type Error struct {
	code       Code
	message    string
	httpStatus int
	grpcCode   string
	retryable  bool
	cause      error
	metadata   map[string]any
}

func (e *Error) Error() string {
	if e.message != "" {
		return e.message
	}
	return e.code.String()
}

func (e *Error) Unwrap() error {
	return e.cause
}

func (e *Error) Code() Code {
	return e.code
}

func (e *Error) Message() string {
	return e.message
}

func (e *Error) HTTPStatus() int {
	return e.httpStatus
}

func (e *Error) GRPCCode() string {
	return e.grpcCode
}

func (e *Error) Retryable() bool {
	return e.retryable
}

func (e *Error) Metadata() map[string]any {
	return e.metadata
}
```

---

## 24.2 constructors.go

```go
package errorx

func New(code Code, opts ...Option) error {
	e := &Error{
		code:       code,
		message:    code.String(),
		httpStatus: defaultHTTPStatus(code),
		retryable:  defaultRetryable(code),
		metadata:   map[string]any{},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

func Wrap(cause error, code Code, opts ...Option) error {
	if cause == nil {
		return nil
	}

	opts = append(opts, WithCause(cause))
	return New(code, opts...)
}

func BadRequest(code Code, message string, opts ...Option) error {
	opts = append(opts, WithMessage(message), WithHTTPStatus(400), WithRetryable(false))
	return New(code, opts...)
}

func Unauthorized(code Code, message string, opts ...Option) error {
	opts = append(opts, WithMessage(message), WithHTTPStatus(401), WithRetryable(false))
	return New(code, opts...)
}

func Forbidden(code Code, message string, opts ...Option) error {
	opts = append(opts, WithMessage(message), WithHTTPStatus(403), WithRetryable(false))
	return New(code, opts...)
}

func NotFound(code Code, message string, opts ...Option) error {
	opts = append(opts, WithMessage(message), WithHTTPStatus(404), WithRetryable(false))
	return New(code, opts...)
}

func Internal(code Code, message string, opts ...Option) error {
	opts = append(opts, WithMessage(message), WithHTTPStatus(500), WithRetryable(false))
	return New(code, opts...)
}
```

---

## 24.3 inspect.go

```go
package errorx

import "errors"

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func IsKernelError(err error) bool {
	_, ok := As(err)
	return ok
}

func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	if e, ok := As(err); ok {
		return e.Code()
	}
	return CodeInternal
}

func MessageOf(err error) string {
	if err == nil {
		return "success"
	}
	if e, ok := As(err); ok {
		return e.Message()
	}
	return "internal server error"
}

func HTTPStatusOf(err error) int {
	if err == nil {
		return 200
	}
	if e, ok := As(err); ok {
		return e.HTTPStatus()
	}
	return 500
}

func RetryableOf(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := As(err); ok {
		return e.Retryable()
	}
	return false
}
```

---

# 25. 验收标准

`errorx` v0.1 完成后必须满足：

```text
1. 可以创建标准错误
2. 可以包装底层错误
3. errors.Is / errors.As 可用
4. 可以提取 error_code
5. 可以提取 http_status
6. 可以提取 retryable
7. 未知错误默认 INTERNAL_ERROR / 500
8. nil error 默认 OK / 200
9. 业务代码可以用快捷函数返回错误
10. logx/httpx/auditx/metricsx 可以无侵入消费 errorx
11. 单元测试覆盖 New / Wrap / Inspect / HTTP 映射
12. kernel-lint 后续可以禁止业务使用 fmt.Errorf / errors.New
```

---

# 26. 总结

`errorx` 是 Aisphere Kernel 的第一块地基。

它不是为了“包装 error”这么简单，而是为了建立整个框架的错误语义中心。

最终目标：

```text
业务返回 errorx
httpx 映射响应
logx 提取错误字段
auditx 记录失败原因
metricsx 聚合错误指标
workerx 判断是否重试
AI 按错误规范写业务
```

核心原则：

> 错误必须有 code。
> 错误必须可映射。
> 错误必须可观测。
> 错误必须可被 AI 和系统理解。
> errorx 只定义语义，不负责日志、不负责响应、不负责审计。
