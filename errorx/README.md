# errorx

`errorx` 是 Aisphere Kernel 的统一错误语义包。它是 Kernel 中**唯一的**业务错误包，
完全替代旧的 `github.com/aisphereio/kernel/errors`。

> **新手上路**：只看本文件即可上手。需要深度细节时再翻其他文档（见末尾"文档地图"）。

---

## 1. 为什么需要 errorx

在 errorx 之前，业务错误散落为 `errors.New("xxx")` / `fmt.Errorf(...)` / `panic(err)`，
导致 HTTP 响应、gRPC 响应、日志、审计、指标、Worker 重试各自写一套错误处理逻辑，
错误码不稳定、不可观测、不可被 AI 和系统理解。

errorx 的核心契约：**业务错误必须有稳定的 `error_code`，并携带 HTTP/gRPC/retryable/metadata
等元信息，让所有消费方从同一个错误对象中提取语义。**

```text
errorx (只定义语义)
  ↓ 稳定契约 (Code / CodedError interface)
logx     → Fields(err)         提取日志字段
httpx    → HTTPStatusOf(err)   映射 HTTP 响应
grpcx    → GRPCCodeOf(err)     映射 gRPC status
auditx   → CodeOf(err)         记录失败原因
metricsx → MetricsLabels(err)  低基数指标
workerx  → RetryableOf(err)    判断是否重试
```

errorx 本身**不打印日志、不写 HTTP 响应、不做审计、不上报指标**。它只定义语义，其他模块只消费。

---

## 2. 30 秒上手

```go
package service

import "github.com/aisphereio/kernel/errorx"

const ErrSkillNotFound errorx.Code = "AIHUB_SKILL_NOT_FOUND"

func GetSkill(ctx context.Context, id string) (*Skill, error) {
    skill, err := repo.Find(ctx, id)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, errorx.NotFound(ErrSkillNotFound, "技能不存在",
            errorx.WithMetadata("skill_id", id),
            errorx.WithPublicMetadata("resource", "skill"),
        )
    }
    if err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
            errorx.WithMessage("查询技能失败"),
            errorx.WithRetryable(true),
        )
    }
    return skill, nil
}
```

HTTP 响应自动变为：

```http
HTTP/1.1 404 Not Found
Content-Type: application/json
```

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "技能不存在",
  "metadata": { "resource": "skill" }
}
```

---

## 3. 构造器速查

| 场景 | 构造器 | HTTP | gRPC | retryable |
|---|---|---:|---|---:|
| 参数校验失败 | `BadRequest` | 400 | InvalidArgument | false |
| 未登录 | `Unauthorized` | 401 | Unauthenticated | false |
| 无权限 | `Forbidden` | 403 | PermissionDenied | false |
| 资源不存在 | `NotFound` | 404 | NotFound | false |
| 资源已存在/冲突 | `Conflict` | 409 | AlreadyExists | false |
| 请求超时（本服务） | `RequestTimeout` | 408 | DeadlineExceeded | false |
| 限流 | `TooManyRequests` | 429 | ResourceExhausted | true |
| 客户端断开 | `ClientClosed` | 499 | Canceled | false |
| 内部错误 | `Internal` | 500 | Internal | false |
| 上游不可用 | `Unavailable` | 503 | Unavailable | true |
| 上游超时 | `Timeout` | 504 | DeadlineExceeded | true |

```go
errorx.BadRequest("REQUEST_VALIDATE_FAILED", "请求参数校验失败")
errorx.Unauthorized("AUTH_TOKEN_MISSING", "缺少授权令牌")
errorx.Forbidden("IAM_PERMISSION_DENIED", "权限被拒绝")
errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在")
errorx.RequestTimeout("REQUEST_TIMEOUT", "请求超时")
errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "请求过于频繁")
errorx.ClientClosed("CLIENT_CLOSED", "客户端断开连接")
errorx.Internal("AIHUB_INTERNAL_ERROR", "服务器内部错误")
errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型上游不可用")
errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型上游超时")
```

`Internal` 和 `InternalServer` 是别名；`Timeout` 和 `GatewayTimeout` 是别名；`ClientClosed` 和 `ClientClosedRequest` 是别名。

---

## 4. Option 列表

构造时按需附加：

```go
errorx.WithMessage(msg)                  // 覆盖默认 message
errorx.WithHTTPStatus(503)               // 覆盖默认 HTTP 状态
errorx.WithGRPCCode("Unavailable")       // 覆盖默认 gRPC code
errorx.WithRetryable(true)               // 覆盖默认 retryable
errorx.WithCause(err)                    // 包装底层错误（保留 errors.Is/As 链）
errorx.WithMetadata("skill_id", id)      // 内部 metadata（日志/审计用，不返回前端）
errorx.WithPublicMetadata("resource", "skill")  // 公开 metadata（可返回前端）
errorx.WithRequestID("req_123")          // 请求 ID
errorx.WithTraceID("trace_abc")          // 链路追踪 ID
errorx.WithCategory(errorx.CategoryAuth) // 错误分类
errorx.WithSeverity(errorx.SeverityError) // 日志严重级别
errorx.WithStack()                       // 捕获调用栈（仅建议用于 500 错误）
```

---

## 5. 包装底层错误

`Wrap` 保留底层 cause，支持 `errors.Is` / `errors.As`：

```go
skill, err := repo.Find(ctx, id)
if err != nil {
    return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
        errorx.WithMessage("查询技能失败"),
        errorx.WithRetryable(true),
        errorx.WithMetadata("skill_id", id),
    )
}

