# Changelog

## Unreleased

### Breaking

- Removed legacy `errors/` package completely.
- Retained `third_party/errors/` proto extension files as the proto annotation contract.
- Retained and converted `cmd/protoc-gen-go-errors/` to generate `errorx` helpers.
- Retained `--go-errors_out` in Kernel proto client generation; it now emits `errorx` helpers.
- Restored Makefile install/test steps for the converted `protoc-gen-go-errors`.
- Added Windows-first toolchain wrappers: `scripts/tools.cmd`, `scripts/test-cmd.cmd`, and PowerShell implementations behind them.
- Updated Makefile targets for Windows `make tools`, `make test-cmd`, `make verify-errorx`, `make proto`, `make clean`, and verification helpers.
- Migrated Kernel HTTP/gRPC/middleware/selector/contrib adapters to `errorx`.

### Added

- HTTP `ErrorResponse` shape based on stable business `code` and safe public metadata.
- gRPC adapter for converting `errorx` to `status.Status` and remote `status.Status` back into `errorx`.
- Error-code validation helpers: `ValidateCode`, `MustCode`, `MustValidCodes`.
- Sensitive-key redaction for public metadata.
- Updated errorx user guide, contract and acceptance checklist.

### Notes

This release intentionally does not provide compatibility with `github.com/aisphereio/kernel/errors`.
