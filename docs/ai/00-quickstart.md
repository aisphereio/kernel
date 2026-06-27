# AI Coding Quickstart for Aisphere Kernel

Use `errorx` for every API/business error.

```go
return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "skill not found")
```

When wrapping a cause:

```go
return nil, errorx.Internal(
    "AIHUB_SKILL_QUERY_FAILED",
    "failed to query skill",
    errorx.WithCause(err),
    errorx.WithMetadata("skill_id", skillID),
)
```

Do not import `github.com/aisphereio/kernel/errors`. It has been removed.