// errors.Is(wrapped, originalErr) == true
// errors.As(wrapped, &errorx.Error{}) == true
```

`Wrap(nil, ...)` 返回 `nil`，调用方无需 nil 检查：

```go
return errorx.Wrap(repo.MaybeError(), "AIHUB_SKILL_QUERY_FAILED")  // repo 返回 nil 时整个表达式为 nil
```

---

## 6. 第三方错误兼容

errorx 通过 `errors.As` 识别第三方错误，**不 import 第三方包**。识别的接口：

```go
Code() errorx.Code
ErrorCode() string
HTTPStatus() int
StatusCode() int
GRPCCode() string
Retryable() bool
Metadata() map[string]any
PublicMetadata() map[string]any
RequestID() string
TraceID() string
Category() errorx.Category
Severity() errorx.Severity
Stack() []errorx.Frame
```

例如 GoFr 风格的 `StatusCode()` 错误会被自动识别：

```go
type gofrNotFound struct{}
func (gofrNotFound) Error() string    { return "not found" }
func (gofrNotFound) StatusCode() int  { return 404 }

err := gofrNotFound{}
errorx.HTTPStatusOf(err)  // 404
errorx.CodeOf(err)        // NOT_FOUND
```

---

## 7. 检查辅助函数

消费方（logx/httpx/auditx/metricsx/workerx）应使用这些函数提取语义，**不要直接类型断言**：

```go
errorx.CodeOf(err)            // 提取错误码（nil → OK；未知 → INTERNAL_ERROR）
errorx.MessageOf(err)         // 提取消息
errorx.HTTPStatusOf(err)      // 提取 HTTP 状态码（nil → 200）
errorx.GRPCCodeOf(err)        // 提取 gRPC code 字符串
errorx.RetryableOf(err)       // 是否可重试
errorx.MetadataOf(err)        // 内部 metadata（防御性拷贝）
errorx.SafeMetadataOf(err)    // 脱敏后的 metadata（password/token/secret → [REDACTED]）
errorx.PublicMetadataOf(err)  // 公开 metadata
errorx.RequestIDOf(err)       // 请求 ID
errorx.TraceIDOf(err)         // 链路追踪 ID
errorx.CategoryOf(err)        // 错误分类
errorx.SeverityOf(err)        // 严重级别
errorx.StackOf(err)           // 调用栈
errorx.Fields(err)            // 日志字段（map[string]any）
errorx.MetricsLabels(err)     // 指标标签（低基数 map[string]string）
```

便捷谓词：

```go
errorx.IsBadRequest(err)
errorx.IsUnauthorized(err)
errorx.IsForbidden(err)
errorx.IsNotFound(err)
errorx.IsConflict(err)
errorx.IsRequestTimeout(err)
errorx.IsTooManyRequests(err)
errorx.IsClientClosedRequest(err)
errorx.IsInternal(err)
errorx.IsUnavailable(err)
errorx.IsTimeout(err)
errorx.IsCode(err, errorx.Code("AIHUB_SKILL_NOT_FOUND"))
```

---

## 8. Metadata 安全规则

errorx 区分三层 metadata 暴露：

| 层 | 函数 | 用途 | 是否脱敏 |
|---|---|---|---|
| 原始 | `Metadata()` / `MetadataOf()` | 日志/审计/调试 | ❌ |
| 脱敏 | `SafeMetadataOf()` | 安全日志 | ✅ password/token/secret → [REDACTED] |
| 公开 | `PublicMetadata()` / `PublicMetadataOf()` | HTTP/gRPC 响应 | 仅显式声明的字段 |

脱敏关键词（出现在 key 中即脱敏）：

```text
password / passwd / pwd / token / secret / authorization
cookie / credential / private_key / apikey / api_key
```

**铁律**：敏感数据只能放 `Metadata`，绝不能放 `PublicMetadata`。

```go
// ✅ 正确
errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("raw_db_error", err.Error()),          // 内部用
    errorx.WithPublicMetadata("resource", "skill"),            // 前端可见
)

