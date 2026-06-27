# Recipe: errorx 标准用法

## 参数错误

```go
if req.Name == "" {
    return errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "技能名称不能为空")
}
```

## 无权限

```go
return errorx.Forbidden("AIHUB_SKILL_DELETE_DENIED", "没有删除技能的权限",
    errorx.WithMetadata("skill_id", skillID),
)
```

## 资源不存在

```go
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", skillID),
)
```

## 数据库失败

```go
if err != nil {
    return errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
        errorx.WithMessage("查询技能失败"),
        errorx.WithRetryable(true),
        errorx.WithMetadata("skill_id", skillID),
    )
}
```

## 模型上游超时

```go
return errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "模型服务超时",
    errorx.WithRetryable(true),
    errorx.WithMetadata("provider", provider),
)
```
