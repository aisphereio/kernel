# 禁止模式

## 已移除的包

```go
import "github.com/aisphereio/kernel/errors"
```

禁止使用。请改用：

```go
import "github.com/aisphereio/kernel/errorx"
```

## 裸业务错误

在 API/业务边界禁止使用：

```go
return errors.New("技能未找到")
return fmt.Errorf("技能 %s 未找到", id)
```

请改用：

```go
return errorx.NotFound(
    "AIHUB_SKILL_NOT_FOUND",
    "技能未找到",
    errorx.WithMetadata("skill_id", id),
)
```

## 动态错误码

禁止使用：

```go
return errorx.NotFound(errorx.Code("SKILL_"+id+"_NOT_FOUND"), "技能未找到")
```

错误码必须是稳定且低基数（low-cardinality）的字符串常量。
