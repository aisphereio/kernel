#!/usr/bin/env bash
#
# check-errorx-usage.sh — verify business code uses errorx, not raw errors.
#
# This is a fast grep-based check that complements golangci-lint. It runs in
# <1 second and works without any Go toolchain installed, making it ideal for
# pre-commit hooks and lightweight CI gates.
#
# Usage:
#   ./scripts/check-errorx-usage.sh           # check all business .go files
#   ./scripts/check-errorx-usage.sh --fix     # show fix suggestions
#   ./scripts/check-errorx-usage.sh --verbose # print every checked file
#
# Exit codes:
#   0  no violations
#   1  violations found (CI should fail)
#   2  script error
#
# CI integration (GitHub Actions):
#   - name: Check errorx usage
#     run: ./scripts/check-errorx-usage.sh
#

set -euo pipefail

MODE="${1:-check}"
VERBOSE=false
FIX=false
case "$MODE" in
  --verbose) VERBOSE=true ;;
  --fix)     FIX=true ;;
  --check|check) ;;
  *) echo "usage: $0 [--check|--fix|--verbose]"; exit 2 ;;
esac

# Colors (disabled if not a TTY)
if [ -t 1 ]; then
  RED='\033[0;31m'
  YELLOW='\033[0;33m'
  GREEN='\033[0;32m'
  NC='\033[0m'
else
  RED=''; YELLOW=''; GREEN=''; NC=''
fi

# Find repository root (directory containing go.mod)
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if [ ! -f go.mod ]; then
  echo "${RED}error${NC}: go.mod not found at $REPO_ROOT" >&2
  exit 2
fi

# Business code globs (NOT tests, NOT kernel internals, NOT examples)
BUSINESS_PATHS=(
  -path './handler/*'        -o
  -path './service/*'        -o
  -path './repository/*'     -o
  -path './repo/*'           -o
  -path './worker/*'         -o
  -path './controller/*'     -o
  -path './internal/handler/*'  -o
  -path './internal/service/*'  -o
  -path './internal/repository/*' -o
  -path './internal/worker/*'
)

# Excluded paths (tests, kernel packages, examples, generated code)
EXCLUDE_PRUNE=(
  -name '*_test.go' -prune -o
  -name '*.pb.go' -prune -o
  -name 'wire_gen.go' -prune -o
  -path './errorx/*' -prune -o
  -path './logx/*' -prune -o
  -path './config/*' -prune -o
  -path './transportx/*' -prune -o
  -path './internal/testutil/*' -prune -o
  -path './examples/*' -prune -o
  -path './contrib/*' -prune -o
  -path './cmd/*' -prune -o
  -type d -name vendor -prune
)

# Collect business .go files
mapfile -t FILES < <(
  find . \( "${EXCLUDE_PRUNE[@]}" \) -o \
    \( -path './handler/*' -o -path './service/*' -o -path './repository/*' \
       -o -path './repo/*' -o -path './worker/*' -o -path './controller/*' \
       -o -path './internal/handler/*' -o -path './internal/service/*' \
       -o -path './internal/repository/*' -o -path './internal/worker/*' \) \
    -name '*.go' -print
)

if [ "${#FILES[@]}" -eq 0 ]; then
  echo "${GREEN}✓${NC} no business .go files found (nothing to check)"
  exit 0
fi

if $VERBOSE; then
  echo "checking ${#FILES[@]} business .go files..."
fi

VIOLATIONS=0
REPORT=""

check_pattern() {
  local label="$1"
  local pattern="$2"
  local suggestion="$3"
  local file

  for file in "${FILES[@]}"; do
    # shellcheck disable=SC2207
    local matches=()
    while IFS= read -r line; do
      [ -n "$line" ] && matches+=("$line")
    done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)

    if [ "${#matches[@]}" -gt 0 ]; then
      for match in "${matches[@]}"; do
        VIOLATIONS=$((VIOLATIONS + 1))
        REPORT+="${RED}✗${NC} ${label}${NC}
  file: ${YELLOW}${file}${NC}:${match%%:*}
  code: $(echo "$match" | cut -d: -f2-)
