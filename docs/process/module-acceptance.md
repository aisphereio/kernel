# Module Acceptance

This checklist is the current Kernel module acceptance baseline after the `errorx` breaking rewrite.

## Error handling

- [ ] API/business errors use `errorx`.
- [ ] No module imports `github.com/aisphereio/kernel/errors`.
- [ ] No service/repository/handler returns raw `errors.New` or `fmt.Errorf` as an API/business error.
- [ ] Dynamic values are stored in metadata, not in error codes.
- [ ] Sensitive metadata is not exposed through public metadata.
- [ ] `errors.Is` / `errors.As` still works for wrapped causes.

## Tests

Each module should include scenarios for:

1. success path
2. validation error: `REQUEST_VALIDATE_FAILED`
3. not found: module-specific `*_NOT_FOUND`
4. permission denied: `IAM_PERMISSION_DENIED` or module-specific forbidden code
5. upstream unavailable/timeout with retryable behavior
6. internal failure with private metadata and safe public message

## Verification

```bash
go test ./...
go vet ./...
```

For errorx specifically:

```bash
go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
```
