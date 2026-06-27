---
name: kernel-module-doc-complete
description: >
  Complete the documentation, examples, and AI-guidance system for an Aisphere
  Kernel module (errorx, logx, httpx, dbx, cachex, authx, etc). Use this
  skill whenever the user says a kernel module is "done", "finished", "ready
  for review", asks to "check if the docs are complete", "fill in missing
  examples", "wire up the AI configs", "make AI use this module", "follow the
  errorx pattern for a module", or wants to verify a module meets the
  documentation and example completeness bar before release. Also trigger
  when the user mentions "kernel module acceptance", "module readiness
  check", or asks "what is missing for a module to be production-ready".
  This skill enforces the 5-dimension completeness standard established by
  errorx: single-entry docs, AI tool configs, example coverage, doc wiring,
  and acceptance checks.
---

# Kernel Module Doc-Complete

This skill brings a newly-finished Aisphere Kernel module up to the
documentation, example, and AI-guidance standard set by `errorx/`. Apply it
once the module's code is functionally complete — it does NOT review code
logic, only the surrounding docs/examples/AI-integration.

## When to use

Trigger this skill when ANY of these are true:

- A kernel module's Go code is functionally complete (compiles, tests pass)
- The user asks "is the <module> documentation complete?"
- The user asks "make AI use <module>" or "wire up AI configs for <module>"
- The user asks "follow the errorx pattern for <module>"
- The user asks "what's missing before <module> can ship"
- The user asks "check <module> against the kernel module standard"
- A new module is being added to `kernel/` and needs the full doc treatment

Do NOT trigger for:

- Code logic review (use a code reviewer instead)
- Bug fixing in module internals
- Performance optimization
- Adding new features to an already-documented module (just edit the existing docs)

## The 5-dimension completeness standard

Every kernel module must satisfy these 5 dimensions, modeled after `errorx/`:

```text
Dimension 1: Single-entry docs (the "find it" layer)
├── <module>/README.md          ← single entry, 15+ sections, ~280 lines
├── <module>/doc.go             ← rich Go doc, 100+ lines, go doc shows quickstart
└── docs/design/<module>.md     ← deep design spec (1000+ lines for major modules)

Dimension 2: AI tool configs (the "AI knows it exists" layer)
├── CLAUDE.md                   ← mention module in hard rules + Example table
├── .cursor/rules/<module>.mdc  ← glob-triggered rule for that module's files
├── .github/copilot-instructions.md  ← mention in module section
├── AGENTS.md                   ← module section + Example lookup table
└── docs/ai/<module>.md         ← single AI recipe (merged, not split)

Dimension 3: Example coverage (the "AI can copy" layer)
├── <module>/example_test.go    ← ExampleXxx for EVERY public constructor/option/inspect/predicate
├── <module>/example_business_test.go  ← 10+ complete business scenarios
└── examples/<module>-*/        ← 1-2 runnable demos with README

Dimension 4: Doc wiring (the "everything links" layer)
├── README.md (root)            ← module listed in table with status + entry doc link
├── docs/README.md              ← module in navigation index with reading path
└── <module>/README.md          ← links to design/contracts/ai/process docs + Example index

Dimension 5: Acceptance checks (the "verify it's done" layer)
├── docs/contracts/<module>.md           ← unbreakable contract
├── docs/process/<module>-acceptance-checklist.md  ← static/unit/integration checks
└── scripts/check-<module>-usage.sh      ← grep-based CI check (if module has usage rules)
```

A module is "doc-complete" only when ALL 5 dimensions pass. Anything less
means AI tools won't reliably use the module correctly.

## Workflow

Follow these 6 steps in order. Read the reference files at each step.

### Step 1: Inventory the current state

Run `scripts/check-module-docs.sh <module>` to get a quick gap report. The
script checks file existence, line counts, and Example counts.

Then read these to understand what's already there:

- `<module>/README.md` (if exists)
- `<module>/doc.go` (if exists)
- `<module>/example_test.go` (if exists)
- `docs/design/<module>.md` (if exists)
- `docs/ai/<module>.md` (if exists)
- `AGENTS.md` (look for existing module section)
- `CLAUDE.md` (look for existing module section)

Output a gap report listing what's missing per dimension.

### Step 2: Read the reference standards

Before generating any files, read:

- `references/completeness-checklist.md` — the 5-dimension checklist with pass/fail criteria
- `references/doc-templates.md` — templates for README / doc.go / AI configs
- `references/example-coverage.md` — example coverage formula and minimum bar

