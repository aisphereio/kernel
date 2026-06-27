#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
FAIL=0

echo "checking configx usage under $ROOT"

if grep -R --include='*.go' -n 'github.com/aisphereio/kernel/config"' "$ROOT" \
  --exclude-dir=.git --exclude-dir=vendor 2>/dev/null; then
  echo "error: old config import path found; use github.com/aisphereio/kernel/configx" >&2
  FAIL=1
fi

if grep -R --include='*.go' -n 'github.com/aisphereio/kernel/config/' "$ROOT" \
  --exclude-dir=.git --exclude-dir=vendor 2>/dev/null; then
  echo "error: old config subpackage import path found; use configx/file or configx/env" >&2
  FAIL=1
fi

# os.Getenv is forbidden in business code, but allowed in source adapters,
# command-line tooling, and contrib integrations. Only scan conventional
# business directories if they exist.
TARGETS=()
for d in internal/biz internal/service internal/data handler service repository worker biz services; do
  if [ -d "$ROOT/$d" ]; then
    TARGETS+=("$ROOT/$d")
  fi
done

if [ "${#TARGETS[@]}" -gt 0 ]; then
  if grep -R --include='*.go' -n 'os\.Getenv' "${TARGETS[@]}" \
    --exclude='*_test.go' 2>/dev/null; then
    echo "error: os.Getenv found in business Go code; use configx/env source and configx.Value" >&2
    FAIL=1
  fi
else
  echo "no conventional business directories found; skipped os.Getenv business scan"
fi

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

echo "configx usage check passed"
