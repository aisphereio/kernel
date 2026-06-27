#!/usr/bin/env bash
#
# check-module-docs.sh — verify a kernel module meets the 5-dimension
# documentation completeness standard.
#
# Usage:
#   ./scripts/check-module-docs.sh <module>
#   ./scripts/check-module-docs.sh errorx
#   ./scripts/check-module-docs.sh logx
#
# Exit codes:
#   0  module is doc-complete (all 5 dimensions pass)
#   1  module has gaps (NOT doc-complete)
#   2  script error (module not found, etc.)
#
# This script checks Dimension 1-4. Dimension 5 (acceptance checks) is
# optional for minor modules — see references/completeness-checklist.md.

set -uo pipefail
# Note: NO `set -e` — we want grep/find returning non-zero (no match) to be
# handled by the check functions, not abort the script.

# Colors
if [ -t 1 ]; then
  RED='\033[0;31m'
  YELLOW='\033[0;33m'
  GREEN='\033[0;32m'
  BLUE='\033[0;34m'
  NC='\033[0m'
else
  RED=''; YELLOW=''; GREEN=''; BLUE=''; NC=''
fi

# --- Args ---
MODULE="${1:-}"
if [ -z "$MODULE" ]; then
  echo "usage: $0 <module>"
  echo "example: $0 errorx"
  exit 2
fi

# Strip leading "kernel/" or "./" if user passed full path
MODULE="${MODULE#kernel/}"
MODULE="${MODULE#./}"

# Find repo root — prefer CWD, fallback to script-relative
# Walk up from CWD looking for go.mod
find_repo_root() {
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "$dir/go.mod" ]; then
      echo "$dir"
      return 0
    fi
    dir="$(dirname "$dir")"
  done
  # Fallback: script-relative (../..)
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
}

REPO_ROOT="$(find_repo_root)"
cd "$REPO_ROOT"

if [ ! -d "$MODULE" ]; then
  echo "${RED}error${NC}: module directory '$MODULE' not found"
  echo "looked in: $REPO_ROOT/$MODULE"
  echo "tip: run this script from the kernel repo root, or pass the full path"
  exit 2
fi

# --- Counters ---
PASS=0
FAIL=0
WARN=0
GAPS=""

record_pass() {
  PASS=$((PASS + 1))
  printf "  ${GREEN}✓${NC} %s\n" "$1"
}

record_fail() {
  FAIL=$((FAIL + 1))
  printf "  ${RED}✗${NC} %s\n" "$1"
  GAPS="${GAPS}\n  - $1"
}

record_warn() {
  WARN=$((WARN + 1))
  printf "  ${YELLOW}⚠${NC} %s\n" "$1"
}

check_file_exists() {
  local file="$1"
  local label="$2"
  if [ -f "$file" ]; then
    record_pass "$label exists"
    return 0
  else
    record_fail "$label missing ($file)"
    return 1
  fi
}

check_file_lines() {
  local file="$1"
  local min="$2"
  local label="$3"
  if [ ! -f "$file" ]; then
    record_fail "$label missing ($file)"
    return 1
  fi
  local lines
  lines=$(wc -l < "$file")
  if [ "$lines" -ge "$min" ]; then
    record_pass "$label has $lines lines (>= $min)"
  else
    record_fail "$label only $lines lines (need $min+)"
  fi
}

check_grep() {
  local pattern="$1"
  local file="$2"
  local label="$3"
  if [ ! -f "$file" ]; then
    record_fail "$label (file missing: $file)"
    return 1
  fi
  if grep -q "$pattern" "$file" 2>/dev/null; then
    record_pass "$label"
  else
    record_fail "$label (pattern '$pattern' not in $file)"
  fi
}

# --- Dimension 1: Single-entry docs ---
echo "${BLUE}━━━ Dimension 1: Single-entry docs ━━━${NC}"

check_file_exists "$MODULE/README.md" "$MODULE/README.md"
check_file_lines "$MODULE/README.md" 200 "$MODULE/README.md"
check_file_exists "$MODULE/doc.go" "$MODULE/doc.go"
check_file_lines "$MODULE/doc.go" 80 "$MODULE/doc.go"

# Check README contains key sections
if [ -f "$MODULE/README.md" ]; then
  check_grep "30 秒\|30-second\|quickstart\|Quickstart" "$MODULE/README.md" "README has quickstart section"
  check_grep "文档地图\|Doc map\|Documentation map\|文档索引" "$MODULE/README.md" "README has doc map"
  check_grep "Example" "$MODULE/README.md" "README mentions Examples"
fi

