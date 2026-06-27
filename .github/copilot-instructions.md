# Copilot Instructions — Aisphere Kernel

This file is loaded automatically by GitHub Copilot Chat. Keep it short.

## Project context

You are working inside **Aisphere Kernel** — an AI-native Go application kernel.
Business code MUST use Kernel APIs. Do not invent infrastructure.

## Hard rules

### 1. Error handling — use errorx

In `handler/`, `service/`, `repository/`, `worker/` code:

```go
// ❌ FORBIDDEN
return errors.New("skill not found")
return fmt.Errorf("create failed: %w", err)
return nil, err
panic(err)

// ✅ REQUIRED
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", id),
)
return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
    errorx.WithMessage("创建技能失败"),
)
```

Constructor cheatsheet:

- `errorx.BadRequest` — 400, validation failed
- `errorx.Unauthorized` — 401, not authenticated
- `errorx.Forbidden` — 403, no permission
- `errorx.NotFound` — 404, resource missing
- `errorx.Conflict` — 409, already exists
- `errorx.RequestTimeout` — 408, request timeout
- `errorx.TooManyRequests` — 429, rate limited
- `errorx.ClientClosed` — 499, client disconnected
- `errorx.Internal` — 500, internal error
- `errorx.Unavailable` — 503, upstream down
- `errorx.Timeout` — 504, upstream timeout

Error code format: `{DOMAIN}_{RESOURCE}_{REASON}` (uppercase snake). Dynamic values in `WithMetadata`, never in code.

### 2. Logging — use logx

```go
// ❌ FORBIDDEN
log.Printf(...)
fmt.Println(...)
slog.Info(...)
zap.L().Info(...)

// ✅ REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created", logx.String("skill_id", id))
```

### 3. Forbidden imports in business code

- `database/sql` → use kernel `dbx`
- `net/http` (server setup) → use kernel `httpx`
- `go-redis` → use kernel `cache`
- `minio` SDK → use kernel `objectstore`
- `casdoor` SDK → use kernel `authn`
- `casbin` enforcer → use kernel `authz`
- `os.Getenv` → use kernel `config`

### 4. Standard flow

```text
Handler   → bind input → call service → return response (or errorx)
Service   → validate rules → check authz → call repo → record audit → return errorx
Repository → use kernel dbx → no HTTP/auth/audit logic → return errorx
```

## 📚 Examples — find by scenario BEFORE writing code

Before writing errorx code, look up the matching Example. All examples are
runnable (`go test ./errorx -run=Example -v`) and show exact output.

### Quick lookup

| Scenario | Example | File |
|---|---|---|
| Validation failed | `ExampleBadRequest` | `errorx/example_test.go` |
| Not authenticated | `ExampleUnauthorized` | `errorx/example_test.go` |
| No permission | `ExampleForbidden` | `errorx/example_test.go` |
| Resource missing | `ExampleNotFound` | `errorx/example_test.go` |
| Already exists | `ExampleConflict` | `errorx/example_test.go` |
| Upstream timeout | `ExampleTimeout` | `errorx/example_test.go` |
| Upstream down | `ExampleUnavailable` | `errorx/example_test.go` |
| Internal error | `ExampleInternal` | `errorx/example_test.go` |
| Wrap underlying error | `ExampleWrap` | `errorx/example_test.go` |
| Convert foreign error | `ExampleFrom` | `errorx/example_test.go` |
| Add internal metadata | `ExampleWithMetadata` | `errorx/example_test.go` |
| Add public metadata | `ExampleWithPublicMetadata` | `errorx/example_test.go` |
| Auto-redact secrets | `ExampleSafeMetadataOf` | `errorx/example_test.go` |
| Capture call stack | `ExampleWithStack` | `errorx/example_test.go` |
| Match by error code | `ExampleIsCode` | `errorx/example_test.go` |
| Debug with %+v | `ExampleError_Format` | `errorx/example_test.go` |
| Validate error code | `ExampleIsValidCode` | `errorx/example_test.go` |

### Complete business scenarios (in `errorx/example_business_test.go`)

| Scenario | Example |
|---|---|
| Repository DB error → errorx | `ExampleBusiness_repositoryLayer` |
| Service validation + conflict | `ExampleBusiness_serviceLayer` |
| Upstream model timeout | `ExampleBusiness_upstreamTimeout` |
| Authz denied | `ExampleBusiness_authzDenied` |
| Worker retry decision | `ExampleBusiness_workerRetry` |
| HTTP response JSON shape | `ExampleBusiness_httpResponse` |
| Audit record | `ExampleBusiness_auditRecord` |
| Log entry with redaction | `ExampleBusiness_logEntry` |
| Metrics labels | `ExampleBusiness_metricsLabels` |
| Multi-layer wrap | `ExampleBusiness_multiLayerWrap` |

### Runnable HTTP example

```bash
go run ./examples/errorx-http
# then:
curl -i 'http://localhost:18080/skills?id=missing'
```

See `examples/errorx-http/README.md` for 7 curl scenarios.

## Pre-commit checks

```bash
go test ./...
go vet ./...
./scripts/check-errorx-usage.sh
golangci-lint run
```

## When unsure

- Error handling → Example table above + `docs/ai/errorx.md` + `errorx/README.md`
- Logging → `logx/README.md`
- New module → `docs/process/module-acceptance.md`
- Project rules → `AGENTS.md`
