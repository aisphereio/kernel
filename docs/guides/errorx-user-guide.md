# errorx User Guide

## 1. What to return

Return `errorx` for every API/business failure.

```go
return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "skill not found")
```

Do not return raw `errors.New` or `fmt.Errorf` from handler/service/repository code when the error can cross a module or API boundary.

## 2. Wrapping a lower-level cause

```go
skill, err := repo.Find(ctx, id)
if errors.Is(err, sql.ErrNoRows) {
    return nil, errorx.NotFound(
        "AIHUB_SKILL_NOT_FOUND",
        "skill not found",
        errorx.WithCause(err),
        errorx.WithMetadata("skill_id", id),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
if err != nil {
    return nil, errorx.Internal(
        "AIHUB_SKILL_QUERY_FAILED",
        "failed to query skill",
        errorx.WithCause(err),
        errorx.WithMetadata("skill_id", id),
    )
}
```

The public message is stable and safe. The original cause is preserved for `errors.Is` / `errors.As` and logs.

## 3. HTTP adapter behavior

A handler returning:

```go
return errorx.Forbidden("IAM_PERMISSION_DENIED", "permission denied")
```

becomes:

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json
```

```json
{
  "code": "IAM_PERMISSION_DENIED",
  "message": "permission denied"
}
```

## 4. gRPC adapter behavior

The gRPC transport converts `errorx` into `status.Status` and writes `errdetails.ErrorInfo`:

- gRPC code: derived from `errorx.GRPCCode()`
- ErrorInfo reason: `errorx.Code()`
- ErrorInfo metadata: request id, trace id and public metadata

The gRPC client adapter converts remote `status.Status` back into `errorx` when possible.

## 5. Logging and metrics

Use logger-neutral fields:

```go
fields := errorx.Fields(err)
labels := errorx.MetricsLabels(err)
```

`MetricsLabels` is intentionally low-cardinality. It does not include message, request id, trace id, object ids or raw metadata.

## 6. Retry decisions

```go
if errorx.RetryableOf(err) {
    retryLater()
}
```

Default retryable errors:

- `TOO_MANY_REQUESTS`
- `SERVICE_UNAVAILABLE`
- `TIMEOUT`

You may override with `errorx.WithRetryable(true)`.

## 7. Validation scenario

```go
return nil, errorx.BadRequest(
    "REQUEST_VALIDATE_FAILED",
    "request validation failed",
    errorx.WithCause(err),
    errorx.WithMetadata("validation_error", err.Error()),
    errorx.WithPublicMetadata("field", "name"),
)
```

## 8. Upstream model call scenario

```go
return nil, errorx.Timeout(
    "MODEL_UPSTREAM_TIMEOUT",
    "model upstream timeout",
    errorx.WithCause(err),
    errorx.WithRetryable(true),
    errorx.WithMetadata("model", "qwen3-asr"),
)
```

## 9. Repository not-found scenario

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, errorx.NotFound(
        "AIHUB_RESOURCE_NOT_FOUND",
        "resource not found",
        errorx.WithCause(err),
        errorx.WithPublicMetadata("resource", "skill"),
    )
}
```

## 10. Forbidden scenario

```go
return nil, errorx.Forbidden(
    "IAM_PERMISSION_DENIED",
    "permission denied",
    errorx.WithMetadata("subject_id", subjectID),
    errorx.WithMetadata("resource", resource),
    errorx.WithPublicMetadata("resource", "skill"),
)
```