# Design doc (major modules only — warn if missing for errorx/logx/httpx/dbx)
case "$MODULE" in
  errorx|logx|httpx|dbx|cachex|authx)
    check_file_exists "docs/design/$MODULE.md" "docs/design/$MODULE.md (major module)"
    check_file_lines "docs/design/$MODULE.md" 500 "docs/design/$MODULE.md"
    ;;
  *)
    if [ -f "docs/design/$MODULE.md" ]; then
      record_pass "docs/design/$MODULE.md exists (optional for minor modules)"
    else
      record_warn "docs/design/$MODULE.md missing (optional for minor modules)"
    fi
    ;;
esac

# --- Dimension 2: AI tool configs ---
echo
echo "${BLUE}━━━ Dimension 2: AI tool configs ━━━${NC}"

check_grep "$MODULE" CLAUDE.md "CLAUDE.md mentions $MODULE"
check_grep "$MODULE" AGENTS.md "AGENTS.md mentions $MODULE"
check_grep "$MODULE" .github/copilot-instructions.md "copilot-instructions mentions $MODULE"
check_file_exists ".cursor/rules/$MODULE.mdc" ".cursor/rules/$MODULE.mdc"
check_file_exists "docs/ai/$MODULE.md" "docs/ai/$MODULE.md (single AI recipe)"

# Check AI docs are NOT split (should be 1 file, not 3+)
AI_DOC_COUNT=$(find docs/ai -name "*$MODULE*" -type f 2>/dev/null | wc -l)
if [ "$AI_DOC_COUNT" -eq 1 ]; then
  record_pass "AI docs merged into single file"
elif [ "$AI_DOC_COUNT" -gt 1 ]; then
  record_fail "AI docs split into $AI_DOC_COUNT files (merge them): $(find docs/ai -name "*$MODULE*" -type f | tr '\n' ' ')"
else
  record_fail "no AI doc found"
fi

# Check AI configs have Example lookup tables
if [ -f "CLAUDE.md" ] && grep -q "$MODULE" CLAUDE.md; then
  check_grep "Example$MODULE\|Example.*$MODULE" CLAUDE.md "CLAUDE.md has Example table for $MODULE" || true
fi
if [ -f ".cursor/rules/$MODULE.mdc" ]; then
  check_grep "Example" ".cursor/rules/$MODULE.mdc" ".cursor/rules/$MODULE.mdc mentions Examples"
fi

# --- Dimension 3: Example coverage ---
echo
echo "${BLUE}━━━ Dimension 3: Example coverage ━━━${NC}"

check_file_exists "$MODULE/example_test.go" "$MODULE/example_test.go"

