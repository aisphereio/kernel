# Kernel Module Completeness Checklist

The 5-dimension standard. A module is "doc-complete" only when ALL 5 dimensions
pass. Use this checklist at Step 1 (gap analysis) and Step 7 (verification).

## How to use

For each dimension, run the check command and verify the criteria. If ANY
criterion fails, the dimension fails. If any dimension fails, the module is
NOT doc-complete.

---

## Dimension 1: Single-entry docs

### Check

```bash
MODULE=errorx  # replace with target module

# File existence
test -f $MODULE/README.md && echo "✓ README.md" || echo "✗ README.md missing"
test -f $MODULE/doc.go && echo "✓ doc.go" || echo "✗ doc.go missing"
test -f docs/design/$MODULE.md && echo "✓ design doc" || echo "✗ design doc missing (major modules only)"

# Line count checks (minimum bar)
LINES_README=$(wc -l < $MODULE/README.md 2>/dev/null || echo 0)
LINES_DOC=$(wc -l < $MODULE/doc.go 2>/dev/null || echo 0)
test $LINES_README -ge 200 && echo "✓ README $LINES_README lines" || echo "✗ README only $LINES_README lines (need 200+)"
test $LINES_DOC -ge 80 && echo "✓ doc.go $LINES_DOC lines" || echo "✗ doc.go only $LINES_DOC lines (need 80+)"
```

### Pass criteria

- [ ] `<module>/README.md` exists, 200+ lines, contains:
  - "Why this module exists" section
  - "30-second quickstart" code block
  - API cheatsheet table (constructor → HTTP/gRPC/retryable)
  - Option list (every public Option with one-line description)
  - Forbidden patterns section
  - Doc map (links to design/contracts/ai/process)
  - Example index table (scenario → ExampleXxx function)
- [ ] `<module>/doc.go` exists, 80+ lines, `go doc <module>` prints:
  - Package purpose (1 paragraph)
  - Quickstart code block
  - Constructor list
  - Option list
  - Inspect helper list
  - Forbidden patterns summary
- [ ] `docs/design/<module>.md` exists for major modules (errorx/logx/httpx/dbx)
  - 500+ lines
  - Covers: design goals, core types, public API, integration with other modules, AI rules

### Fail examples

