#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
FAIL=0

echo "checking dbx usage under $ROOT"

# Forbidden imports in business code.
# Note: gorm.io/gorm is allowed in dbx/ itself and in test files (for
# gorm.DeletedAt tag), but NOT in business code (handler/service/repository).
FORBIDDEN_PATTERNS=(
    '"database/sql"'
    '"github.com/jmoiron/sqlx"'
    '"gorm.io/driver/postgres"'
    '"gorm.io/driver/mysql"'
    '"github.com/jackc/pgx/v5"'
    '"github.com/jackc/pgx/v4"'
    '"github.com/lib/pq"'
    '"github.com/go-sql-driver/mysql"'
)

# Directories that are allowed to import drivers (registration only).
ALLOWED_DIRS=(
    "dbx/postgres"
    "dbx/mysql"
    "examples/dbx-basic"  # example imports _ "dbx/postgres" for registration
    "cmd/kernel"          # CLI tooling may import for migration helpers
    "internal/testdata"
)

# Find business code directories (handler / service / repository / worker / biz / data).
TARGETS=()
for d in internal/biz internal/service internal/data handler service repository worker biz services; do
    if [ -d "$ROOT/$d" ]; then
        TARGETS+=("$ROOT/$d")
    fi
done

if [ ${#TARGETS[@]} -eq 0 ]; then
    echo "no business directories found; skipping forbidden-import check"
else
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        for tgt in "${TARGETS[@]}"; do
            # Skip allowed subdirectories inside business dirs (rare).
            skip=0
            for allowed in "${ALLOWED_DIRS[@]}"; do
                if [[ "$tgt" == *"$allowed"* ]]; then
                    skip=1
                    break
                fi
            done
            if [ "$skip" -eq 1 ]; then
                continue
            fi
            if grep -R --include='*.go' -n "$pattern" "$tgt" 2>/dev/null; then
                echo "error: forbidden dbx bypass import $pattern found in $tgt" >&2
                FAIL=1
            fi
        done
    done

    # Also check for direct gorm.io/gorm import in business code.
    # gorm.DeletedAt tag usage is allowed (it's a type reference, not a DB
    # connection), but gorm.Open / gorm.DB direct usage is forbidden.
    for tgt in "${TARGETS[@]}"; do
        if grep -R --include='*.go' -n 'gorm\.Open\|gorm\.DB{' "$tgt" 2>/dev/null; then
            echo "error: direct gorm.Open or gorm.DB struct usage found in $tgt (use dbx.New)" >&2
            FAIL=1
        fi
    done
fi

# Ensure driver is registered before use (best-effort check).
if [ -f "$ROOT/main.go" ] || [ -f "$ROOT/cmd/main.go" ]; then
    if ! grep -R --include='*.go' -l 'dbx/postgres\|dbx/mysql' "$ROOT" 2>/dev/null | grep -q .; then
        : # Not an error — many subprojects don't use dbx.
    fi
fi

if [ "$FAIL" -ne 0 ]; then
    exit 1
fi

echo "dbx usage check passed"
