# Aisphere Kernel

Aisphere Kernel is a breaking-rewrite microservice foundation for Aisphere projects.
It is AI-native: every module ships a single-entry README, a Go-doc-rich `doc.go`,
runnable examples, and an AI coding recipe.

> **New here?** Read [docs/README.md](docs/README.md) for the doc map, then
> [errorx/README.md](errorx/README.md) and [logx/README.md](logx/README.md) for
> the two foundational modules. Run `go run ./examples/errorx-basic` to see
> errorx in action.

---

## What's inside

| Module | Status | One-line | Entry doc |
|---|---|---|---|
| `errorx/` | ✅ stable | Unified error semantics, stdlib-only | [errorx/README.md](errorx/README.md) |
| `logx/` | ✅ stable | slog-based structured logging | [logx/README.md](logx/README.md) |
| `transport/http/` | 🚧 in progress | HTTP server / client / middleware | — |
| `transport/grpc/` | 🚧 in progress | gRPC server / client + errorx adapter | — |
| `config/` | ✅ stable | Multi-source config (file/env/remote) | [config/README.md](config/README.md) |
| `middleware/` | 🚧 in progress | recovery / ratelimit / logging / selector | — |
| `selector/` | ✅ stable | Load balancing (wrr/p2c/random/ewma) | — |
| `registry/` | ✅ stable | Service discovery | [registry/README.md](registry/README.md) |
| `encoding/` | ✅ stable | json / yaml / xml / form / proto | [encoding/README.md](encoding/README.md) |
| `cmd/kernel/` | ✅ stable | Project scaffolding CLI | — |

Status legend: ✅ stable / 🚧 in progress / ⬜ planned

---

## Core design

- `errorx` is the only runtime/business error semantics package.
- The legacy root package `github.com/aisphereio/kernel/errors` has been removed.
- HTTP/gRPC/middleware/selector/contrib adapters now produce or consume `errorx`.
- Proto error-code generation is retained, but it now generates `errorx` helpers.
- Business code must not return raw `errors.New` or `fmt.Errorf` as API/business errors.
- Transport layers expose stable `error_code` values instead of Kratos-style `code + reason` status objects.

---

## Quick example: errorx

```go
return errorx.NotFound(
    "AIHUB_SKILL_NOT_FOUND",
    "skill not found",
    errorx.WithMetadata("skill_id", skillID),
    errorx.WithPublicMetadata("resource", "skill"),
)
```

HTTP response shape (produced by `httpx` middleware, see `examples/errorx-http`):

```json
{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "skill not found",
  "metadata": { "resource": "skill" }
}
```

HTTP status stays in the HTTP status code. The JSON `code` field is the stable business error code.

See [errorx/README.md](errorx/README.md) for the full guide and [docs/ai/errorx.md](docs/ai/errorx.md) for the AI coding recipe.

---

## Quick example: logx

```go
cfg := logx.DefaultConfig("dev")
cfg.ServiceName = "aihub"
logger, _, err := logx.New(cfg)
if err != nil {
    panic(err)
}

logger.Info("service started", logx.String("addr", ":8000"))
```

See [logx/README.md](logx/README.md) and [docs/design/logx.md](docs/design/logx.md).

---

## Proto error generation

The proto generation capability is not deleted. It is converted to produce `errorx` helpers.

Retained compatibility pieces:

- `third_party/errors/errors.proto`
- `cmd/protoc-gen-go-errors/`
- `--go-errors_out=paths=source_relative:.`

The name is kept for proto compatibility with existing Kratos-style annotations, but the generated Go code imports `github.com/aisphereio/kernel/errorx` and returns `*errorx.Error`.

Example proto:

```proto
import "errors/errors.proto";

enum SkillErrorReason {
  option (errors.default_code) = 500;

  SKILL_NOT_FOUND = 0 [(errors.code) = 404];
  SKILL_FORBIDDEN = 1 [(errors.code) = 403];
}
```

Generated helpers look like:

```go
func IsSkillNotFound(err error) bool
func NewSkillNotFound(message string, opts ...errorx.Option) *errorx.Error
func ErrorSkillNotFound(format string, args ...interface{}) *errorx.Error
```

So old proto contracts can remain, while runtime errors are standardized on `errorx`.

---

## Documentation map

```text
Quick start (everyone)
├── README.md                  ← you are here
├── AGENTS.md                  ← AI work rules
├── errorx/README.md           ← error handling
├── logx/README.md             ← logging
└── docs/README.md             ← full doc index

AI coding guides
└── docs/ai/errorx.md          ← complete errorx recipe (10 scenarios + forbidden patterns)

Deep specs (architects / PR review)
├── docs/design/errorx.md      ← 1250-line design spec
├── docs/contracts/errorx.md   ← unbreakable contract
└── docs/design/logx.md        ← logx design

Acceptance & ops (CI/CD)
├── docs/process/errorx-acceptance-checklist.md
├── docs/process/errorx-test-report.md
└── docs/process/module-acceptance.md

Runnable examples
├── examples/errorx-basic/     ← minimal: Wrap + Inspect
└── examples/errorx-http/      ← full HTTP handler with 7 error scenarios
```

See [docs/README.md](docs/README.md) for the complete index with recommended reading paths.

---

## What was removed

The following old runtime package is intentionally gone:

- `errors/`
- imports of `github.com/aisphereio/kernel/errors`

The proto generator and proto extension directory are retained because they are part of the contract-generation toolchain, not the old runtime error package.

---

## Verify

Use Go 1.25+ locally. Linux/macOS can use Make directly:

```bash
make tools
make test-errorx
make verify-errorx
make test
make vet
```

Windows is a first-class path. You can either keep using Make:

```powershell
make tools
make test-cmd
make verify-errorx
```

or run the cmd wrappers directly, which also avoids PowerShell script-signing issues:

```bat
scripts\tools.cmd
scripts\test-cmd.cmd
scripts\verify-errorx.cmd
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md).

For new modules, follow the documentation pattern established by `errorx/`:

1. **Single entry README** in the package directory (e.g. `httpx/README.md`)
2. **Rich `doc.go`** so `go doc <pkg>` gives useful output
3. **`example_test.go`** with `ExampleXxx` for every public constructor
4. **One AI guide** at `docs/ai/<module>.md` (recipes + forbidden patterns in one file)
5. **Design spec** at `docs/design/<module>.md` for deep reference
6. **Contract** at `docs/contracts/<module>.md` for unbreakable behaviors
7. **Runnable example** at `examples/<module>-*/`
8. **Index entry** in `docs/README.md` and root `README.md`

See [docs/process/module-acceptance.md](docs/process/module-acceptance.md) for the full checklist.