- README is 50 lines → fail (too thin, AI can't learn from it)
- doc.go is 7 lines → fail (`go doc` returns nothing useful)
- No Example index table in README → fail (AI can't find Examples)

---

## Dimension 2: AI tool configs

### Check

```bash
MODULE=errorx

# AI tool config files mention the module
grep -c "$MODULE" CLAUDE.md && echo "✓ CLAUDE.md mentions $MODULE" || echo "✗ CLAUDE.md missing $MODULE"
grep -c "$MODULE" AGENTS.md && echo "✓ AGENTS.md mentions $MODULE" || echo "✗ AGENTS.md missing $MODULE"
grep -c "$MODULE" .github/copilot-instructions.md && echo "✓ copilot-instructions mentions $MODULE" || echo "✗ copilot-instructions missing $MODULE"
test -f .cursor/rules/$MODULE.mdc && echo "✓ .cursor/rules/$MODULE.mdc" || echo "✗ .cursor/rules/$MODULE.mdc missing"

# AI recipe doc (single file, not split)
test -f docs/ai/$MODULE.md && echo "✓ docs/ai/$MODULE.md" || echo "✗ docs/ai/$MODULE.md missing"
find docs/ai -name "*$MODULE*" -type f | wc -l  # should be 1, not 3+
```

### Pass criteria

- [ ] `CLAUDE.md` mentions module with:
  - Hard rule section (if module has usage rules)
  - Example lookup table (scenario → ExampleXxx)
- [ ] `.cursor/rules/<module>.mdc` exists with:
  - `globs` field matching module's use sites
  - `alwaysApply: true` for foundational modules
  - Forbidden patterns
  - Example lookup table
- [ ] `.github/copilot-instructions.md` mentions module with cheatsheet
- [ ] `AGENTS.md` mentions module in:
  - Hard rules (if applicable)
  - "I want to..." Example lookup table
- [ ] `docs/ai/<module>.md` is a SINGLE file (not split into quickstart/recipes/forbidden)
  - 200+ lines
  - Contains: cheatsheet, 10+ recipes, forbidden patterns, complete handler example, acceptance checklist

### Fail examples

- 3 AI docs split as `00-quickstart.md` / `recipes/errorx.md` / `99-forbidden-patterns.md` → fail (merge them)
- `.cursor/rules/` missing the module → fail (Cursor won't auto-enforce rules)
- `CLAUDE.md` doesn't have an Example lookup table → fail (AI won't find Examples)

---

## Dimension 3: Example coverage

### Check

```bash
MODULE=errorx

# Count public APIs
CONSTRUCTORS=$(grep -E '^func [A-Z][a-zA-Z]+\(' $MODULE/*.go | grep -v _test.go | wc -l)
OPTIONS=$(grep -E '^func With[A-Z][a-zA-Z]+\(' $MODULE/*.go | grep -v _test.go | wc -l)
INSPECT=$(grep -E '^func [A-Z][a-zA-Z]+Of\(|^func Is[A-Z][a-zA-Z]+\(' $MODULE/*.go | grep -v _test.go | wc -l)
TOTAL_API=$((CONSTRUCTORS + OPTIONS + INSPECT))

# Count ExampleXxx functions
EXAMPLES=$(grep -c '^func Example' $MODULE/example_test.go 2>/dev/null || echo 0)
BUSINESS_EXAMPLES=$(grep -c '^func ExampleBusiness' $MODULE/example_business_test.go 2>/dev/null || echo 0)

echo "Public APIs: $TOTAL_API"
echo "Examples: $EXAMPLES (test) + $BUSINESS_EXAMPLES (business) = $((EXAMPLES + BUSINESS_EXAMPLES))"

# Coverage ratio (rough — manual review needed for accuracy)
if [ $TOTAL_API -gt 0 ]; then
  RATIO=$((EXAMPLES * 100 / TOTAL_API))
  echo "Coverage ratio: ${RATIO}%"
  test $RATIO -ge 90 && echo "✓ coverage >= 90%" || echo "✗ coverage < 90% (need more Examples)"
fi

# Business scenario check
test $BUSINESS_EXAMPLES -ge 10 && echo "✓ 10+ business scenarios" || echo "✗ only $BUSINESS_EXAMPLES business scenarios (need 10+)"

# Runnable examples
test -d examples/$MODULE-basic && echo "✓ examples/$MODULE-basic/" || echo "✗ examples/$MODULE-basic/ missing"
```

### Pass criteria

- [ ] `<module>/example_test.go` has `ExampleXxx` for:
  - Every public constructor (e.g. `ExampleBadRequest` for `BadRequest`)
  - Every public Option (e.g. `ExampleWithMetadata` for `WithMetadata`)
  - Every public inspect function (e.g. `ExampleCodeOf` for `CodeOf`)
  - Every public predicate (e.g. `ExampleIsNotFound` for `IsNotFound`)
  - nil safety (e.g. `ExampleCodeOf_nil`)
- [ ] Each Example has `// Output:` comment (verified by `go test`)
- [ ] `<module>/example_business_test.go` has 10+ complete business scenarios
- [ ] `examples/<module>-basic/` exists with main.go + README
- [ ] `examples/<module>-http/` exists for HTTP-touching modules, with 5+ scenarios
- [ ] Coverage ratio >= 90% (Examples / public APIs)

### Fail examples

- 18 Examples for 50 public APIs (36% coverage) → fail
- Examples without `// Output:` → fail (can't be auto-verified)
- No business scenarios → fail (AI doesn't see real usage patterns)

### The 10 standard business scenarios

`example_business_test.go` should include these 10 scenario categories (adapt
names to the module):

1. **Repository layer** — convert DB/storage errors to module errors
2. **Service layer** — validation + business rules + conflict detection
3. **Upstream dependency failure** — model API / external service timeout
4. **Authz denied** — permission check with metadata
5. **Worker retry decision** — retryable flag drives retry logic
6. **HTTP response shape** — JSON output that clients see
7. **Audit record** — what auditx records from the error
8. **Log entry** — what logx records (with redaction)
9. **Metrics labels** — low-cardinality Prometheus labels
10. **Multi-layer wrap** — error chain preserved across layers

---

## Dimension 4: Doc wiring

### Check

```bash
MODULE=errorx

# Root README links to module
grep -c "$MODULE" README.md && echo "✓ root README mentions $MODULE" || echo "✗ root README missing $MODULE"

# docs/README.md links to module
grep -c "$MODULE" docs/README.md && echo "✓ docs/README.md mentions $MODULE" || echo "✗ docs/README.md missing $MODULE"

# Module README links to design/contracts/ai
grep -c "docs/design/$MODULE" $MODULE/README.md && echo "✓ README links to design doc" || true
grep -c "docs/contracts/$MODULE" $MODULE/README.md && echo "✓ README links to contracts" || true
grep -c "docs/ai/$MODULE" $MODULE/README.md && echo "✓ README links to AI guide" || true

# Cross-links between AI configs and module README
grep -c "$MODULE/README.md" CLAUDE.md AGENTS.md .github/copilot-instructions.md
```

### Pass criteria

- [ ] `README.md` (root) lists module in:
  - Module status table (with `✅ stable` / `🚧 in progress`)
  - Documentation map section
- [ ] `docs/README.md` lists module in:
  - Quick start section
  - AI coding guides section
  - Deep specs section (if major module)
  - Acceptance & ops section (if has checklist)
- [ ] `<module>/README.md` links to:
  - `docs/design/<module>.md` (if exists)
  - `docs/contracts/<module>.md` (if exists)
  - `docs/ai/<module>.md`
  - `docs/process/<module>-acceptance-checklist.md` (if exists)
  - Example files (`example_test.go`, `example_business_test.go`)
  - Runnable examples (`examples/<module>-*/`)
- [ ] AI config files (CLAUDE.md, AGENTS.md, copilot-instructions.md) link to
  `<module>/README.md` and `docs/ai/<module>.md`

### Fail examples

- Module exists but root README doesn't list it → fail (orphaned module)
- Module README doesn't link to design doc → fail (design doc unreachable)
- AI configs don't link to module README → fail (AI can't find entry point)

---

## Dimension 5: Acceptance checks

### Check

```bash
MODULE=errorx

# Contract doc
test -f docs/contracts/$MODULE.md && echo "✓ contracts/$MODULE.md" || echo "✗ contracts/$MODULE.md missing"

# Acceptance checklist
test -f docs/process/$MODULE-acceptance-checklist.md && echo "✓ acceptance checklist" || echo "✗ acceptance checklist missing"

# Usage check script (only for modules with usage rules)
if grep -q "Forbidden imports\|FORBIDDEN\|❌" $MODULE/README.md; then
  test -f scripts/check-$MODULE-usage.sh && echo "✓ check-$MODULE-usage.sh" || echo "✗ check-$MODULE-usage.sh missing (module has usage rules)"
fi

# CI workflow runs the check
test -f .github/workflows/$MODULE-enforcement.yml && echo "✓ CI workflow" || echo "✗ CI workflow missing (optional)"
```

### Pass criteria

- [ ] `docs/contracts/<module>.md` exists for major modules with:
  - "Unbreakable behaviors" section
  - "Breaking change判定" section
  - Verification commands
- [ ] `docs/process/<module>-acceptance-checklist.md` exists for major modules with:
  - Static checks (grep/ls commands)
  - Unit checks (go test commands)
  - Integration checks (end-to-end scenarios)
- [ ] `scripts/check-<module>-usage.sh` exists IF the module has usage rules
  (e.g. errorx forbids `errors.New` in business code)
- [ ] `.github/workflows/<module>-enforcement.yml` runs the check script on
  every PR (optional but recommended for major modules)

### Fail examples

- Module has forbidden patterns in README but no check script → fail (rules
  aren't enforced)
- No acceptance checklist → fail (no verification procedure)

### Skip for minor modules

Minor modules (retry / selectors / encoding / registry) can skip Dimension 5.
They don't have usage rules and don't need contracts. Just do Dimensions 1-4.

---

## Summary scorecard

Print this at the end of Step 1 (gap analysis) and Step 7 (verification):

```text
Module: <module>
─────────────────────────────────────────────
Dimension 1: Single-entry docs        [✓/✗]
Dimension 2: AI tool configs          [✓/✗]
Dimension 3: Example coverage         [✓/✗]
Dimension 4: Doc wiring               [✓/✗]
Dimension 5: Acceptance checks        [✓/✗/N/A]
─────────────────────────────────────────────
Overall: <DOC-COMPLETE / NEEDS WORK>

Gaps:
- <list each failing criterion>
```

A module is "doc-complete" only when ALL applicable dimensions pass.
Dimensions marked N/A (e.g. Dimension 5 for minor modules) don't count against
the module.
