# CLAUDE.md

This file is loaded automatically by Claude Code at the start of every session.
Keep it short �?full rules live in AGENTS.md and docs/ai/errorx.md.

## Prime directive

You are working inside **Aisphere Kernel** �?an AI-native Go application kernel.
**Do not invent infrastructure.** Use Kernel APIs only.

Before writing any business code (handler/service/repository/worker), read:
- `AGENTS.md` �?full project rules
- `docs/ai/errorx.md` �?errorx AI recipe (10 scenarios + forbidden patterns)

## Hard rules (violations will fail CI)

### 1. Error handling �?use errorx, never raw errors

In `handler/`, `service/`, `repository/`, `worker/` code:

```go
// �?FORBIDDEN
return errors.New("skill not found")
return fmt.Errorf("create failed: %w", err)
return nil, err
panic(err)

// �?REQUIRED
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", id),
)
return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
    errorx.WithMessage("创建技能失�?),
)
```

Constructor cheatsheet:

| Scenario | Constructor | HTTP |
|---|---|---:|
| Validation failed | `errorx.BadRequest` | 400 |
| Not authenticated | `errorx.Unauthorized` | 401 |
| No permission | `errorx.Forbidden` | 403 |
| Resource missing | `errorx.NotFound` | 404 |
| Already exists | `errorx.Conflict` | 409 |
| Upstream timeout | `errorx.Timeout` | 504 |
| Upstream down | `errorx.Unavailable` | 503 |
| Internal error | `errorx.Internal` | 500 |

Error code format: `{DOMAIN}_{RESOURCE}_{REASON}` (uppercase snake).
Dynamic values go in `WithMetadata`, NEVER in the error code.

### 2. Logging �?use logx, never raw log

```go
// �?FORBIDDEN in business code
log.Printf("...")
fmt.Println("...")
slog.Info("...")
zap.L().Info("...")

// �?REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created", logx.String("skill_id", id))
```

### 3. Forbidden imports in business code

```text
database/sql          �?use kernel dbx
net/http (server)     �?use kernel httpx
go-redis              �?use kernel cache
minio SDK             �?use kernel objectstore
casdoor SDK           �?use kernel authn
casbin enforcer       �?use kernel authz
os.Getenv             �?use kernel configx
```

### 4. Standard flow

```text
Handler   �?bind input �?call service �?return response (or errorx)
Service   �?validate rules �?check authz �?call repo �?record audit �?return errorx
Repository �?use kernel dbx �?no HTTP/auth/audit logic �?return errorx
```

## 📚 Examples �?find by scenario BEFORE writing code

**Before writing errorx code, look up the matching Example.** Examples are
runnable (`go test ./errorx -run=Example -v`) and show exact output.

### Constructor examples (11)

| Constructor | Example function | File |
|---|---|---|
| `BadRequest` | `ExampleBadRequest` | `errorx/example_test.go` |
| `Unauthorized` | `ExampleUnauthorized` | `errorx/example_test.go` |
| `Forbidden` | `ExampleForbidden` | `errorx/example_test.go` |
| `NotFound` | `ExampleNotFound` | `errorx/example_test.go` |
| `Conflict` | `ExampleConflict` | `errorx/example_test.go` |
| `RequestTimeout` | `ExampleRequestTimeout` | `errorx/example_test.go` |
| `TooManyRequests` | `ExampleTooManyRequests` | `errorx/example_test.go` |
| `ClientClosed` | `ExampleClientClosed` | `errorx/example_test.go` |
| `Internal` | `ExampleInternal` | `errorx/example_test.go` |
| `Unavailable` | `ExampleUnavailable` | `errorx/example_test.go` |
| `Timeout` | `ExampleTimeout` | `errorx/example_test.go` |

### Core constructors (3)

| Function | Example | When to use |
|---|---|---|
| `New` | `ExampleNew` | Custom error with non-default HTTP status |
| `Wrap` | `ExampleWrap`, `ExampleWrap_nil`, `ExampleWrap_seesThroughFmtErrorf` | Wrap underlying cause |
| `From` | `ExampleFrom`, `ExampleFrom_withOptions`, `ExampleFrom_foreignStatusCode` | Convert foreign error to errorx |

