# errorx 契约

本契约对 Kernel 及业务模块是强制性要求。

## 包使用规则

`github.com/aisphereio/kernel/errorx` 是唯一的 Kernel 错误包。

在 Kernel/业务 API 错误路径中禁止以下导入：

```go
import "github.com/aisphereio/kernel/errors"
```

该包已不再存在，且不得重新创建。

## 错误码规则

错误码是稳定的、大写蛇形（upper-snake-case）字符串。

合法：

```text
AIHUB_SKILL_NOT_FOUND
REQUEST_VALIDATE_FAILED
MODEL_UPSTREAM_TIMEOUT
```

不合法：

```text
skill-not-found
aihub.skill.not_found
notFound
```

使用：

```go
errorx.IsValidCode(code)
errorx.MustValidCodes(code1, code2)
```

## 响应规则

HTTP 错误响应体：

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "技能未找到",
  "request_id": "可选",
  "trace_id": "可选",
  "metadata": {}
}
```

HTTP 状态码不会作为错误标识重复出现。

## Metadata 规则

- `Metadata`：内部使用，通过 `SafeMetadataOf` / `Fields` 写入日志
- `PublicMetadata`：传输安全，可能出现在 API 响应中
- 敏感的 public metadata 会被自动脱敏

敏感键包括 password、token、secret、authorization、cookie、credential、private key 和 API key 等变体。

## 互操作规则

以下操作必须正常工作：

```go
errors.Is(wrapped, cause)
errors.As(err, &target)
errors.Is(errorx.NotFound("X", "x"), errorx.NotFound("X", "other"))
```

## 传输规则

HTTP 和 gRPC 传输层不得依赖已移除的旧 `errors` 包。它们必须通过 `errorx.From`、`errorx.HTTPStatusOf`、`errorx.GRPCCodeOf` 和 `errorx.CodeOf` 进行转换。
