# protoc-gen-go-errors

This directory is retained for proto contract compatibility, but it no longer generates the old `github.com/aisphereio/kernel/errors` runtime error type.

It now generates helpers backed by `github.com/aisphereio/kernel/errorx`.

The binary name and protoc flag remain:

```bash
--go-errors_out=paths=source_relative:.
```

This avoids forcing every existing proto file and build script to change at once.

Generated helpers:

```go
func IsXxx(err error) bool
func NewXxx(message string, opts ...errorx.Option) *errorx.Error
func ErrorXxx(format string, args ...interface{}) *errorx.Error
```

The proto annotation import also remains compatible:

```proto
import "errors/errors.proto";

enum UserErrorReason {
  option (errors.default_code) = 500;
  USER_NOT_FOUND = 0 [(errors.code) = 404];
}
```

Runtime rule: do not import `github.com/aisphereio/kernel/errors`; use `github.com/aisphereio/kernel/errorx`.