### Option examples (12)

| Option | Example | When to use |
|---|---|---|
| `WithMessage` | `ExampleWithMessage` | Override default message |
| `WithHTTPStatus` | `ExampleWithHTTPStatus` | Override default HTTP status (rare) |
| `WithGRPCCode` | `ExampleWithGRPCCode` | Override default gRPC code (rare) |
| `WithRetryable` | `ExampleWithRetryable` | Override default retryable |
| `WithCause` | `ExampleWithCause` | Preserve underlying error |
| `WithMetadata` | `ExampleWithMetadata` | Internal metadata (logs/audit) |
| `WithMetadataMap` | `ExampleWithMetadataMap` | Multiple metadata at once |
| `WithPublicMetadata` | `ExampleWithPublicMetadata` | Client-safe metadata |
| `WithPublicMetadataMap` | `ExampleWithPublicMetadataMap` | Multiple public metadata |
| `WithRequestID` | `ExampleWithRequestID` | Request ID for tracing |
| `WithTraceID` | `ExampleWithTraceID` | Trace ID for distributed tracing |
| `WithCategory` | `ExampleWithCategory` | Override category (rare) |
| `WithSeverity` | `ExampleWithSeverity` | Override severity for logx |
| `WithStack` | `ExampleWithStack` | Capture call stack (5xx only) |

### Inspect examples (13)

| Function | Example | Used by |
|---|---|---|
| `CodeOf` | `ExampleCodeOf`, `ExampleCodeOf_nil`, `ExampleCodeOf_foreignError` | httpx, auditx |
| `MessageOf` | `ExampleMessageOf` | httpx, auditx |
| `HTTPStatusOf` | `ExampleHTTPStatusOf`, `ExampleHTTPStatusOf_nil`, `ExampleHTTPStatusOf_foreignError` | httpx |
| `GRPCCodeOf` | `ExampleGRPCCodeOf` | grpcx |
| `RetryableOf` | `ExampleRetryableOf` | workerx |
| `MetadataOf` | `ExampleMetadataOf` | logx, auditx |
| `SafeMetadataOf` | `ExampleSafeMetadataOf` | logx (auto-redacts secrets) |
| `PublicMetadataOf` | `ExamplePublicMetadataOf` | httpx, grpcx |
| `RequestIDOf` | `ExampleRequestIDOf` | logx, auditx |
| `TraceIDOf` | `ExampleTraceIDOf` | logx, auditx |
| `CategoryOf` | `ExampleCategoryOf` | metricsx, alerting |
| `SeverityOf` | `ExampleSeverityOf` | logx |
| `StackOf` | `ExampleStackOf` | logx (debug only) |

### Predicate examples (12)

| Predicate | Example |
|---|---|
| `IsBadRequest` | `ExampleIsBadRequest` |
| `IsUnauthorized` | `ExampleIsUnauthorized` |
| `IsForbidden` | `ExampleIsForbidden` |
| `IsNotFound` | `ExampleIsNotFound` |
| `IsConflict` | `ExampleIsConflict` |
| `IsRequestTimeout` | `ExampleIsRequestTimeout` |
| `IsTooManyRequests` | `ExampleIsTooManyRequests` |
| `IsClientClosedRequest` | `ExampleIsClientClosedRequest` |
| `IsInternal` | `ExampleIsInternal` |
| `IsUnavailable` | `ExampleIsUnavailable` |
| `IsTimeout` | `ExampleIsTimeout` |
| `IsCode` | `ExampleIsCode` (match by exact code) |

### Consumer examples (logx / metricsx / auditx / workerx)

| Consumer | Example | File |
|---|---|---|
| logx fields | `ExampleFields`, `ExampleFields_nil`, `ExampleBusiness_logEntry` | `example_test.go`, `example_business_test.go` |
| Prometheus labels | `ExampleMetricsLabels`, `ExampleBusiness_metricsLabels` | both |
| Audit record | `ExampleBusiness_auditRecord` | `example_business_test.go` |
| Worker retry decision | `ExampleBusiness_workerRetry` | `example_business_test.go` |
| HTTP response shape | `ExampleBusiness_httpResponse` | `example_business_test.go` |

