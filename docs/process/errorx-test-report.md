# errorx Test Report

Date: 2026-06-27

## Local sandbox result

Passed:

```bash
go test ./errorx
```

The sandbox only has Go 1.23.2, while the project requires Go 1.25.0. I temporarily lowered `go.mod` only for the local `./errorx` package test and restored it afterward.

Not fully runnable in the sandbox:

```bash
go test ./...
```

Reason: the sandbox cannot download missing Go module dependencies from `proxy.golang.org`, and the project requires Go 1.25+ for the full repository.

## Required developer verification

Run locally with Go 1.25+ and network/module cache ready:

```bash
make test-errorx
make verify-errorx
make test
make vet
```

## Migration result to verify

- old runtime `errors/` package removed
- old imports of `github.com/aisphereio/kernel/errors` removed
- `cmd/protoc-gen-go-errors` retained and converted to generate `errorx` helpers
- `third_party/errors/errors.proto` retained for proto enum option compatibility
- `kernel proto client` still emits `--go-errors_out`, but generated code now returns `*errorx.Error`
- HTTP error response migrated to `code/message/request_id/trace_id/metadata`
- gRPC adapter added for `errorx` to/from `status.Status`
- middleware/selector/contrib migrated to `errorx`
