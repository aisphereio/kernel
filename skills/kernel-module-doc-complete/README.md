# kernel-module-doc-complete

A skill that brings a newly-finished Aisphere Kernel module up to the
documentation, example, and AI-guidance standard set by `errorx/`.

## What this skill does

When a kernel module's Go code is functionally complete, this skill verifies
and fills in the surrounding docs / examples / AI-integration so that:

1. AI tools (Claude Code, Cursor, Copilot) automatically discover and use the module
2. Developers can learn the module from a single README entry point
3. Every public API has a runnable `ExampleXxx` for `go doc` and AI to copy
4. Docs are wired together (README → package README → Examples → runnable demos)
5. CI enforces usage rules (e.g. "use errorx, never raw errors")

## When to trigger

Trigger this skill when the user says:

- "errorx is done, check if docs are complete"
- "logx is finished, wire up the AI configs"
- "httpx is ready, follow the errorx pattern"
- "is the dbx documentation complete?"
- "make AI use the cachex module"
- "what's missing before authx can ship"
- "check <module> against the kernel module standard"
- "kernel module acceptance for <module>"

## The 5-dimension standard

```text
Dimension 1: Single-entry docs        README.md + doc.go + design doc
Dimension 2: AI tool configs          CLAUDE.md + .cursor/rules + copilot + AGENTS.md + docs/ai/
Dimension 3: Example coverage         example_test.go + example_business_test.go + examples/
Dimension 4: Doc wiring               root README + docs/README.md + cross-links
Dimension 5: Acceptance checks        contracts + checklist + usage script + CI workflow
```

A module is "doc-complete" only when ALL applicable dimensions pass.

## Quick start

### 1. Check a module's current state

```bash
# From the kernel repo root
./scripts/check-module-docs.sh errorx
./scripts/check-module-docs.sh logx
./scripts/check-module-docs.sh httpx
```

Output shows pass/fail per dimension, with specific gaps listed.

### 2. Read the reference docs

```text
references/completeness-checklist.md   ← 5-dimension pass/fail criteria
references/doc-templates.md            ← README / doc.go / AI config / example templates
references/example-coverage.md         ← Example coverage formula + 10 standard scenarios
```

### 3. Generate missing files

Use the templates in `references/doc-templates.md` to generate:

- `<module>/README.md` (single entry, 200+ lines)
- `<module>/doc.go` (rich Go doc, 80+ lines)
- `<module>/example_test.go` (Example per public API, 200+ lines)
- `<module>/example_business_test.go` (10 business scenarios)
- `docs/ai/<module>.md` (single AI recipe, 200+ lines)
- `.cursor/rules/<module>.mdc` (glob-triggered rule)
- `examples/<module>-basic/` and `examples/<module>-http/`
- Updates to `CLAUDE.md`, `AGENTS.md`, `.github/copilot-instructions.md`
- Updates to root `README.md` and `docs/README.md`

### 4. Re-verify

```bash
./scripts/check-module-docs.sh <module>
# Should print "✓ <module> is DOC-COMPLETE"
```

## File structure

```text
kernel-module-doc-complete/
├── SKILL.md                              ← main skill instructions (<500 lines)
├── README.md                             ← this file
├── references/
│   ├── completeness-checklist.md         ← Dimension 1-5 pass/fail criteria
│   ├── doc-templates.md                  ← 14 templates (README/doc.go/AI configs/examples/...)
│   └── example-coverage.md               ← Coverage formula + 10 standard scenarios
└── scripts/
    └── check-module-docs.sh              ← automated gap check (Dimension 1-5)
```

## How the check script works

`scripts/check-module-docs.sh <module>` performs these checks:

### Dimension 1: Single-entry docs

- `<module>/README.md` exists, 200+ lines
- `<module>/doc.go` exists, 80+ lines
- README contains: quickstart section, doc map, Example mentions
- `docs/design/<module>.md` exists for major modules (errorx/logx/httpx/dbx/...), 500+ lines

### Dimension 2: AI tool configs

- `CLAUDE.md` mentions module
- `AGENTS.md` mentions module
- `.github/copilot-instructions.md` mentions module
- `.cursor/rules/<module>.mdc` exists
- `docs/ai/<module>.md` exists as a SINGLE file (not split into quickstart/recipes/forbidden)
- AI configs have Example lookup tables

### Dimension 3: Example coverage

- `<module>/example_test.go` exists, 200+ lines
- Example count vs public API count (target: 90%+ coverage)
- `<module>/example_business_test.go` exists with 10+ business scenarios
- `examples/<module>-basic/` exists with main.go + README
- `examples/<module>-http/` exists for HTTP-touching modules

### Dimension 4: Doc wiring

- Root `README.md` mentions module
- `docs/README.md` mentions module
- Module README links to design/contracts/ai docs
- AI config files link to module README and AI guide

### Dimension 5: Acceptance checks

