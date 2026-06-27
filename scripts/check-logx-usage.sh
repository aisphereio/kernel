#!/usr/bin/env bash
#
# check-logx-usage.sh — verify business code uses logx, not raw log.
#
# This is a fast grep-based check that complements golangci-lint. It runs in
# <1 second and works without any Go toolchain installed.
#
# Usage:
#   ./scripts/check-logx-usage.sh           # check all business .go files
#   ./scripts/check-logx-usage.sh --verbose # print every checked file
#
# Exit codes:
#   0  no violations
#   1  violations found (CI should fail)
#   2  script error

set -uo pipefail

MODE="${1:-check}"
VERBOSE=false
case "$MODE" in
  --verbose) VERBOSE=true ;;
  --check|check) ;;
  *) echo "usage: $0 [--check|--verbose]"; exit 2 ;;
esac

# Colors
if [ -t 1 ]; then
  RED='\033[0;31m'; YELLOW='\033[0;33m'; GREEN='\033[0;32m'; NC='\033[0m'
else
  RED=''; YELLOW=''; GREEN=''; NC=''
fi

# Find repo root
find_repo_root() {
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "$dir/go.mod" ]; then echo "$dir"; return 0; fi
    dir="$(dirname "$dir")"
  done
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
}
REPO_ROOT="$(find_repo_root)"
cd "$REPO_ROOT"

if [ ! -f go.mod ]; then
  echo "${RED}error${NC}: go.mod not found at $REPO_ROOT" >&2
  exit 2
fi

# Business code globs
BUSINESS_DIRS=(
  ./handler ./service ./repository ./repo ./worker ./controller
  ./internal/handler ./internal/service ./internal/repository ./internal/worker
)

# Excluded paths
EXCLUDE_PRUNE=(
  -name '*_test.go' -prune -o
  -name '*.pb.go' -prune -o
  -name 'wire_gen.go' -prune -o
  -path './logx/*' -prune -o
  -path './errorx/*' -prune -o
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
    local matches=()
    while IFS= read -r line; do
      [ -n "$line" ] && matches+=("$line")
    done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)

    if [ "${#matches[@]}" -gt 0 ]; then
      for match in "${matches[@]}"; do
        VIOLATIONS=$((VIOLATIONS + 1))
        REPORT+="${RED}✗${NC} ${label}
  file: ${YELLOW}${file}${NC}:${match%%:*}
  code: $(echo "$match" | cut -d: -f2-)
  fix:  ${suggestion}
"
      done
    fi
  done
}

# === Rule 1: no log.Printf / log.Println / log.Print ===
check_pattern \
  "log.Printf/Println/Print in business code" \
  '\blog\.(Print|Println|Printf|Fatal|Fatalf|Panic|Panicf)\(' \
  'use logx.FromContext(ctx).Info("...", logx.String("key", val))'

# === Rule 2: no fmt.Println / fmt.Printf / fmt.Print ===
check_pattern \
  "fmt.Println/Printf/Print in business code (use logx)" \
  '\bfmt\.(Print|Println|Printf|Fprint|Fprintln|Fprintf)\(' \
  'use logx.FromContext(ctx).Info("...", logx.String("key", val))'

# === Rule 3: no direct slog usage ===
check_pattern \
  "direct slog usage in business code" \
  '\bslog\.(Debug|Info|Warn|Error|Fatal|Panic|With|WithGroup|New|Default|Log|LogAttrs)\(' \
  'use logx.FromContext(ctx).Info(...) — slog bypasses Kernel logging contract'

# === Rule 4: no direct zap usage ===
check_pattern \
  "direct zap usage in business code" \
  '\bzap\.(L|S|New|Must|NewProduction|NewDevelopment)\(' \
  'use logx.FromContext(ctx).Info(...) — zap bypasses Kernel logging contract'

# === Rule 5: no direct logrus usage ===
check_pattern \
  "direct logrus usage in business code" \
  '\blogrus\.' \
  'use logx.FromContext(ctx).Info(...) — logrus bypasses Kernel logging contract'

# === Rule 6: no credentials in log fields ===
check_pattern \
  "credential in log field (redaction is a safety net, not a license)" \
  'logx\.String\("(password|passwd|pwd|token|secret|authorization|cookie|api_key|private_key)"' \
  'NEVER log credentials directly. Redaction auto-applies but best practice is to not log secrets at all.'

# === Report ===
if [ "$VIOLATIONS" -eq 0 ]; then
  echo "${GREEN}✓${NC} logx usage OK — ${#FILES[@]} business files checked, no violations"
  exit 0
fi

echo "${RED}✗${NC} logx usage check failed — $VIOLATIONS violation(s) found"
echo
echo "$REPORT"
echo "Read docs/ai/logx.md for the complete AI coding recipe."
echo
echo "Quick fix cheatsheet:"
echo "  log.Printf(\"x\")           → logx.FromContext(ctx).Info(\"x\")"
echo "  fmt.Println(\"x\")         → logx.FromContext(ctx).Info(\"x\")"
echo "  slog.Info(\"x\")           → logx.FromContext(ctx).Info(\"x\")"
echo "  zap.L().Info(\"x\")        → logx.FromContext(ctx).Info(\"x\")"
echo "  logger.Info(\"login\", logx.String(\"password\", pwd))  → DON'T LOG SECRETS"
exit 1
