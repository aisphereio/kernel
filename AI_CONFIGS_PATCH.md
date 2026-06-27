# logx Patch for AI Tool Configs

> These are PATCH files. They show what to ADD to existing CLAUDE.md / AGENTS.md
> / .github/copilot-instructions.md / docs/README.md / README.md (root).
> They do NOT overwrite — they add logx sections alongside existing errorx sections.

---

## 1. CLAUDE.md — add logx section

Add this section to CLAUDE.md (alongside the existing errorx section, before "## 📚 Examples"):

```markdown
### 2. Logging — use logx, never raw log

In ALL business code (handler/service/repository/worker):

```go
// ❌ FORBIDDEN
log.Printf("...")
fmt.Println("...")
fmt.Printf("...")
slog.Info("...")
slog.Default().Info("...")
zap.L().Info("...")
logrus.Info("...")

// ✅ REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created",
    logx.String("skill_id", id),
    logx.String("operation", "aihub.skill.create"),
)

// ✅ At handler boundary, inject request-scoped fields:
ctx = logx.Inject(ctx, logger,
    logx.String("request_id", reqID),
    logx.String("trace_id", traceID),
    logx.String("subject_id", userID),
)
// All downstream logx.FromContext(ctx) calls auto-include these fields.
```

API cheatsheet:

| Scenario | API |
|---|---|
| Request-scoped log | `logx.FromContext(ctx).Info(...)` |
| Service startup | `logx.New(cfg)` + `logger.Info(...)` |
| Error log | `logger.Error("...", logx.Err(err))` |
| Discard logs | `logx.Noop()` |
| HTTP access log | `logx.HTTPAccessLog(logger, cfg)` middleware |
| Upstream call log | `logx.LogExternalCall(logger, call)` |
| Test logger | `logx.NewTestLogger(t).AssertLogged(...)` |

Field constructors: `logx.String` / `logx.Int` / `logx.Bool` / `logx.Duration` /
`logx.Time` / `logx.Any` / `logx.Event` / `logx.Err` / `logx.Group`.

Redaction auto-applies to: password / token / secret / authorization / cookie /
credential / private_key / api_key / ak / sk. Never log credentials directly —
redaction is a safety net, not a license.
```

Also add to the "Examples — find by scenario" table:

```markdown
### logx examples (45+)

| Scenario | Example function | File |
|---|---|---|
| Request-scoped log | `ExampleFromContext`, `ExampleInject` | `logx/example_test.go` |
| Service startup | `ExampleNew` | `logx/example_test.go` |
| Error log | `ExampleErr`, `ExampleLogError` | `logx/example_test.go` |
| HTTP access log | `ExampleLogAccess` | `logx/example_test.go` |
| Upstream call | `ExampleLogExternalCall` | `logx/example_test.go` |
| Audit hint | `ExampleLogAuditHint` | `logx/example_test.go` |
| Test logger | `ExampleNewTestLogger` | `logx/example_test.go` |
| Level control | `ExampleLevelController` | `logx/example_test.go` |
| Persistent fields | `ExampleLogger_With`, `ExampleLogger_Named` | `logx/example_test.go` |
| nil-safe FromContext | `ExampleFromContext_nil` | `logx/example_test.go` |

### logx business scenarios (10)

| Scenario | Example | Layer |
|---|---|---|
| Repository query log | `ExampleBusiness_repositoryLog` | repository |
| Service operation log | `ExampleBusiness_serviceLog` | service |
| Upstream call log | `ExampleBusiness_upstreamCall` | service |
| Worker log with redaction | `ExampleBusiness_workerLog` | worker |
| HTTP access log | `ExampleBusiness_httpAccess` | handler |
| Error log with errorx | `ExampleBusiness_errorLog` | service |
| Audit hint | `ExampleBusiness_auditHint` | service |
| Test logger assertion | `ExampleBusiness_testLogger` | test |
| Request-scoped fields | `ExampleBusiness_requestScoped` | handler |
| Sampling noisy logs | `ExampleBusiness_sampling` | cross-layer |
```

---

## 2. .github/copilot-instructions.md — add logx section

Add this section (alongside existing errorx section):

```markdown
### 2. Logging — use logx

In ALL business code:

```go
// ❌ FORBIDDEN
log.Printf(...)
fmt.Println(...)
slog.Info(...)
zap.L().Info(...)