// ❌ 错误：把 token 放进公开 metadata
errorx.Unauthorized("AUTH_TOKEN_INVALID", "token 无效",
    errorx.WithPublicMetadata("token", userToken),             // 泄漏！
)
```

---

## 9. 错误码命名规范

格式：`{DOMAIN}_{RESOURCE}_{REASON}`，全大写蛇形。

```text
✅ AIHUB_SKILL_NOT_FOUND
✅ IAM_TOKEN_INVALID
✅ MODEL_UPSTREAM_TIMEOUT
✅ DB_TRANSACTION_FAILED

❌ skillNotFound        （驼峰）
❌ skill-not-found      （连字符）
❌ not_found            （过于宽泛）
❌ SKILL_123_NOT_FOUND  （包含动态 ID）
❌ ERROR / FAILED       （无意义）
```

**动态信息必须放 metadata，不能放 code**：

```go
// ❌ 错误
errorx.NotFound(errorx.Code("SKILL_"+skillID+"_NOT_FOUND"), "技能不存在")

// ✅ 正确
errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", skillID),
)
```

校验：

```go
const ErrSkillNotFound errorx.Code = "AIHUB_SKILL_NOT_FOUND"

func TestErrorCodes(t *testing.T) {
    if !errorx.IsValidCode(ErrSkillNotFound) {
        t.Fatal("invalid error code")
    }
}
```

---

## 10. Retryable 与 Worker 重试

默认 retryable 规则：

| Code | retryable |
|---|---:|
| TOO_MANY_REQUESTS | true |
| SERVICE_UNAVAILABLE | true |
| TIMEOUT | true |
| 其他 | false |

业务可覆盖：

```go
// 默认 false，但业务确认可重试
return errorx.Internal("AIHUB_DB_TRANSACTION_FAILED", "数据库事务失败",
    errorx.WithRetryable(true),
)

// Worker 侧判断
if errorx.RetryableOf(err) {
    return workerx.Retry(err)
}
return workerx.Fail(err)
```

---

## 11. 调试与 `%+v`

`Error()` 返回安全 message（可返回前端）。`fmt.Printf("%+v", err)` 输出完整调试信息：

```go
err := errorx.Internal("AIHUB_SKILL_QUERY_FAILED", "查询技能失败",
    errorx.WithCause(errors.New("pq: connection refused")),
    errorx.WithRequestID("req_123"),
)

fmt.Println(err)
// 查询技能失败

fmt.Printf("%+v\n", err)
// error_code=AIHUB_SKILL_QUERY_FAILED message="查询技能失败" http_status=500 grpc_code=Internal retryable=false category=internal severity=error request_id=req_123 trace_id= metadata={} public_metadata={} cause=pq: connection refused
```

---

## 12. 测试与验收

```bash
# 单元测试
go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
go test ./errorx -bench=.

# 模糊测试
go test ./errorx -run=^$ -fuzz=FuzzNewError -fuzztime=30s

# 可运行示例
go run ./examples/errorx-basic
go run ./examples/errorx-http

# 完整验收
make test-errorx
make verify-errorx
```

---

## 13. 禁止事项（AI 必读）

业务代码（handler/service/repository）禁止：

```go
return errors.New("skill not found")              // ❌ 裸 error
return fmt.Errorf("create failed: %w", err)       // ❌ fmt.Errorf
return nil, err                                   // ❌ 透传底层错误
panic(err)                                        // ❌ panic
```

替代：

```go
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", id),
)
return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
    errorx.WithMessage("创建技能失败"),
)
```

测试代码中可以使用 `errors.New` / `fmt.Errorf` 构造 fixture。

---

## 14. 文档地图

errorx 的文档分为四类，按需查阅：

```text
快速上手
├── 本文件 (errorx/README.md)              ← 你正在看的，单一入口
├── errorx/doc.go                          ← go doc 输出源
└── errorx/example_test.go                 ← Go 标准示例（go test -v 可看输出）

深度规范（架构师/PR review 时看）
├── docs/design/errorx.md                  ← 1254 行设计规范，覆盖 26 章
└── docs/contracts/errorx.md               ← 不可破坏契约 + 验收命令

AI 编码指南（AI 写业务代码时看）
├── docs/ai/errorx.md                      ← 合并版 AI 指南（含速查/食谱/禁用模式）
└── AGENTS.md                              ← 项目级 AI 规则

验收与运维（CI/CD 时看）
├── docs/process/errorx-acceptance-checklist.md  ← 静态/单元/集成验收清单
├── docs/process/errorx-test-report.md           ← 测试报告
└── docs/process/module-acceptance.md            ← 通用模块验收流程

可运行示例
├── examples/errorx-basic/                 ← 最小示例：Wrap + Inspect
└── examples/errorx-http/                  ← HTTP handler 示例：返回 JSON 响应
```

**优先级**：日常开发只看本 README + `docs/ai/errorx.md` 即可。
其他文档按场景查阅，无需通读。

---

## 15. 设计哲学一句话

> 错误必须有 code。
> 错误必须可映射。
> 错误必须可观测。
> 错误必须可被 AI 和系统理解。
> errorx 只定义语义，不负责日志、不负责响应、不负责审计。
