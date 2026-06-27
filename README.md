# Aisphere Kernel

Aisphere Kernel is a breaking-rewrite microservice foundation for Aisphere projects.

Current direction:

- `errorx` is the only runtime/business error semantics package.
- The legacy root package `github.com/aisphereio/kernel/errors` has been removed.
- HTTP/gRPC/middleware/selector/contrib adapters now produce or consume `errorx`.
- Proto error-code generation is retained, but it now generates `errorx` helpers instead of old `errors.Error` helpers.
- Business code must not return raw `errors.New` or `fmt.Errorf` as API/business errors.
- Transport layers expose stable `error_code` values instead of Kratos-style `code + reason` status objects.

## Error model

Use `github.com/aisphereio/kernel/errorx` everywhere in handler, service, repository, worker and adapter code.

```go
return errorx.NotFound(
    "AIHUB_SKILL_NOT_FOUND",
    "skill not found",
    errorx.WithMetadata("skill_id", skillID),
    errorx.WithPublicMetadata("resource", "skill"),
)
```

HTTP response shape:

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "skill not found",
  "metadata": {
    "resource": "skill"
  }
}
```

HTTP status stays in the HTTP status code. The JSON `code` field is the stable business error code.

## Proto error generation

The proto generation capability is not deleted. It is converted to produce `errorx` helpers.

Retained compatibility pieces:

- `third_party/errors/errors.proto`
- `cmd/protoc-gen-go-errors/`
- `--go-errors_out=paths=source_relative:.`

The name is kept for proto compatibility with existing Kratos-style annotations, but the generated Go code imports `github.com/aisphereio/kernel/errorx` and returns `*errorx.Error`.

Example proto:

```proto
import "errors/errors.proto";

enum SkillErrorReason {
  option (errors.default_code) = 500;

  SKILL_NOT_FOUND = 0 [(errors.code) = 404];
  SKILL_FORBIDDEN = 1 [(errors.code) = 403];
}
```

Generated helpers look like:

```go
func IsSkillNotFound(err error) bool
func NewSkillNotFound(message string, opts ...errorx.Option) *errorx.Error
func ErrorSkillNotFound(format string, args ...interface{}) *errorx.Error
```

So old proto contracts can remain, while runtime errors are standardized on `errorx`.

## What was removed

The following old runtime package is intentionally gone:

- `errors/`
- imports of `github.com/aisphereio/kernel/errors`

The proto generator and proto extension directory are retained because they are part of the contract-generation toolchain, not the old runtime error package.

## Verify

Use Go 1.25+ locally. Linux/macOS can use Make directly:

```bash
make tools
make test-errorx
make verify-errorx
make test
make vet
```

Windows is a first-class path. You can either keep using Make:

```powershell
make tools
make test-cmd
make verify-errorx
```

or run the cmd wrappers directly, which also avoids PowerShell script-signing issues:

```bat
scripts\tools.cmd
scripts\test-cmd.cmd
scripts\verify-errorx.cmd
```

## Documentation

Start with:

- `errorx/README.md`
- `docs/guides/errorx-user-guide.md`
- `docs/contracts/errorx.md`
- `docs/process/errorx-acceptance-checklist.md`
