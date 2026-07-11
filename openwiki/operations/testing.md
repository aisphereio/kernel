# Testing & Operations

Aisphere Kernel has a comprehensive testing strategy spanning unit tests, integration tests, acceptance checklists, and CI validation.

## Testing Strategy

### Verification Priority

1. **Single-package tests** — `go test ./<package>/...` (fast, local)
2. **Generated project tests** — `make verify` in generated service (medium)
3. **GitHub Actions long builds** — Full governance tests, layout smoke tests (slow)

### Local vs CI

| Scope | Local | GitHub Actions |
|---|---|---|
| Targeted tests | `go test ./serverx ./validation/...` | Full suite |
| Dependency download | Manual `go mod download` | Automatic |
| Governance tests | — | Full governance validation |
| Layout/fullflow compile smoke | — | kernel-layout smoke tests |
| Build logs | — | Full build log capture |

## Acceptance Checklists

Each major package has a detailed acceptance checklist in `/docs/process/`:

### errorx Acceptance (`docs/process/errorx-acceptance-checklist.md`)

| Category | Checks |
|---|---|
| Static | No old `errors/` package imports, `protoc-gen-go-errors` exists |
| Unit | Constructors, status mapping, gRPC code, metadata, redaction, `errors.Is`/`errors.As`, clone, stack capture, proto enum generation |
| Integration | 11 scenarios: HTTP 404/403, gRPC error recovery, panic recovery, validate, ratelimit, circuit breaker, selector, metrics labels, logs, proto enum helpers |

### logx Acceptance (`docs/process/logx-acceptance-checklist.md`)

| Category | Checks |
|---|---|
| Static | No `log.Printf`/`fmt.Println`/`slog`/`zap` in business code |
| Unit | Logger interface, Field constructors, Context fields, nil safety, redaction, sampling, filtering, helpers, HTTP/RPC middleware, level control, test logger, format |
| Integration | 15 scenarios: New, DefaultConfig, FromContext, Err extraction, LogAccess, LogExternalCall, HTTP middleware, Recovery, Redaction, Sampling, TestLogger, LevelController, LevelHTTPHandler |

### configx Acceptance (`docs/process/configx-acceptance-checklist.md`)

| Category | Checks |
|---|---|
| Static | No old imports, no `os.Getenv` in business code, no viper |
| Unit | Get, GetOrDefault, MustGet, Load, Watch, Close, nil safety, placeholder resolution, merge, Value methods |
| Example | 19 public API examples + 10 business scenarios |
| Integration | Watch fires, multiple observers, reload, close, cross-package |
| Coverage | `configx` ≥ 85%, `configx/env` ≥ 90%, `configx/file` ≥ 80% |
| Fuzz | No panic on arbitrary JSON/YAML |
| Benchmark | Get < 200ns/op, Value < 100ns/op, Scan < 5000ns/op, Load < 50000ns/op |

### dbx Acceptance (`docs/process/dbx-acceptance-checklist.md`)

| Category | Checks |
|---|---|
| Static | No `database/sql`/`gorm.io/gorm`/`pgx`/etc. in business code |
| Unit | Nil config, unknown driver, sentinel errors, SafeUpsert protection |
| Integration | Postgres + MySQL: Create, FindOne, DuplicateKey, SafeUpsert, SoftDelete, Increment, Paginate, InTx commit/rollback/panic |
| Coverage | Unit ≥ 40%, Integration ≥ 75% |

## Module Acceptance Baseline (`docs/process/module-acceptance.md`)

Every module must test:

- Success path
- Validation error
- Not found
- Permission denied
- Upstream unavailable/timeout
- Internal failure

## Key Test Files

| Test File | What It Tests |
|---|---|
| `/app_test.go` | App lifecycle (start/stop, signal handling) |
| `/options_test.go` | App options |
| `/authn/trusted_headers_test.go` | Gateway claim header principal restoration |
| `/contextx/authn_test.go` | Lossless principal mirror |
| `/authz/authz_test.go` | Authorization service (recently modified) |

## CI Pipeline

The CI pipeline (`.github/workflows/`) includes:

- **openwiki-update.yml** — Scheduled OpenWiki knowledge base refresh
- Standard Go CI: lint, test, build
- Proto contract validation via `make proto-check`
- Full governance test suite

## Windows Compatibility

The toolchain supports Windows development:

- PowerShell Execution Policy handled via `.cmd` wrappers with `-ExecutionPolicy Bypass`
- Makefile is Windows-compatible
- `.cmd` scripts available as alternatives to shell scripts

## Related Documentation

- [Quickstart](../quickstart.md)
- [docs/process/module-acceptance.md](../../docs/process/module-acceptance.md)
- [docs/process/configx-acceptance-checklist.md](../../docs/process/configx-acceptance-checklist.md)
- [docs/process/dbx-acceptance-checklist.md](../../docs/process/dbx-acceptance-checklist.md)
- [docs/process/errorx-acceptance-checklist.md](../../docs/process/errorx-acceptance-checklist.md)
- [docs/process/logx-acceptance-checklist.md](../../docs/process/logx-acceptance-checklist.md)
- [docs/process/windows-script-policy.md](../../docs/process/windows-script-policy.md)
- [docs/ai/github-actions-build-delegation.md](../../docs/ai/github-actions-build-delegation.md)
- [Makefile](../../Makefile)