# errorx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的错误处理规范。**只看本文件即可写对所有错误场景。**

---

## 0. 一句话规则

> 业务代码（handler/service/repository）返回错误时，**必须**使用 `github.com/aisphereio/kernel/errorx`。
> **禁止**使用 `errors.New` / `fmt.Errorf` / `panic` 表达业务失败。

---

## 1. 速查：什么场景用什么构造器

| 业务场景 | 构造器 | HTTP | 何时使用 |
|---|---|---:|---|
| 参数校验失败 | `errorx.BadRequest` | 400 | 必填字段缺失、格式非法、值越界 |
| 未登录 / token 无效 | `errorx.Unauthorized` | 401 | 缺少 Authorization、token 过期 |
| 已登录但无权限 | `errorx.Forbidden` | 403 | 鉴权通过但 RBAC 拒绝 |
| 资源不存在 | `errorx.NotFound` | 404 | 数据库查不到、目录不存在 |
| 资源已存在 / 状态冲突 | `errorx.Conflict` | 409 | 唯一约束冲突、状态机不允许 |
| 请求超时（本服务处理） | `errorx.RequestTimeout` | 408 | 请求处理超过配置的超时 |
| 限流 | `errorx.TooManyRequests` | 429 | 触发 rate limiter |
| 客户端断开 | `errorx.ClientClosed` | 499 | `r.Context().Err() == context.Canceled` |
| 服务器内部错误 | `errorx.Internal` | 500 | 不该发生的异常、未知错误 |
| 上游依赖不可用 | `errorx.Unavailable` | 503 | DB/Redis/模型服务连不上 |
| 上游依赖超时 | `errorx.Timeout` | 504 | 调用上游 API 超时 |

---

## 2. 标准食谱（10 个场景，复制即用）

### 2.1 参数校验失败

```go
if req.Name == "" {
    return errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "技能名称不能为空")
}
if len(req.Name) > 128 {
    return errorx.BadRequest("AIHUB_SKILL_NAME_TOO_LONG", "技能名称不能超过 128 字符",
        errorx.WithPublicMetadata("field", "name"),
        errorx.WithPublicMetadata("max", "128"),
    )
}
```

### 2.2 未登录 / token 无效

```go
token := extractBearer(r)
if token == "" {
    return errorx.Unauthorized("AUTH_TOKEN_MISSING", "缺少授权令牌")
}
claims, err := authn.Verify(ctx, token)
if err != nil {
    return errorx.Unauthorized("AUTH_TOKEN_INVALID", "授权令牌无效",
        errorx.WithCause(err),
    )
}
```

### 2.3 无权限

```go
if err := rt.Access.Require(ctx, access.Check{
    Resource: "aihub:skill:" + name,
    Action:   "skill.delete",
}); err != nil {
    return errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除技能的权限",
        errorx.WithMetadata("subject_id", principal.SubjectID),
        errorx.WithMetadata("resource", "aihub:skill:"+name),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
```

### 2.4 资源不存在（含底层错误转换）

```go
skill, err := repo.Find(ctx, id)
if errors.Is(err, sql.ErrNoRows) {
    return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
        errorx.WithCause(err),
        errorx.WithMetadata("skill_id", id),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
```

### 2.5 资源已存在 / 冲突

```go
if existing, _ := repo.FindByName(ctx, name); existing != nil {
    return errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能名已存在",
        errorx.WithMetadata("skill_id", existing.ID),
        errorx.WithPublicMetadata("name", name),
    )
}
```

### 2.6 数据库失败（保留 cause + 可重试）

```go
if err := repo.Create(ctx, skill); err != nil {
    return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
        errorx.WithMessage("创建技能失败"),
        errorx.WithRetryable(true),  // 网络抖动可重试
        errorx.WithMetadata("skill_id", skill.ID),
    )
}
```

### 2.7 模型上游超时

```go
resp, err := modelClient.Invoke(ctx, req)
if errors.Is(err, context.DeadlineExceeded) {
    return errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
        errorx.WithCause(err),
        errorx.WithRetryable(true),
        errorx.WithMetadata("provider", "openai"),
        errorx.WithMetadata("model", "gpt-4"),
    )
}
```

### 2.8 模型上游不可用