### Business scenario examples (10 complete flows)

| Scenario | Example | Layer |
|---|---|---|
| Repository DB error �?errorx | `ExampleBusiness_repositoryLayer` | repository |
| Service validation + authz + conflict | `ExampleBusiness_serviceLayer` | service |
| Upstream model timeout | `ExampleBusiness_upstreamTimeout` | service |
| Authz denied with metadata | `ExampleBusiness_authzDenied` | service |
| Worker retry decision | `ExampleBusiness_workerRetry` | worker |
| HTTP response JSON shape | `ExampleBusiness_httpResponse` | handler |
| Audit record | `ExampleBusiness_auditRecord` | auditx |
| Log entry with redaction | `ExampleBusiness_logEntry` | logx |
| Metrics labels (low cardinality) | `ExampleBusiness_metricsLabels` | metricsx |
| Multi-layer wrap (preserve chain) | `ExampleBusiness_multiLayerWrap` | cross-layer |

### Advanced examples

| Topic | Example |
|---|---|
| Clone (deep copy with override) | `ExampleError_Clone` |
| %+v debug format | `ExampleError_Format`, `ExampleError_Format_verbose` |
| As (extract from chain) | `ExampleAs` |
| IsKernelError | `ExampleIsKernelError` |
| Foreign StatusCode() compat | `ExampleFrom_foreignStatusCode`, `ExampleHTTPStatusOf_foreignError` |
| Foreign ErrorCode() compat | `ExampleCodeOf_foreignError` |
| Code validation | `ExampleIsValidCode`, `ExampleNormalizeCode` |
| nil safety | `ExampleCodeOf_nil`, `ExampleHTTPStatusOf_nil`, `ExampleFields_nil` |
| Context integration | `ExampleWithContext` |

### Runnable HTTP example

For a complete HTTP server with 7 error scenarios, see `examples/errorx-http/`.
Run it with `go run ./examples/errorx-http` and test with curl commands in its README.

## Before finishing a task

Run these (CI will reject if any fails):

```bash
go test ./...
go vet ./...
./scripts/check-errorx-usage.sh    # grep for forbidden patterns
golangci-lint run                   # depguard enforces import rules
```

## When unsure

- Error handling? �?`docs/ai/errorx.md` + `errorx/README.md` + Example table above
- Logging? �?`logx/README.md` + `docs/design/logx.md`
- New module? �?`docs/process/module-acceptance.md`
- Anything else? �?`AGENTS.md` + `docs/README.md`

## configx module rules

Use `github.com/aisphereio/kernel/configx` for all runtime configuration. Do not import the old `github.com/aisphereio/kernel/config` path and do not read `os.Getenv` in business code. Read [configx/README.md](configx/README.md) and [docs/ai/configx.md](docs/ai/configx.md).

### Example configx lookup

| Scenario | Example | File |
|---|---|---|
| Create config | `ExampleNew` | `configx/example_test.go` |
| Read typed value | `ExampleGet` | `configx/example_test.go` |
| Required startup value | `ExampleMustGet` | `configx/example_test.go` |
| Optional default | `ExampleGetOrDefault` | `configx/example_test.go` |
| Combine sources | `ExampleWithSource` | `configx/example_test.go` |
| Scan struct | `ExampleConfig_Scan` | `configx/example_test.go` |
| Watch key | `ExampleConfig_Watch` | `configx/example_test.go` |
| Layered file/env override | `Example_businessLayeredFileAndEnv` | `configx/example_business_test.go` |
| Validate required config | `Example_businessValidateRequiredConfig` | `configx/example_business_test.go` |

<!-- OPENWIKI:START -->

## OpenWiki

This repository uses OpenWiki for recurring code documentation. Start with `openwiki/quickstart.md`, then follow its links to architecture, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

The scheduled OpenWiki GitHub Actions workflow refreshes the repository wiki. Do not hand-edit generated OpenWiki pages unless explicitly asked; prefer updating source code/docs and letting OpenWiki regenerate.

<!-- OPENWIKI:END -->