These contain the canonical patterns established by errorx. Copy them, don't
reimagine them — consistency across modules is the whole point.

### Step 3: Generate Dimension 1 (single-entry docs)

Generate or rewrite:

1. **`<module>/README.md`** — single entry, 15+ sections. Use the template in
   `references/doc-templates.md`. Must include:
   - "Why this module exists" (1 paragraph)
   - "30-second quickstart" (1 code block)
   - API cheatsheet table
   - Option list
   - Forbidden patterns
   - Doc map (links to design/contracts/ai/process)
   - **Example index table** (links each scenario to ExampleXxx function)

2. **`<module>/doc.go`** — rich package doc. `go doc <module>` must print a
   useful quickstart with constructor list and code example. Aim for 100+ lines.

3. **`docs/design/<module>.md`** — deep design spec. Only required for major
   modules (errorx/logx/httpx). Skip for minor modules (retry/selectors).

### Step 4: Generate Dimension 2 (AI tool configs)

For each AI tool config, **add a section for the new module** — do NOT
overwrite the file (other modules already have sections).

1. **`CLAUDE.md`** — add module to:
   - Hard rules section (if module has usage rules like errorx does)
   - Example lookup table (one row per common scenario)

2. **`.cursor/rules/<module>.mdc`** — new file with:
   - `globs` matching the module's typical use sites (e.g. `**/handler/**` for httpx)
   - `alwaysApply: true` for foundational modules (errorx, logx)
   - Forbidden patterns + required patterns + Example lookup table

3. **`.github/copilot-instructions.md`** — add module section with cheatsheet
   and Example references.

4. **`AGENTS.md`** — add module to:
   - Hard rules (if applicable)
   - "I want to..." Example lookup table

5. **`docs/ai/<module>.md`** — single AI recipe file. If the module previously
   had split AI docs (quickstart / recipes / forbidden-patterns), MERGE them
   into this one file. Never split AI docs across multiple files.

### Step 5: Generate Dimension 3 (examples)

This is the most labor-intensive step. Use `references/example-coverage.md`
to compute the required Example count.

1. **`<module>/example_test.go`** — must have `ExampleXxx` for:
   - Every public constructor (e.g. `ExampleBadRequest` for `BadRequest`)
   - Every public Option (e.g. `ExampleWithMetadata` for `WithMetadata`)
   - Every public inspect function (e.g. `ExampleCodeOf` for `CodeOf`)
   - Every public predicate (e.g. `ExampleIsNotFound` for `IsNotFound`)
   - nil safety (e.g. `ExampleCodeOf_nil`)
   - Third-party compatibility (if module integrates with foreign types)
   - Advanced features (Clone / Format / As / IsXxx)

   Each Example MUST have `// Output:` comment so `go test` verifies it.

2. **`<module>/example_business_test.go`** — 10 complete business scenarios
   showing the module in realistic handler → service → repository flow. See
   `references/example-coverage.md` for the standard 10 scenario categories.

3. **`examples/<module>-basic/`** — minimal runnable demo (main.go + README).

4. **`examples/<module>-http/`** (for HTTP-touching modules) — full HTTP server
   demo with 5-7 error scenarios and curl commands in README.

### Step 6: Generate Dimension 4 + 5 (wiring + acceptance)

1. **`README.md` (root)** — update the module status table to mark the module
   as `✅ stable` and link to its entry doc.

2. **`docs/README.md`** — add module to:
   - "Quick start" section
   - "AI coding guides" section
   - "Deep specs" section (if it has a design doc)
   - "Acceptance & ops" section (if it has an acceptance checklist)
   - Recommended reading paths

3. **`docs/contracts/<module>.md`** — unbreakable contract (required for
   major modules). Lists behaviors that cannot change without a major version
   bump.

4. **`docs/process/<module>-acceptance-checklist.md`** — static/unit/integration
   checks. Required for major modules.

5. **`scripts/check-<module>-usage.sh`** — grep-based CI check (only if the
   module has usage rules like "always use errorx, never raw errors"). For
   modules without usage rules (e.g. selectors), skip this.

### Step 7: Verify completeness

Run:

```bash
# 1. Run the gap check script
./scripts/check-module-docs.sh <module>

# 2. Run all Example tests
go test ./<module> -run=Example -v

# 3. Verify go doc shows useful output
go doc <module>
go doc <module>.SomePublicFunction

# 4. Verify AI configs mention the module
grep -l "<module>" CLAUDE.md AGENTS.md .github/copilot-instructions.md
ls .cursor/rules/<module>.mdc

# 5. Verify doc wiring
grep "<module>" README.md docs/README.md
```