- `docs/contracts/<module>.md` exists for major modules
- `docs/process/<module>-acceptance-checklist.md` exists for major modules
- `scripts/check-<module>-usage.sh` exists IF module has usage rules
- `.github/workflows/<module>-enforcement.yml` exists (optional)

Minor modules (retry / selectors / encoding) skip Dimension 5 — it's marked N/A.

## Example output

```
$ ./scripts/check-module-docs.sh errorx
━━━ Dimension 1: Single-entry docs ━━━
  ✓ errorx/README.md exists
  ✓ errorx/README.md has 280 lines (>= 200)
  ✓ errorx/doc.go exists
  ✓ errorx/doc.go has 120 lines (>= 80)
  ✓ README has quickstart section
  ✓ README has doc map
  ✓ README mentions Examples
  ✓ docs/design/errorx.md exists
  ✓ docs/design/errorx.md has 1253 lines (>= 500)

━━━ Dimension 2: AI tool configs ━━━
  ✓ CLAUDE.md mentions errorx
  ✓ AGENTS.md mentions errorx
  ✓ copilot-instructions mentions errorx
  ✓ .cursor/rules/errorx.mdc exists
  ✓ docs/ai/errorx.md exists
  ✓ AI docs merged into single file

━━━ Dimension 3: Example coverage ━━━
  ✓ errorx/example_test.go exists
  Public APIs: 50 (constructors=11, options=14, inspect=25)
  Examples: 55 (test=45, business=10)
  Coverage ratio: 90%
  ✓ Example coverage >= 90%
  ✓ 10 business scenarios (>= 10)
  ✓ errorx/example_test.go has 280 lines (>= 200)
  ✓ examples/errorx-basic/main.go exists
  ✓ examples/errorx-http/main.go exists

━━━ Dimension 4: Doc wiring ━━━
  ✓ root README.md mentions errorx
  ✓ docs/README.md mentions errorx
  ✓ README links to design doc
  ✓ README links to contracts
  ✓ README links to AI guide
  ✓ CLAUDE.md links to errorx entry docs

━━━ Dimension 5: Acceptance checks ━━━
  ✓ docs/contracts/errorx.md exists
  ✓ acceptance checklist exists
  ✓ scripts/check-errorx-usage.sh exists
  ⚠ .github/workflows/errorx-enforcement.yml missing (optional)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Module: errorx
  Pass: 25
  Warn: 1 (optional items missing)
  Fail: 0

✓ errorx is DOC-COMPLETE (all required dimensions pass, 1 optional item missing)
```

## Using the skill with AI tools

### Claude Code

The skill's `SKILL.md` is loaded automatically when Claude Code detects the
user is working on kernel module documentation. Just say:

> "logx is done, check if docs are complete"

Claude Code will:
1. Run `./scripts/check-module-docs.sh logx`
2. Read the gap report
3. Read `references/completeness-checklist.md` and `references/doc-templates.md`
4. Generate the missing files
5. Re-run the check to verify

### Cursor / Copilot / other AI tools

The skill works with any AI tool that can read markdown files. Point the AI
at this directory and ask it to follow `SKILL.md`.

## Module-specific notes

### errorx (reference implementation)

Already doc-complete (after applying the patches from prior conversations).
Use as the gold standard — when in doubt, check how errorx does it.

### logx

Same 5 dimensions. Key differences:
- AI rule: forbid `log.Printf` / `fmt.Println` / direct `slog` / direct `zap`
- Example scenarios: structured logging, log levels, filtering, context extraction, redaction, sampling
- `.cursor/rules/logx.mdc` globs: all `*.go` files (logging is everywhere)

### httpx (when created)

- AI rule: forbid `net/http` server setup in business code
- Example scenarios: handler registration, middleware chain, error response, streaming (SSE), WebSocket, graceful shutdown
- `.cursor/rules/httpx.mdc` globs: `**/handler/**`, `**/server/**`

### dbx (when created)

- AI rule: forbid `database/sql` direct usage in business code
- Example scenarios: connection pool, transaction, query, scan, migration
- `.cursor/rules/dbx.mdc` globs: `**/repository/**`, `**/repo/**`, `**/data/**`

### Minor modules (retry / selectors / encoding)

Skip Dimension 5. Just do Dimensions 1-4. Use the 80/20 rule: 80% of the
value comes from single-entry README + Example coverage + AI config mention.

## Installation

Copy this directory to your kernel repo (or a sibling location) and run the
check script:

```bash
# Option A: copy into kernel repo
cp -r kernel-module-doc-complete /path/to/kernel/.skills/

# Option B: keep external, reference by path
/path/to/kernel-module-doc-complete/scripts/check-module-docs.sh errorx
```

For AI tools, the `SKILL.md` is auto-discovered if placed in the project's
skill directory. See your AI tool's docs for skill installation.

## License

Same as Aisphere Kernel.