# Count public APIs
CONSTRUCTORS=$( (grep -HE '^func [A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null || true) \
  | grep -v '_test.go:' | grep -vE '^.*:func With[A-Z]' | grep -v '//' | wc -l)
OPTIONS=$( (grep -HE '^func With[A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null || true) \
  | grep -v '_test.go:' | wc -l)
INSPECT=$( (grep -HE '^func [A-Z][a-zA-Z]+Of\(|^func Is[A-Z][a-zA-Z]+\(' $MODULE/*.go 2>/dev/null || true) \
  | grep -v '_test.go:' | wc -l)
TOTAL_API=$((CONSTRUCTORS + OPTIONS + INSPECT))

# Count Examples
EXAMPLES=$(grep -c '^func Example' "$MODULE/example_test.go" 2>/dev/null || true)
EXAMPLES=${EXAMPLES:-0}
BUSINESS_EXAMPLES=$(grep -Ec '^func Example(Business|_business)' "$MODULE/example_business_test.go" 2>/dev/null || true)
BUSINESS_EXAMPLES=${BUSINESS_EXAMPLES:-0}
TOTAL_EXAMPLES=$((EXAMPLES + BUSINESS_EXAMPLES))

echo "  Public APIs: $TOTAL_API (constructors=$CONSTRUCTORS, options=$OPTIONS, inspect=$INSPECT)"
echo "  Examples: $TOTAL_EXAMPLES (test=$EXAMPLES, business=$BUSINESS_EXAMPLES)"

# Coverage ratio
if [ "$TOTAL_API" -gt 0 ]; then
  RATIO=$((EXAMPLES * 100 / TOTAL_API))
  echo "  Coverage ratio: ${RATIO}%"
  if [ "$RATIO" -ge 90 ]; then
    record_pass "Example coverage >= 90%"
  elif [ "$RATIO" -ge 70 ]; then
    record_warn "Example coverage 70-89% (needs improvement)"
  else
    record_fail "Example coverage < 70% (need more Examples)"
  fi
fi

# Business scenarios
if [ "$BUSINESS_EXAMPLES" -ge 10 ]; then
  record_pass "$BUSINESS_EXAMPLES business scenarios (>= 10)"
else
  record_fail "only $BUSINESS_EXAMPLES business scenarios (need 10+)"
fi

# Example_test.go line count (rough indicator)
check_file_lines "$MODULE/example_test.go" 200 "$MODULE/example_test.go"

# Runnable examples
check_file_exists "examples/$MODULE-basic/main.go" "examples/$MODULE-basic/main.go"
check_file_exists "examples/$MODULE-basic/README.md" "examples/$MODULE-basic/README.md"

# HTTP example (for HTTP-touching modules)
case "$MODULE" in
  errorx|logx|httpx|grpcx)
    check_file_exists "examples/$MODULE-http/main.go" "examples/$MODULE-http/main.go"
    check_file_exists "examples/$MODULE-http/README.md" "examples/$MODULE-http/README.md"
    ;;
esac

# --- Dimension 4: Doc wiring ---
echo
echo "${BLUE}━━━ Dimension 4: Doc wiring ━━━${NC}"

check_grep "$MODULE" README.md "root README.md mentions $MODULE"
check_grep "$MODULE" docs/README.md "docs/README.md mentions $MODULE"

if [ -f "$MODULE/README.md" ]; then
  if [ -f "docs/design/$MODULE.md" ]; then
    check_grep "docs/design/$MODULE" "$MODULE/README.md" "README links to design doc"
  fi
  if [ -f "docs/contracts/$MODULE.md" ]; then
    check_grep "docs/contracts/$MODULE" "$MODULE/README.md" "README links to contracts"
  fi
  if [ -f "docs/ai/$MODULE.md" ]; then
    check_grep "docs/ai/$MODULE" "$MODULE/README.md" "README links to AI guide"
  fi
fi

# Cross-links from AI configs to module README
for cfg in CLAUDE.md AGENTS.md .github/copilot-instructions.md; do
  if [ -f "$cfg" ] && grep -q "$MODULE" "$cfg"; then
    check_grep "$MODULE/README.md\|docs/ai/$MODULE" "$cfg" "$cfg links to $MODULE entry docs"
  fi
done

# --- Dimension 5: Acceptance checks (optional for minor modules) ---
echo
echo "${BLUE}━━━ Dimension 5: Acceptance checks ━━━${NC}"

IS_MAJOR=false
case "$MODULE" in
  errorx|logx|httpx|dbx|cachex|authx|grpcx)
    IS_MAJOR=true
    ;;
esac

if [ "$IS_MAJOR" = true ]; then
  check_file_exists "docs/contracts/$MODULE.md" "docs/contracts/$MODULE.md"
  check_file_exists "docs/process/$MODULE-acceptance-checklist.md" "acceptance checklist"
else
  if [ -f "docs/contracts/$MODULE.md" ]; then
    record_pass "docs/contracts/$MODULE.md exists (optional for minor modules)"
  else
    record_warn "docs/contracts/$MODULE.md missing (optional for minor modules)"
  fi
  if [ -f "docs/process/$MODULE-acceptance-checklist.md" ]; then
    record_pass "acceptance checklist exists (optional for minor modules)"
  else
    record_warn "acceptance checklist missing (optional for minor modules)"
  fi
fi

# Usage check script (only if module has usage rules)
if [ -f "$MODULE/README.md" ] && grep -qiE "FORBIDDEN|❌|禁止" "$MODULE/README.md"; then
  check_file_exists "scripts/check-$MODULE-usage.sh" "scripts/check-$MODULE-usage.sh (module has usage rules)"
else
  record_pass "no usage rules (no check script needed)"
fi

# CI workflow (optional)
if [ -f ".github/workflows/$MODULE-enforcement.yml" ]; then
  record_pass ".github/workflows/$MODULE-enforcement.yml exists"
else
  record_warn ".github/workflows/$MODULE-enforcement.yml missing (optional)"
fi

# --- Summary ---
echo
echo "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo "Module: $MODULE"
echo "  Pass: $PASS"
echo "  Warn: $WARN (optional items missing)"
echo "  Fail: $FAIL"
echo

if [ "$FAIL" -eq 0 ]; then
  if [ "$WARN" -eq 0 ]; then
    echo "${GREEN}✓ $MODULE is DOC-COMPLETE${NC} (all 5 dimensions pass)"
  else
    echo "${GREEN}✓ $MODULE is DOC-COMPLETE${NC} (all required dimensions pass, $WARN optional items missing)"
  fi
  exit 0
else
  echo "${RED}✗ $MODULE is NOT doc-complete${NC} ($FAIL gaps)"
  echo
  echo "Gaps:"
  echo -e "$GAPS"
  echo
  echo "Fix steps:"
  echo "  1. Read kernel-module-doc-complete/references/completeness-checklist.md"
  echo "  2. Read kernel-module-doc-complete/references/doc-templates.md"
  echo "  3. Generate missing files using the templates"
  echo "  4. Re-run: $0 $MODULE"
  exit 1
fi
