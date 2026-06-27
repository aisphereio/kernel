# Aisphere Kernel Agent Guide

> **You are working inside Aisphere Kernel** — an AI-native Go application kernel.
> This file is loaded by all AI coding tools (Claude Code via `CLAUDE.md`,
> Cursor via `.cursor/rules/`, Copilot via `.github/copilot-instructions.md`).

---

## ⚠️ Prime directive (read first)

**Do not invent infrastructure.** Use Kernel APIs only.

Before writing ANY business code (handler/service/repository/worker), you MUST:

1. **Look up the matching Example** in the table below — Examples are runnable
   and show exact output. Copy the pattern.
2. Read `docs/ai/errorx.md` for the complete AI recipe (10 scenarios).
3. Read `errorx/README.md` if Example doesn't answer your question.

If you skip step 1, your code will fail CI (see "Before finishing" below).

---

## 🚫 Hard rules (violations will fail CI)

### Rule 1: Error handling — use errorx, never raw errors

In `handler/`, `service/`, `repository/`, `worker/` code (NOT in `*_test.go`):

```go
// ❌ FORBIDDEN
return errors.New("skill not found")
return fmt.Errorf("create failed: %w", err)
return nil, err              // raw error passthrough
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

Error code format: `{DOMAIN}_{RESOURCE}_{REASON}` (uppercase snake). Dynamic values in `WithMetadata`, NEVER in the error code.

### Rule 2: Logging — use logx, never raw log

```go
// ❌ FORBIDDEN in business code
log.Printf("...")
fmt.Println("...")
slog.Info("...")
zap.L().Info("...")

