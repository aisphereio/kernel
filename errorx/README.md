# errorx

`errorx` is Aisphere Kernel's canonical error package. It replaces the old Kratos-derived `errors` package completely.

## Design goals

- stable business error codes, for example `AIHUB_SKILL_NOT_FOUND`
- transport-neutral core with only standard-library dependencies
- explicit HTTP status and gRPC code mapping
- safe public metadata for API responses
- private metadata for logs/audit/debugging
- `errors.Is` / `errors.As` compatibility
- retryable/category/severity fields for workers, metrics and alerting
- no raw internal error leakage to clients

## Basic usage

```go
func GetSkill(ctx context.Context, id string) (*Skill, error) {
    skill, err := repo.FindSkill(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, errorx.NotFound(
                "AIHUB_SKILL_NOT_FOUND",
                "skill not found",
                errorx.WithCause(err),
                errorx.WithMetadata("skill_id", id),
                errorx.WithPublicMetadata("resource", "skill"),
            )
        }
        return nil, errorx.Internal(
            "AIHUB_SKILL_QUERY_FAILED",
            "failed to query skill",
            errorx.WithCause(err),
            errorx.WithMetadata("skill_id", id),
        )
    }
    return skill, nil
}
```

## Constructors

Use semantic constructors instead of manually setting status:

```go
errorx.BadRequest("REQUEST_VALIDATE_FAILED", "request validation failed")
errorx.Unauthorized("AUTH_TOKEN_MISSING", "authorization token is missing")
errorx.Forbidden("IAM_PERMISSION_DENIED", "permission denied")
errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "skill not found")
errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "skill already exists")
errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "too many requests")
errorx.Unavailable("MODEL_UPSTREAM_UNAVAILABLE", "model upstream unavailable")
errorx.Timeout("MODEL_UPSTREAM_TIMEOUT", "model upstream timeout")
errorx.Internal("AIHUB_INTERNAL_ERROR", "internal server error")
```

## Metadata rules

`Metadata` is internal only. It is safe for logs/audit after redaction.

`PublicMetadata` may be returned by HTTP/gRPC adapters. Sensitive keys are redacted.

```go
err := errorx.BadRequest(
    "REQUEST_VALIDATE_FAILED",
    "request validation failed",
    errorx.WithMetadata("raw_validator_error", err.Error()),
    errorx.WithPublicMetadata("field", "name"),
)
```

Never put tokens, passwords, secrets, cookies, authorization headers or private keys into public metadata.

## Inspection helpers

```go
errorx.CodeOf(err)
errorx.MessageOf(err)
errorx.HTTPStatusOf(err)
errorx.GRPCCodeOf(err)
errorx.RetryableOf(err)
errorx.MetadataOf(err)
errorx.SafeMetadataOf(err)
errorx.PublicMetadataOf(err)
errorx.Fields(err)
errorx.MetricsLabels(err)
```

## Error-code contract

Codes must be upper snake case:

```go
const ErrSkillNotFound errorx.Code = "AIHUB_SKILL_NOT_FOUND"

func TestErrorCodes(t *testing.T) {
    errorx.MustValidCodes(ErrSkillNotFound)
}
```

## Test

```bash
go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
go test ./errorx -bench=.
go run ./examples/errorx-basic
```
