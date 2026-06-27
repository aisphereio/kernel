# Aisphere Kernel AI 编码快速入门

对每个 API/业务错误使用 `errorx`。

```go
return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能未找到")
```

包装底层错误时：

```go
return nil, errorx.Internal(
    "AIHUB_SKILL_QUERY_FAILED",  // 稳定错误码
    "查询技能失败",               // 人类可读消息
    errorx.WithCause(err),        // 保留原始底层错误
    errorx.WithMetadata("skill_id", skillID), // 内部排障上下文
)
```

不要导入 `github.com/aisphereio/kernel/errors`。该包已被移除。

## 包装底层错误

```go
import "errors"

func GetSkill(ctx context.Context, id string) (*Skill, error) {
    skill, err := repo.FindSkill(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, errorx.NotFound(
                "AIHUB_SKILL_NOT_FOUND",
                "技能未找到",
                errorx.WithCause(err),
                errorx.WithMetadata("skill_id", id),
                errorx.WithPublicMetadata("resource", "skill"),
            )
        }
        return nil, errorx.Internal(
            "AIHUB_SKILL_QUERY_FAILED",
            "查询技能失败",
            errorx.WithCause(err),
            errorx.WithMetadata("skill_id", id),
        )
    }
    return skill, nil
}
```

提供给用户的消息是稳定且安全的。原始底层错误通过 `errors.Is` / `errors.As` 和日志保留。

## 行为规则

引用 `docs/contracts/errorx.md` 中的契约。
