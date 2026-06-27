# errorx 用户指南

## 1. 返回什么

对每个 API/业务失败返回 `errorx`。

```go
return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能未找到")
```

在 handler/service/repository 代码中，当错误可能跨越模块或 API 边界时，不要返回裸的 `errors.New` 或 `fmt.Errorf`。

## 2. 包装底层错误

```go
skill, err := repo.Find(ctx, id)
if errors.Is(err, sql.ErrNoRows) {
    return nil, errorx.NotFound(
        "AIHUB_SKILL_NOT_FOUND",
        "技能未找到",
        errorx.WithCause(err),
        errorx.WithMetadata("skill_id", id),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
if err != nil {
    return nil, errorx.Internal(
        "AIHUB_SKILL_QUERY_FAILED",
        "查询技能失败",
        errorx.WithCause(err),
        errorx.WithMetadata("skill_id", id),
    )
}
```

公开消息是稳定且安全的。原始底层错误被保留，可通过 `errors.Is` / `errors.As` 和日志访问。

## 3. HTTP 适配器行为

handler 返回：

```go
return errorx.Forbidden("IAM_PERMISSION_DENIED", "权限被拒绝")
```

变为：

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json
```

```json
{
  "code": "IAM_PERMISSION_DENIED",
  "message": "权限被拒绝"
}
```

## 4. gRPC 适配器行为

gRPC 传输层将 `errorx` 转换为 `status.Status` 并写入 `errdetails.ErrorInfo`：

- gRPC code：由 `errorx.GRPCCode()` 推导
- ErrorInfo reason：`errorx.Code()`
- ErrorInfo metadata：request id、trace id 及 public metadata

gRPC 客户端适配器将远程的 `status.Status` 尽可能转回 `errorx`。

## 5. 日志和指标

使用与日志器无关的字段：

```go
fields := errorx.Fields(err)
labels := errorx.MetricsLabels(err)
```

`MetricsLabels` 刻意保持低基数。它不包含 message、request id、trace id、对象 ID 或原始 metadata。

## 6. 重试决策

```go
if errorx.RetryableOf(err) {
    retryLater()
}
```

默认可重试的错误：

- `TOO_MANY_REQUESTS`
- `SERVICE_UNAVAILABLE`
- `TIMEOUT`

你可以用 `errorx.WithRetryable(true)` 覆盖默认行为。

## 7. 参数校验场景

```go
return nil, errorx.BadRequest(
    "REQUEST_VALIDATE_FAILED",
    "请求参数校验失败",
    errorx.WithCause(err),
    errorx.WithMetadata("validation_error", err.Error()),
    errorx.WithPublicMetadata("field", "name"),
)
```

## 8. 模型上游调用场景

```go
return nil, errorx.Timeout(
    "MODEL_UPSTREAM_TIMEOUT",
    "模型上游超时",
    errorx.WithCause(err),
    errorx.WithRetryable(true),
    errorx.WithMetadata("model", "qwen3-asr"),
)
```

## 9. 仓库层未找到场景

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, errorx.NotFound(
        "AIHUB_RESOURCE_NOT_FOUND",
        "资源未找到",
        errorx.WithCause(err),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
```

## 10. 无权限场景

```go
return nil, errorx.Forbidden(
    "IAM_PERMISSION_DENIED",
    "权限被拒绝",
    errorx.WithMetadata("subject_id", subjectID),
    errorx.WithMetadata("resource", resource),
    errorx.WithPublicMetadata("resource", "skill"),
)
```
