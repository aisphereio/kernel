# errorx Contract

This contract is mandatory for Kernel and business modules.

## Package rule

`github.com/aisphereio/kernel/errorx` is the only Kernel error package.

The following imports are forbidden in Kernel/business API error paths:

```go
import "github.com/aisphereio/kernel/errors"
```

The package no longer exists and must not be recreated.

## Code rule

Error codes are stable, upper-snake-case strings.

Valid:

```text
AIHUB_SKILL_NOT_FOUND
REQUEST_VALIDATE_FAILED
MODEL_UPSTREAM_TIMEOUT
```

Invalid:

```text
skill-not-found
aihub.skill.not_found
notFound
```

Use:

```go
errorx.IsValidCode(code)
errorx.MustValidCodes(code1, code2)
```

## Response rule

HTTP error response body:

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "skill not found",
  "request_id": "optional",
  "trace_id": "optional",
  "metadata": {}
}
```

HTTP status is not duplicated as the error identity.

## Metadata rule

- `Metadata`: internal, logged through `SafeMetadataOf` / `Fields`
- `PublicMetadata`: transport-safe, may appear in API responses
- sensitive public metadata is redacted

Sensitive keys include password, token, secret, authorization, cookie, credential, private key and API key variants.

## Interop rule

The following must work:

```go
errors.Is(wrapped, cause)
errors.As(err, &target)
errors.Is(errorx.NotFound("X", "x"), errorx.NotFound("X", "other"))
```

## Transport rule

HTTP and gRPC transports must not depend on the removed legacy `errors` package. They must convert via `errorx.From`, `errorx.HTTPStatusOf`, `errorx.GRPCCodeOf` and `errorx.CodeOf`.