```go
resp, err := modelClient.Invoke(ctx, req)
if isConnectionRefused(err) {
    return errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "模型服务不可用",
        errorx.WithCause(err),
        errorx.WithRetryable(true),
        errorx.WithMetadata("endpoint", modelClient.Endpoint()),
    )
}
```

### 2.9 限流

```go
if !limiter.Allow(ctx, userID) {
    return errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "请求过于频繁",
        errorx.WithPublicMetadata("retry_after_seconds", "60"),
    )
}
```

### 2.10 客户端断开（流式接口必加）

```go
select {
case <-ctx.Done():
    return errorx.ClientClosed("HTTP_CLIENT_CLOSED_REQUEST", "客户端断开连接",
        errorx.WithCause(ctx.Err()),
    )
case chunk := <-stream:
    // handle chunk
}
```

---

## 3. 错误码命名规则

格式：`{DOMAIN}_{RESOURCE}_{REASON}`，全大写蛇形。

**常用 REASON 词汇**：

```text
REQUIRED        必填字段缺失
INVALID         格式或值非法
NOT_FOUND       资源不存在
ALREADY_EXISTS  资源已存在
DENIED          鉴权拒绝
UNAUTHORIZED    未登录
FORBIDDEN       无权限
CONFLICT        状态冲突
FAILED          操作失败（通用）
TIMEOUT         超时
UNAVAILABLE     不可用
RATE_LIMITED    限流
EXPIRED         已过期
DISABLED        已禁用
```

**示例**：

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

## 4. 禁止模式

### 4.1 业务代码禁止（handler/service/repository）

```go
// ❌ 裸 error
return errors.New("skill not found")

// ❌ fmt.Errorf 作为业务错误
return fmt.Errorf("create skill failed: %w", err)

// ❌ 透传底层错误（前端看到 "pq: connection refused" 是泄漏）
return nil, err

// ❌ panic
panic(err)

// ❌ 动态信息放 code（导致 Prometheus 标签爆炸）
return errorx.NotFound(errorx.Code("SKILL_"+skillID+"_NOT_FOUND"), "技能不存在")
```

### 4.2 替代写法

```go
// ✅ 用 errorx 构造器
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", skillID),
)

// ✅ 用 Wrap 保留 cause
return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
    errorx.WithMessage("创建技能失败"),
)

// ✅ 动态信息放 metadata
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", skillID),
)
```

### 4.3 允许的例外

测试代码（`*_test.go`）可以使用 `errors.New` / `fmt.Errorf` 构造 fixture：

```go
func TestWrap(t *testing.T) {
    cause := errors.New("db timeout")  // ✅ 测试代码允许
    err := errorx.Wrap(cause, "AIHUB_SKILL_QUERY_FAILED")
    // ...
}
```

---

## 5. Metadata 安全规则

```go
// ✅ 敏感数据放 Metadata（内部用，自动脱敏后给日志）
errorx.Internal("AIHUB_DB_FAILED", "数据库失败",
    errorx.WithMetadata("raw_error", err.Error()),
    errorx.WithMetadata("dsn", dsn),
)

// ✅ 可公开数据放 PublicMetadata（HTTP 响应可见）
errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithPublicMetadata("resource", "skill"),
    errorx.WithPublicMetadata("name", name),
)

// ❌ 永远不要把敏感数据放 PublicMetadata
errorx.Unauthorized("AUTH_TOKEN_INVALID", "token 无效",
    errorx.WithPublicMetadata("token", userToken),  // 泄漏！
)
```

`SafeMetadataOf` 会自动脱敏以下 key：`password / passwd / pwd / token / secret /
authorization / cookie / credential / private_key / apikey / api_key`。

---

## 6. 消费方模式（logx/httpx/grpcx/auditx/metricsx/workerx）

如果你在写消费方代码（不是业务代码），用 `inspect` 函数，**不要类型断言**：

```go
// ✅ 正确
code := errorx.CodeOf(err)
status := errorx.HTTPStatusOf(err)
fields := errorx.Fields(err)           // 日志字段
labels := errorx.MetricsLabels(err)    // Prometheus 标签（低基数）

// ❌ 错误：直接类型断言
if e, ok := err.(*errorx.Error); ok {  // 不识别第三方包装的错误
    code := e.Code()
}
```

`CodeOf` / `HTTPStatusOf` 等函数对 `nil` 和未知错误安全：