// ✅ REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created",
    logx.String("skill_id", id),
    logx.String("operation", "aihub.skill.create"),
)
```

API cheatsheet:

- `logx.New(cfg)` — bootstrap logger (returns Logger + LevelController)
- `logx.FromContext(ctx)` — request-scoped logger (Noop if none)
- `logx.Inject(ctx, logger, fields...)` — attach request fields
- `logx.Noop()` — discard logs (tests, disabled features)
- `logx.Err(err)` — auto-extract errorx fields (no cycle)
- `logx.HTTPAccessLog(logger, cfg)` — HTTP access log middleware
- `logx.LogExternalCall(logger, call)` — upstream API call log
- `logx.LogAuditHint(logger, hint)` — audit breadcrumb
- `logx.NewTestLogger(t)` — test logger with AssertLogged

Field constructors: `logx.String` / `logx.Int` / `logx.Bool` / `logx.Duration` /
`logx.Time` / `logx.Any` / `logx.Event` / `logx.Err` / `logx.Group`.

## 📚 logx Examples — find by scenario

### Quick lookup

| Scenario | Example | File |
|---|---|---|
| Request-scoped log | `ExampleFromContext` | `logx/example_test.go` |
| Service startup | `ExampleNew` | `logx/example_test.go` |
| Error log | `ExampleErr` | `logx/example_test.go` |
| HTTP access log | `ExampleLogAccess` | `logx/example_test.go` |
| Upstream call | `ExampleLogExternalCall` | `logx/example_test.go` |
| Audit hint | `ExampleLogAuditHint` | `logx/example_test.go` |
| Test logger | `ExampleNewTestLogger` | `logx/example_test.go` |
| Level control | `ExampleLevelController` | `logx/example_test.go` |

### Complete business scenarios (in `logx/example_business_test.go`)

| Scenario | Example |
|---|---|
| Repository log | `ExampleBusiness_repositoryLog` |
| Service log | `ExampleBusiness_serviceLog` |
| Upstream call | `ExampleBusiness_upstreamCall` |
| Worker log | `ExampleBusiness_workerLog` |
| HTTP access log | `ExampleBusiness_httpAccess` |
| Error log | `ExampleBusiness_errorLog` |
| Audit hint | `ExampleBusiness_auditHint` |
| Test logger | `ExampleBusiness_testLogger` |
| Request-scoped | `ExampleBusiness_requestScoped` |
| Sampling | `ExampleBusiness_sampling` |

### Runnable examples

```bash
go run ./examples/logx-basic      # minimal: New + Info + Sync
go run ./examples/logx-http       # HTTP server with access log + recovery
```
```

---

## 3. AGENTS.md — add logx to hard rules + Example table

Add to the "Hard rules" section (after errorx Rule 1):

```markdown
### Rule 2: Logging — use logx, never raw log

In ALL business code (handler/service/repository/worker):

```go
// ❌ FORBIDDEN
log.Printf("...")
fmt.Println("...")
fmt.Printf("...")
slog.Info("...")
slog.Default().Info("...")
zap.L().Info("...")
logrus.Info("...")

// ✅ REQUIRED
logger := logx.FromContext(ctx)
logger.Info("skill created",
    logx.String("skill_id", id),
    logx.String("operation", "aihub.skill.create"),
)
```

API cheatsheet:

| Scenario | API |
|---|---|
| Request-scoped log | `logx.FromContext(ctx).Info(...)` |
| Service startup | `logx.New(cfg)` + `logger.Info(...)` |
| Error log | `logger.Error("...", logx.Err(err))` |
| HTTP access log | `logx.HTTPAccessLog(logger, cfg)` middleware |
| Upstream call log | `logx.LogExternalCall(logger, call)` |
| Audit hint | `logx.LogAuditHint(logger, hint)` |
| Test logger | `logx.NewTestLogger(t).AssertLogged(...)` |
| Dynamic level | `LevelController.SetLevel("debug")` |
| Discard logs | `logx.Noop()` |

Field constructors: `logx.String` / `logx.Int` / `logx.Int64` / `logx.Uint64` /
`logx.Bool` / `logx.Float64` / `logx.Duration` / `logx.Time` / `logx.Any` /
`logx.Event` / `logx.Err` / `logx.Group`.

Redaction auto-applies to: password / token / secret / authorization / cookie /
credential / private_key / api_key / ak / sk. Never log credentials directly.
```

Add to the "Examples — find by scenario" section (after errorx examples):

