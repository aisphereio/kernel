# Aisphere Kernel Docs

This documentation follows the breaking error-system rewrite.

Current source of truth:

- `errorx` is the only Kernel error package.
- `github.com/aisphereio/kernel/errors` no longer exists.
- API/business errors must use `errorx`.
- HTTP/gRPC/middleware/metrics/tracing adapters must consume `errorx` helpers.

Recommended reading order:

1. `errorx/README.md`
2. `docs/guides/errorx-user-guide.md`
3. `docs/contracts/errorx.md`
4. `docs/process/errorx-acceptance-checklist.md`
5. `docs/ai/99-forbidden-patterns.md`