"
        if $FIX; then
          REPORT+="  fix:  ${suggestion}${NC}
"
        fi
      done
    fi
  done
}

# === Rule 1: no errors.New in business code ===
check_pattern \
  "errors.New() in business code" \
  'errors\.New\(' \
  'use errorx.New("DOMAIN_RESOURCE_REASON", "message", errorx.WithMetadata(...))'

# === Rule 2: no fmt.Errorf in business code ===
check_pattern \
  "fmt.Errorf() in business code" \
  'fmt\.Errorf\(' \
  'use errorx.Wrap(err, "DOMAIN_RESOURCE_FAILED", errorx.WithMessage("..."))'

# === Rule 3: no panic() in business code (except in main.go init) ===
check_pattern \
  "panic() in business code" \
  '\bpanic\(' \
  'return errorx.Internal("DOMAIN_INTERNAL_FAILED", "...") instead of panic'

# === Rule 4: no raw error passthrough (return nil, err) ===
# This is harder to grep accurately, so we only flag obvious cases.
check_pattern \
  "raw error passthrough (return ..., err) — wrap with errorx" \
  'return nil,\s*err\b' \
  'return nil, errorx.Wrap(err, "DOMAIN_RESOURCE_FAILED", errorx.WithMessage("..."))'

# === Rule 5: no log.Printf / fmt.Println in business code ===
check_pattern \
  "log.Printf() in business code" \
  '\blog\.(Print|Fatal|Panic)\(' \
  'use logx.FromContext(ctx).Info("...", logx.String("key", val))'

check_pattern \
  "fmt.Println() / fmt.Printf() in business code (use logx)" \
  '\bfmt\.(Print|Println|Printf|Fprint|Fprintln|Fprintf)\(' \
  'use logx.FromContext(ctx).Info("...", logx.String("key", val))'

# === Rule 6: no direct slog / zap in business code ===
check_pattern \
  "direct slog usage in business code" \
  '\bslog\.(Debug|Info|Warn|Error|Fatal|Panic)\(' \
  'use logx.FromContext(ctx).Info(...) — slog bypasses Kernel logging contract'

check_pattern \
  "direct zap usage in business code" \
  '\bzap\.(L|S|New)\(' \
  'use logx.FromContext(ctx).Info(...) — zap bypasses Kernel logging contract'

# === Rule 7: no os.Getenv in business code ===
check_pattern \
  "os.Getenv() in business code" \
  '\bos\.Getenv\(' \
  'use kernel config; os.Getenv bypasses config layer'

# === Rule 8: dynamic value in errorx.Code (high cardinality risk) ===
check_pattern \
  "dynamic value in errorx.Code — high cardinality risk" \
  'errorx\.(Code|New)\([^"]*(\+|fmt\.Sprintf|%v|%s|_ID)' \
  'use a stable code like "AIHUB_SKILL_NOT_FOUND" and put dynamic value in errorx.WithMetadata'

# === Report ===
if [ "$VIOLATIONS" -eq 0 ]; then
  echo "${GREEN}✓${NC} errorx usage OK — ${#FILES[@]} business files checked, no violations"
  exit 0
fi

echo "${RED}✗${NC} errorx usage check failed — $VIOLATIONS violation(s) found"
echo
echo "$REPORT"
echo "Read docs/ai/errorx.md for the complete AI coding recipe."
echo
echo "Quick fix cheatsheet:"
echo "  errors.New(\"x\")              → errorx.BadRequest(\"DOMAIN_REASON\", \"x\")"
echo "  fmt.Errorf(\"x: %w\", err)    → errorx.Wrap(err, \"DOMAIN_FAILED\", errorx.WithMessage(\"x\"))"
echo "  return nil, err              → return nil, errorx.Wrap(err, \"DOMAIN_FAILED\", ...)"
echo "  log.Printf(\"x\")             → logx.FromContext(ctx).Info(\"x\")"
echo "  panic(err)                   → return errorx.Internal(\"DOMAIN_INTERNAL\", \"...\")"
exit 1