All 5 must pass. If any fails, fix before declaring the module doc-complete.

## Output format

The skill produces:

1. **Gap report** (step 1) — what's missing per dimension
2. **Generated files** (steps 3-6) — patch files for each missing/improved doc
3. **Verification report** (step 7) — pass/fail for each of the 5 dimensions

Package all generated files as a single patch directory:

```text
<module>-doc-complete-patch/
├── PATCH_SUMMARY.md
├── <module>/
│   ├── README.md
│   ├── doc.go
│   ├── example_test.go
│   └── example_business_test.go
├── docs/
│   ├── ai/<module>.md
│   ├── design/<module>.md         (if major module)
│   ├── contracts/<module>.md      (if major module)
│   ├── process/<module>-acceptance-checklist.md  (if major module)
│   └── README.md (patch)
├── examples/
│   ├── <module>-basic/
│   └── <module>-http/             (if HTTP-touching)
├── CLAUDE.md (patch)
├── AGENTS.md (patch)
├── .cursor/rules/<module>.mdc
├── .github/copilot-instructions.md (patch)
├── README.md (root, patch)
└── scripts/check-<module>-usage.sh  (if module has usage rules)
```

## Key principles

### 1. Single source of truth

Each piece of information lives in ONE file. Other files LINK to it, never
copy it. If `errorx.NotFound` example appears in 5 files, maintenance
becomes a nightmare. The README is the single entry; AI configs LINK to
the README, they don't duplicate it.

### 2. AI configs point to Examples, not source

When AI tools read `CLAUDE.md` / `.cursor/rules/`, they see a lookup table
mapping scenarios to `ExampleXxx` functions. AI then runs `go doc errorx.NotFound`
to see the Example inline. This is far more reliable than AI exploring the
repo on its own.

### 3. Example coverage > 90%

Count public API surface (constructors + options + inspect + predicates).
Example coverage = (APIs with ExampleXxx) / (total public APIs). Target 90%+.
Below 70%, AI will use the module incorrectly because it doesn't know
advanced features exist.

### 4. Merge, don't split

If a module has `docs/ai/00-quickstart.md` + `docs/ai/recipes/<module>.md` +
`docs/ai/99-forbidden-patterns.md`, MERGE them into one `docs/ai/<module>.md`.
AI tools get confused by split docs. One file per module per doc type.

### 5. Match errorx patterns exactly

Consistency across modules is the whole point. Don't invent new patterns for
logx or httpx — copy errorx's README structure, doc.go style, Example naming,
AI config layout. Future modules (dbx, cachex, authx) all follow the same
template.

## Reference files

Read these at the relevant step:

- `references/completeness-checklist.md` — Step 1 & 7 (the 5-dimension checklist)
- `references/doc-templates.md` — Step 3 & 4 (README / doc.go / AI config templates)
- `references/example-coverage.md` — Step 5 (Example coverage formula + 10 standard business scenarios)
- `scripts/check-module-docs.sh` — Step 1 & 7 (automated gap check)

## Module-specific notes

### For `errorx`

Already doc-complete (this skill's reference implementation). Use it as the
gold standard — when in doubt, check how errorx does it.

### For `logx`

Apply the same 5 dimensions. Key differences from errorx:
- AI rule: forbid `log.Printf` / `fmt.Println` / direct `slog` / direct `zap`
- Example scenarios: structured logging, log levels, log filtering, context
  extraction, redaction, sampling
- `.cursor/rules/logx.mdc` globs: all `*.go` files (logging is everywhere)

### For `httpx` (when created)

- AI rule: forbid `net/http` server setup in business code
- Example scenarios: handler registration, middleware chain, error response,
  streaming (SSE), WebSocket, graceful shutdown
- `.cursor/rules/httpx.mdc` globs: `**/handler/**`, `**/server/**`

### For `dbx` (when created)

- AI rule: forbid `database/sql` direct usage in business code
- Example scenarios: connection pool, transaction, query, scan, migration
- `.cursor/rules/dbx.mdc` globs: `**/repository/**`, `**/repo/**`, `**/data/**`

### For minor modules (retry / selectors / encoding)

Skip Dimension 5 (no acceptance checklist or contract needed). Just do
Dimensions 1-4. Use the 80/20 rule: 80% of the value comes from single-entry
README + Example coverage + AI config mention.
