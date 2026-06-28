# dbx-basic

Minimal runnable demo for `dbx` with Postgres, mirroring aisphere-hub's
skill.go patterns.

Requires a running Postgres instance.

## Run

```bash
export KERNEL_DBX_PG_DSN=postgres://user:pass@localhost:5432/testdb?sslmode=disable
go run ./examples/dbx-basic
```

Expected output:

```text
opened db (driver=postgres)
migrated dbx_basic_skills table
created skill id=1 name=demo owner=user_123
found skill id=1 name=demo owner=user_123
ErrNoRows handled: skill 'nope' correctly reported missing
upserted skill: display_name=Demo V2, owner_id=user_123 (unchanged)
ErrUnsafeUpsert handled: owner_id correctly blocked from whitelist
incremented download_count to 5
paginated: page=1 size=10 total=12 hasMore=true
transaction committed: 2 skills created
soft-deleted skill demo
normal query correctly hides soft-deleted skill
unscoped query found soft-deleted skill: demo
dropped table dbx_basic_skills
done
```

If `KERNEL_DBX_PG_DSN` is not set, the example prints usage hints and exits
without trying to connect.

## What it demonstrates

- Constructing `dbx.Config` and calling `dbx.New`
- Registering the `postgres` driver via `_ "github.com/aisphereio/kernel/dbx/postgres"`
- `AutoMigrate` (dev only) for schema setup
- `Create` with auto-generated primary key
- `FindOne` for single-row struct scan
- `ErrNoRows` handling for missing rows
- `SafeUpsert` with whitelist (owner_id is protected, never overwritten)
- `ErrUnsafeUpsert` when whitelist contains a protected column
- `Increment` for atomic counter
- `Paginate` with hasMore + total
- `InTx` callback-style transaction (auto commit on nil return)
- Soft delete (auto-filters deleted_at)
- `dbx.WithUnscoped(ctx)` to opt-in to reading soft-deleted rows
- `dbx.ErrDuplicateKey` detection
- `defer db.Close()` for resource cleanup

For the full pattern including configx integration, repository-layer error
conversion to errorx, and multi-repo transaction sharing, see
`docs/ai/dbx.md`.