// ✅ REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created", logx.String("skill_id", id))
```

### Rule 3: Forbidden imports in business code

```text
database/sql          → use kernel dbx
net/http (server)     → use kernel httpx
go-redis              → use kernel cache
minio SDK             → use kernel objectstore
casdoor SDK           → use kernel authn
casbin enforcer       → use kernel authz
os.Getenv             → use kernel config
```

---

## 📚 Examples — find by scenario BEFORE writing code

**Before writing errorx code, look up the matching Example.** All examples are
runnable (`go test ./errorx -run=Example -v`) and show exact output. You can
also see them inline with `go doc errorx.Xxx`.

### Quick lookup: "I want to..."

| Your scenario | Example function | File |
|---|---|---|
| Return validation error | `ExampleBadRequest` | `example_test.go` |
| Return not-authenticated | `ExampleUnauthorized` | `example_test.go` |
| Return no-permission | `ExampleForbidden` | `example_test.go` |
| Return resource-not-found | `ExampleNotFound` | `example_test.go` |
| Return already-exists | `ExampleConflict` | `example_test.go` |
| Return request-timeout | `ExampleRequestTimeout` | `example_test.go` |
| Return rate-limited | `ExampleTooManyRequests` | `example_test.go` |
| Return client-closed | `ExampleClientClosed` | `example_test.go` |
| Return internal-error | `ExampleInternal` | `example_test.go` |
| Return upstream-unavailable | `ExampleUnavailable` | `example_test.go` |
| Return upstream-timeout | `ExampleTimeout` | `example_test.go` |
| Wrap underlying error | `ExampleWrap` | `example_test.go` |
| Wrap (nil-safe) | `ExampleWrap_nil` | `example_test.go` |
| Wrap through fmt.Errorf | `ExampleWrap_seesThroughFmtErrorf` | `example_test.go` |
| Convert foreign error | `ExampleFrom` | `example_test.go` |
| Convert with options | `ExampleFrom_withOptions` | `example_test.go` |
| Convert GoFr-style error | `ExampleFrom_foreignStatusCode` | `example_test.go` |
| Add internal metadata | `ExampleWithMetadata` | `example_test.go` |
| Add multiple metadata | `ExampleWithMetadataMap` | `example_test.go` |
| Add public metadata | `ExampleWithPublicMetadata` | `example_test.go` |
| Add multiple public metadata | `ExampleWithPublicMetadataMap` | `example_test.go` |
| Override HTTP status | `ExampleWithHTTPStatus` | `example_test.go` |
| Override gRPC code | `ExampleWithGRPCCode` | `example_test.go` |
| Override retryable | `ExampleWithRetryable` | `example_test.go` |
| Preserve cause | `ExampleWithCause` | `example_test.go` |
| Add request ID | `ExampleWithRequestID` | `example_test.go` |
| Add trace ID | `ExampleWithTraceID` | `example_test.go` |
| Override category | `ExampleWithCategory` | `example_test.go` |
| Override severity | `ExampleWithSeverity` | `example_test.go` |
| Capture call stack | `ExampleWithStack` | `example_test.go` |
| Extract error code | `ExampleCodeOf` | `example_test.go` |
| Extract message | `ExampleMessageOf` | `example_test.go` |
| Extract HTTP status | `ExampleHTTPStatusOf` | `example_test.go` |
| Extract gRPC code | `ExampleGRPCCodeOf` | `example_test.go` |
| Check retryable | `ExampleRetryableOf` | `example_test.go` |
| Get internal metadata | `ExampleMetadataOf` | `example_test.go` |
| Get redacted metadata | `ExampleSafeMetadataOf` | `example_test.go` |
| Get public metadata | `ExamplePublicMetadataOf` | `example_test.go` |
| Get request ID | `ExampleRequestIDOf` | `example_test.go` |
| Get trace ID | `ExampleTraceIDOf` | `example_test.go` |
| Get category | `ExampleCategoryOf` | `example_test.go` |
| Get severity | `ExampleSeverityOf` | `example_test.go` |
| Get call stack | `ExampleStackOf` | `example_test.go` |
| Match by HTTP status | `ExampleIsNotFound` etc. (11 predicates) | `example_test.go` |
| Match by exact code | `ExampleIsCode` | `example_test.go` |
| Extract from chain | `ExampleAs` | `example_test.go` |
| Check if kernel error | `ExampleIsKernelError` | `example_test.go` |
| Deep copy with override | `ExampleError_Clone` | `example_test.go` |
| Debug with %+v | `ExampleError_Format` | `example_test.go` |
| Validate error code format | `ExampleIsValidCode` | `example_test.go` |
| Handle nil error safely | `ExampleCodeOf_nil` etc. | `example_test.go` |

### Business scenario examples (complete flows in `example_business_test.go`)

| Scenario | Example | Layer |
|---|---|---|
| Repository DB error → errorx | `ExampleBusiness_repositoryLayer` | repository |
| Service validation + conflict | `ExampleBusiness_serviceLayer` | service |
| Upstream model timeout | `ExampleBusiness_upstreamTimeout` | service |
| Authz denied | `ExampleBusiness_authzDenied` | service |
| Worker retry decision | `ExampleBusiness_workerRetry` | worker |
| HTTP response JSON shape | `ExampleBusiness_httpResponse` | handler |
| Audit record from error | `ExampleBusiness_auditRecord` | auditx |
| Log entry with redaction | `ExampleBusiness_logEntry` | logx |
| Metrics labels (low cardinality) | `ExampleBusiness_metricsLabels` | metricsx |
| Multi-layer wrap (preserve chain) | `ExampleBusiness_multiLayerWrap` | cross-layer |

### Consumer examples (for logx/metricsx/auditx/workerx/httpx developers)

| Consumer | Example | What it shows |
|---|---|---|
| logx | `ExampleFields`, `ExampleBusiness_logEntry` | Extract log fields, auto-redact secrets |
| metricsx | `ExampleMetricsLabels`, `ExampleBusiness_metricsLabels` | Low-cardinality Prometheus labels |
| auditx | `ExampleBusiness_auditRecord` | Build audit record from error |
| workerx | `ExampleBusiness_workerRetry` | Decide retry vs fail |
| httpx | `ExampleBusiness_httpResponse` | JSON response shape |

### Runnable HTTP example

For a complete HTTP server with 7 error scenarios, see `examples/errorx-http/`:

```bash
go run ./examples/errorx-http
# then in another terminal:
curl -i 'http://localhost:18080/skills?id=missing'        # 404
curl -i -X POST http://localhost:18080/skills -d '{"name":""}'  # 400
curl -i 'http://localhost:18080/skills?id=boom'           # 500
```

---

## ✅ Allowed work

You may create or edit:

- business modules (handler / service / repository / DTO)
- tests (`*_test.go`)
- migrations (`migrations/*.sql`)
- examples (`examples/*`)

You may NOT edit (unless explicitly asked):

- `errorx/` internals
- `logx/` internals
- `transport/` internals
- `cmd/protoc-gen-go-*` internals

---

## 📐 Standard flow

```text
Handler     → bind input → call service → return response (or errorx)
Service     → validate rules → check authz → call repo → record audit → return errorx
Repository  → use kernel dbx → no HTTP/auth/audit logic → return errorx
```

### Handler example (copy from `cmd/kernel/internal/project/templates/handler_template.go`)

```go
func (h *SkillHandler) Create(ctx context.Context, req *CreateSkillRequest) (*Skill, error) {
    if req.Name == "" {
        return nil, errorx.BadRequest("AIHUB_SKILL_NAME_REQUIRED", "技能名称不能为空")
    }
    if err := h.rt.Access.Require(ctx, access.Check{
        Resource: "aihub:skill:*",
        Action:   "skill.create",
    }); err != nil {
        return nil, errorx.Forbidden("AIHUB_SKILL_CREATE_DENIED", "没有创建权限",
            errorx.WithCause(err),
        )
    }
    return h.svc.Create(ctx, &Skill{Name: req.Name})
}
```

### Repository example (see `ExampleBusiness_repositoryLayer` for full version)

```go
func (r *SkillRepo) Find(ctx context.Context, id string) (*Skill, error) {
    var row skillModel
    err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
            errorx.WithCause(err),
            errorx.WithMetadata("skill_id", id),
            errorx.WithPublicMetadata("resource", "skill"),
        )
    }
    if err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
            errorx.WithMessage("查询技能失败"),
            errorx.WithRetryable(true),
            errorx.WithMetadata("skill_id", id),
        )
    }
    return toBiz(&row), nil
}
```

---

## ✅ Before finishing a task

Run these — **CI will reject the PR if any fails**:

```bash
# 1. Unit tests (includes all Example tests)
go test ./...

# 2. Go vet
go vet ./...

# 3. errorx usage check (forbidden patterns grep)
./scripts/check-errorx-usage.sh

# 4. golangci-lint (depguard enforces import rules)
golangci-lint run

# 5. Contract check (optional, for module owners)
make verify-errorx
```

If any check fails, fix the violation before submitting the PR.

---

## 📚 When unsure

| Question | Read this |
|---|---|
| How to handle errors? | **Example table above** → `errorx/README.md` → `docs/ai/errorx.md` |
| How to log? | `logx/README.md` + `docs/design/logx.md` |
| How to add a new module? | `docs/process/module-acceptance.md` |
| What's the errorx contract? | `docs/contracts/errorx.md` |
| Complete errorx design? | `docs/design/errorx.md` |
| Full doc index? | `docs/README.md` |

---

## 🔧 AI tool integration

This project ships configs for all major AI coding tools:

| Tool | Config file | Loaded how |
|---|---|---|
| Claude Code | `CLAUDE.md` | Auto-loaded at session start |
| Cursor | `.cursor/rules/*.mdc` | Auto-loaded by glob match |
| GitHub Copilot | `.github/copilot-instructions.md` | Auto-loaded in Copilot Chat |
| aider | `.aider.conf.yml` (optional) | Manual `aider` invocation |
| All others | `AGENTS.md` (this file) | Convention; many tools read it |

All these files point to the same source of truth: the **Examples table above**
+ `docs/ai/errorx.md` + `errorx/README.md`.

---

## 📝 Summary

> **errorx is the only error package. logx is the only logger. Kernel APIs are the only infrastructure.**
> Before writing code: **look up the Example**. If you're about to write
> `errors.New`, `fmt.Errorf`, `log.Printf`, or `os.Getenv` in business code —
> STOP, find the matching Example, and copy the pattern.