```text
CodeOf(nil)             → CodeOK
CodeOf(errors.New("x")) → CodeInternal
HTTPStatusOf(nil)       → 200
HTTPStatusOf(unknown)   → 500
```

---

## 7. 调试技巧

```go
err := errorx.Internal("AIHUB_SKILL_QUERY_FAILED", "查询技能失败",
    errorx.WithCause(errors.New("pq: connection refused")),
    errorx.WithRequestID("req_123"),
    errorx.WithStack(),
)

// 安全 message（可返回前端）
fmt.Println(err)
// 查询技能失败

// 完整调试信息（开发用）
fmt.Printf("%+v\n", err)
// error_code=AIHUB_SKILL_QUERY_FAILED message="查询技能失败" http_status=500 ...

// 调用栈
for _, frame := range errorx.StackOf(err) {
    fmt.Printf("%s\n  %s:%d\n", frame.Function, frame.File, frame.Line)
}
```

---

## 8. 完整 handler 示例

```go
package handler

import (
    "context"
    "database/sql"
    "errors"

    "github.com/aisphereio/kernel/errorx"
)

type SkillHandler struct {
    rt  *Runtime
    svc SkillService
}

func (h *SkillHandler) Create(ctx context.Context, req *CreateSkillRequest) (*Skill, error) {
    // 1. 参数校验
    if req.Name == "" {
        return nil, errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "技能名称不能为空")
    }
    if len(req.Name) > 128 {
        return nil, errorx.BadRequest("AIHUB_SKILL_NAME_TOO_LONG", "技能名称不能超过 128 字符",
            errorx.WithPublicMetadata("field", "name"),
            errorx.WithPublicMetadata("max", 128),
        )
    }

    // 2. 鉴权
    p := principalFromContext(ctx)
    if err := h.rt.Access.Require(ctx, access.Check{
        Resource: "aihub:skill:*",
        Action:   "skill.create",
    }); err != nil {
        return nil, errorx.Forbidden("AIHUB_SKILL_CREATE_DENIED", "没有创建技能的权限",
            errorx.WithCause(err),
            errorx.WithMetadata("subject_id", p.SubjectID),
        )
    }

    // 3. 业务调用
    skill, err := h.svc.Create(ctx, &Skill{
        Name:    req.Name,
        OwnerID: p.SubjectID,
    })
    if err != nil {
        // svc 已经返回 errorx，直接透传
        return nil, err
    }

    return skill, nil
}

// 仓库层把底层错误转换成 errorx
func (r *SkillRepo) Find(ctx context.Context, id string) (*Skill, error) {
    var row skillModel
    err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
            errorx.WithCause(err),
            errorx.WithMetadata("skill_id", id),
            errorx.WithPublicMetadata("resource", "skill"),
        )
    }
    if err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
            errorx.WithMessage("查询技能失败"),
            errorx.WithRetryable(true),
            errorx.WithMetadata("skill_id", id),
        )
    }
    return toBiz(&row), nil
}
```

---

## 9. 验收清单（写完 errorx 代码后自检）

- [ ] 所有 `return err` 都换成 `return errorx.Wrap(err, "DOMAIN_RESOURCE_FAILED", ...)` 或具体构造器
- [ ] 错误码符合 `{DOMAIN}_{RESOURCE}_{REASON}` 格式，全大写蛇形
- [ ] 错误码不含动态 ID（动态信息在 metadata）
- [ ] 敏感数据在 `Metadata`，不在 `PublicMetadata`
- [ ] 数据库错误用 `Wrap(err, ...)` 保留 cause
- [ ] 上游超时/不可用错误标记 `WithRetryable(true)`
- [ ] 测试代码 `go test ./...` 通过
- [ ] `go vet ./...` 通过

---

## 10. 相关文档

- `errorx/README.md` — 单一入口用户指南
- `errorx/doc.go` — `go doc errorx` 输出源
- `errorx/example_test.go` — 所有构造器的 Go 标准 Example
- `docs/design/errorx.md` — 1254 行设计规范（深度参考）
- `docs/contracts/errorx.md` — 不可破坏契约
- `docs/process/errorx-acceptance-checklist.md` — CI 验收清单
- `examples/errorx-basic/` — 最小可运行示例
- `examples/errorx-http/` — HTTP handler 完整示例
