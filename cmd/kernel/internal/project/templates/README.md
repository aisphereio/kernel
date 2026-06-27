# Handler template — errorx reference implementation

This file (`handler_template.go`) is a **reference template** showing correct
errorx usage in a typical service / repository pair. AI coding tools should
use it as the starting point when generating new business handlers.

## How to use

### Option A: AI tools (Cursor / Claude Code / Copilot)

When you ask the AI to "create a new module called `agent`", it should:

1. Read this file (`cmd/kernel/internal/project/templates/handler_template.go`)
2. Copy the structure
3. Rename `Skill` → `Agent`, `AIHUB_SKILL_*` → `AIHUB_AGENT_*`
4. Fill in the business logic

The AI tool configs (`.cursor/rules/errorx.mdc`, `CLAUDE.md`,
`.github/copilot-instructions.md`) point here as the canonical example.

### Option B: Manual copy

```bash
cp cmd/kernel/internal/project/templates/handler_template.go \
   internal/service/agent.go
# then rename Skill → Agent, AIHUB_SKILL_* → AIHUB_AGENT_*
```

### Option C: `cmd/kernel new` (future)

When `cmd/kernel new` is extended to scaffold from local templates (currently
it pulls from a remote layout repo), this file will be the source template.

## What this template demonstrates

| Concept | Where in template |
|---|---|
| Domain error codes as constants | Section 1 |
| Domain object (DO) without proto/storage tags | Section 2 |
| Repository interface | Section 3 |
| Service with validation → authz → duplicate check → persist → log | Section 4 |
| `errorx.BadRequest` for validation | `Create` step 4.1 |
| `errorx.Forbidden` for authz | `Create` step 4.2 (commented) |
| `errorx.Conflict` for duplicate | `Create` step 4.3 |
| `errorx.Wrap` for repo failure (with retryable) | `Create` step 4.4 |
| `errorx.NotFound` for missing resource | `Get` |
| `WithMetadata` for internal context | throughout |
| `WithPublicMetadata` for client-safe context | throughout |
| `logx.FromContext(ctx).Info(...)` for logging | `Create` step 4.5 |

## What this template does NOT do (forbidden)

```go
// ❌ Never in business code:
return errors.New("skill not found")
return fmt.Errorf("create failed: %w", err)
return nil, err
panic(err)
log.Printf("created skill %s", id)
fmt.Println("debug")
slog.Info("...")
os.Getenv("DATABASE_URL")
```

## Lint enforcement

The patterns in this template are enforced by:

- `.golangci.yml` — depguard forbids `errors` / `fmt` / `log` / `slog` / `os`
  imports in `handler/`, `service/`, `repository/`, `worker/` code
- `scripts/check-errorx-usage.sh` — grep-based check for forbidden patterns
- `.github/workflows/errorx-enforcement.yml` — CI runs both checks on every PR

## Related docs

- `errorx/README.md` — single entry guide
- `docs/ai/errorx.md` — AI coding recipe (10 scenarios + complete handler)
- `errorx/example_test.go` — 18 Go Examples for all constructors
- `examples/errorx-http/` — full HTTP server example with 7 error scenarios
