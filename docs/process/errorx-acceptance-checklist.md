# errorx Acceptance Checklist

## Static checks

- [ ] `grep -R "github.com/aisphereio/kernel/errors" --include="*.go" .` returns no result.
- [ ] Root runtime package `errors/` does not exist.
- [ ] `cmd/protoc-gen-go-errors` exists and generates `errorx` helpers.
- [ ] `third_party/errors/errors.proto` exists for proto enum option compatibility.
- [ ] Makefile installs/tests `cmd/protoc-gen-go-errors`.
- [ ] `kernel proto client` emits `--go-errors_out=paths=source_relative:.` so existing proto workflows still generate helpers.
- [ ] Generated helper template imports `github.com/aisphereio/kernel/errorx`, not `github.com/aisphereio/kernel/errors`.

## Unit checks

Run:

```bash
go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
(cd cmd/protoc-gen-go-errors && go test ./...)
```

Expected coverage areas:

- constructors
- status mapping
- gRPC code string mapping
- metadata and public metadata
- sensitive metadata redaction
- `errors.Is` / `errors.As`
- clone defensive copy
- stack capture
- fields and metrics labels
- foreign error conversion
- proto enum option helper generation to `errorx`

## Integration checks

Run on a full local machine with Go 1.25+ and dependencies available:

```bash
go test ./...
go vet ./...
go run ./examples/errorx-basic
```

Expected scenarios:

1. HTTP handler returns `errorx.NotFound`; client receives HTTP 404 and JSON `code`.
2. HTTP handler returns `errorx.Forbidden`; client receives HTTP 403 and no private metadata.
3. gRPC server returns `errorx.BadRequest`; client can recover `errorx.CodeOf(err)`.
4. Recovery middleware turns panic into `KERNEL_PANIC_RECOVERED`.
5. Validate middleware returns `REQUEST_VALIDATE_FAILED`.
6. Ratelimit middleware returns `RATE_LIMIT_EXCEEDED`.
7. Circuit breaker returns `CIRCUIT_BREAKER_OPEN`.
8. Selector returns `NO_AVAILABLE_NODE`.
9. Metrics labels include `error_code`, `http_status`, `grpc_code`, `retryable`, `category` only.
10. Logs include `error_code`, `http_status`, `grpc_code`, `category`, `severity` and redacted metadata.
11. Proto enum annotations with `errors/errors.proto` generate `NewXxx`, `ErrorXxx`, and `IsXxx` helpers returning/inspecting `errorx`.

## Windows commands

Windows can use Make directly:

```powershell
make tools
make test-cmd
make verify-errorx
```

Or run the wrapper scripts directly:

```bat
scripts\tools.cmd
scripts\test-cmd.cmd
scripts\verify-errorx.cmd
```

The `.cmd` wrappers launch PowerShell with process-level `ExecutionPolicy Bypass` and do not change user or machine policy.