```markdown
### logx examples

#### Quick lookup: "I want to..."

| Your scenario | Example function | File |
|---|---|---|
| Request-scoped log | `ExampleFromContext`, `ExampleInject` | `logx/example_test.go` |
| Service startup | `ExampleNew` | `logx/example_test.go` |
| Discard logs | `ExampleNoop` | `logx/example_test.go` |
| Error log | `ExampleErr`, `ExampleLogError` | `logx/example_test.go` |
| HTTP access log | `ExampleLogAccess`, `ExampleHTTPAccessLog` | `logx/example_test.go` |
| Upstream call log | `ExampleLogExternalCall` | `logx/example_test.go` |
| Audit hint | `ExampleLogAuditHint` | `logx/example_test.go` |
| Test logger | `ExampleNewTestLogger` | `logx/example_test.go` |
| Level control | `ExampleLevelController` | `logx/example_test.go` |
| Persistent fields | `ExampleLogger_With`, `ExampleLogger_Named` | `logx/example_test.go` |
| Filter / drop | `ExampleFilterKey`, `ExampleDropEvents` | `logx/example_test.go` |
| nil-safe FromContext | `ExampleFromContext_nil` | `logx/example_test.go` |

#### Business scenario examples

| Scenario | Example | Layer |
|---|---|---|
| Repository query log | `ExampleBusiness_repositoryLog` | repository |
| Service operation log | `ExampleBusiness_serviceLog` | service |
| Upstream call log | `ExampleBusiness_upstreamCall` | service |
| Worker log with redaction | `ExampleBusiness_workerLog` | worker |
| HTTP access log | `ExampleBusiness_httpAccess` | handler |
| Error log with errorx | `ExampleBusiness_errorLog` | service |
| Audit hint | `ExampleBusiness_auditHint` | service |
| Test logger assertion | `ExampleBusiness_testLogger` | test |
| Request-scoped fields | `ExampleBusiness_requestScoped` | handler |
| Sampling noisy logs | `ExampleBusiness_sampling` | cross-layer |

#### Runnable examples

```bash
go run ./examples/logx-basic      # minimal: New + Info + Sync
go run ./examples/logx-http       # HTTP server with access log + recovery
```
```

---

## 4. docs/README.md — add logx to navigation

Add logx to all relevant sections of `docs/README.md`:

In "Quick start" section, add:

```markdown
| `../logx/README.md` | logx 单一入口指南 | 写任何日志前 |
```

In "AI coding guides" section, add:

```markdown
| `ai/logx.md` | logx 完整 AI 指南：速查 + 10 食谱 + 禁用模式 + 完整 handler 示例 |
```

In "Deep specs" section, add:

```markdown
| `design/logx.md` | logx 设计规范 | — |
| `contracts/logx.md` | logx 不可破坏契约 + 验收命令 | ~150 |
```

In "Acceptance & ops" section, add:

```markdown
| `process/logx-acceptance-checklist.md` | logx 静态/单元/集成验收清单 |
```

In "Runnable examples" (if section exists), add:

```markdown
| `../examples/logx-basic/` | 最小示例：New + Info + Sync |
| `../examples/logx-http/` | HTTP access log + recovery 完整示例 |
```

---

## 5. README.md (root) — add logx to module status table

Update the module status table in root README.md:

```markdown
| `logx/` | ✅ stable | slog-based structured logging | [logx/README.md](logx/README.md) |
```

(Replace the existing `logx/` row if it has a different status.)

Add to "Quick example: logx" section (already exists, just verify it points to
the new README).

Add to "Documentation map" section, in "Quick start" list:

```markdown
├── logx/README.md             ← logging
```

In "AI coding guides":

```markdown
└── docs/ai/logx.md          ← complete logx recipe (10 scenarios + forbidden patterns)
```

In "Deep specs":

```markdown
└── docs/design/logx.md        ← logx design
```

In "Runnable examples":

```markdown
├── examples/logx-basic/     ← minimal: New + Info + Sync
└── examples/logx-http/      ← full HTTP server with access log + recovery
```

---

## 6. .golangci.yml — add logx to depguard (if not already covered)

The existing `.golangci.yml` should already forbid `log` / `log/slog` / `zap` /
`logrus` in business code via depguard. Verify these rules exist:

```yaml
business-no-raw-log:
  list-mode: lax
  files:
    - "**/handler/**/*.go"
    - "**/service/**/*.go"
    - "**/repository/**/*.go"
    - "**/repo/**/*.go"
    - "**/worker/**/*.go"
    - "**/controller/**/*.go"
    - "!**/*_test.go"
  deny:
    - pkg: "log"
      desc: "use github.com/aisphereio/kernel/logx instead; log.Printf loses structured fields"
    - pkg: "log/slog"
      desc: "use github.com/aisphereio/kernel/logx instead; direct slog bypasses Kernel logging contract"
    - pkg: "go.uber.org/zap"
      desc: "use github.com/aisphereio/kernel/logx instead; direct zap bypasses Kernel logging contract"
    - pkg: "github.com/sirupsen/logrus"
      desc: "use github.com/aisphereio/kernel/logx instead"
```

If these rules are missing, add them. If they exist, no change needed.
