# dbx-scenarios

Comprehensive scenario validator for `dbx`. Exercises every advertised
capability end-to-end against a real database and prints PASS/FAIL for
each scenario.

## Run

### Postgres

```bash
export KERNEL_DBX_PG_DSN=postgres://user:pass@localhost:5432/testdb?sslmode=disable
go run ./examples/dbx-scenarios
```

### MySQL

```bash
export KERNEL_DBX_DRIVER=mysql
export KERNEL_DBX_MYSQL_DSN='user:pass@tcp(localhost:3306)/testdb?parseTime=true&charset=utf8mb4'
go run ./examples/dbx-scenarios
```

## What it validates (28 scenarios)

| # | Scenario | Validates |
|---:|---|---|
| 01 | open + ping | `dbx.New` + `PingContext` |
| 02 | AutoMigrate | `AutoMigrate` creates tables |
| 03 | Create + auto PK | `Create` fills auto-increment PK |
| 04 | FindOne by query | `FindOne(ctx, &dest, "name = ?", ...)` |
| 05 | FindOneByPK | `FindOneByPK(ctx, &dest, pk)` |
| 06 | FindMany | `FindMany(ctx, &slice, query, args...)` |
| 07 | Count | `Count(ctx, model, query, args...)` |
| 08 | ErrNoRows | `gorm.ErrRecordNotFound` → `dbx.ErrNoRows` |
| 09 | ErrDuplicateKey | PG 23505 / MySQL 1062 → `dbx.ErrDuplicateKey` |
| 10 | SafeUpsert whitelist | `SafeUpsert` updates whitelisted cols, leaves `owner_id` |
| 11 | SafeUpsert blocks protected | `owner_id` in whitelist → `ErrUnsafeUpsert` |
| 12 | Soft delete hides row | `Delete` soft-deletes; normal query returns `ErrNoRows` |
| 13 | WithUnscoped reveals row | `dbx.WithUnscoped(ctx)` sees soft-deleted rows |
| 14 | Increment atomic | `Increment` does `col = col + delta` server-side |
| 15 | Paginate hasMore+total | `Paginate` returns correct page/total/hasMore |
| 16 | InTx commit | `InTx(fn)` commits on nil return |
| 17 | InTx rollback on error | `InTx(fn)` rolls back on error return |
| 18 | InTx rollback on panic | `InTx(fn)` rolls back on panic, re-panics |
| 19 | Context propagation | `InjectDB(ctx, gormDB)` picked up by `GORM(ctx)` |
| 20 | GORM escape hatch | `db.GORM(ctx).Raw(...)` for complex joins |
| 21 | QueryTimeout fires | `Config.QueryTimeout` cancels long queries |
| 22 | Slow query log | `Config.SlowQueryThreshold` logs slow queries |
| 23 | Debug mode | `Config.Debug=true` logs every query |
| 24 | Update partial | `Update(ctx, model, query, args, columns)` |
| 25 | Save full row | `Save(ctx, &row)` full insert-or-update |
| 26 | DeleteByPK | `DeleteByPK(ctx, model, pk)` |
| 27 | AssertAffected ErrNoEffect | `AssertAffected(res)` → `ErrNoEffect` on 0 rows |
| 28 | nested InTx | Nested `InTx` reuses outer tx (no double-commit) |

## Expected output

```text
=== dbx scenario validator ===
driver=postgres dsn=postgres://user:***@localhost:5432/testdb

[PASS] open + ping
[PASS] ping
[PASS] AutoMigrate
[PASS] Create + auto PK
[PASS]   auto PK set
[PASS] FindOne by query
[PASS]   name matches
...
[PASS] nested InTx reuses outer tx
[PASS]   both creates ran
[PASS]   2 rows committed (no double-commit)

=== Summary: 28/28 PASS, 0 FAIL ===
```

If any scenario fails, the output shows `[FAIL]` with the error, and the
exit code is 1.

## What this example does NOT validate

- **Audit hook** — `Config.AuditEnabled` is a placeholder (TODO in
  `dbx/postgres/callbacks.go`); no auditx integration yet.
- **Metrics** — `Config.MetricsEnabled` is a placeholder; no metricsx
  integration yet.
- **Preload/Relations** — dbx does not wrap GORM's Preload; use
  `db.GORM(ctx).Preload(...)` escape hatch (scenario 20 demonstrates
  Raw join as a proxy).

These are known gaps in dbx v2; see `docs/design/dbx.md` § 8 for the
roadmap.
