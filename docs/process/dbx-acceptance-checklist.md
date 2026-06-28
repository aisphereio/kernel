# dbx Acceptance Checklist

## Static checks

- [ ] No import of `database/sql` in business code (handler/service/repository/worker).
- [ ] No import of `gorm.io/gorm` in business code (use `dbx.DB` / `dbx.Tx` interfaces; `db.GORM(ctx)` is the escape hatch but the import stays in dbx).
- [ ] No import of `gorm.io/driver/postgres` / `gorm.io/driver/mysql` in business code.
- [ ] No import of `github.com/jackc/pgx` / `github.com/lib/pq` / `github.com/go-sql-driver/mysql` in business code.
- [ ] No import of `github.com/jmoiron/sqlx` in business code.
- [ ] Root `README.md` links to `dbx/README.md`.
- [ ] `docs/ai/dbx.md` exists as the single AI guide.
- [ ] `docs/contracts/dbx.md` exists.
- [ ] `docs/design/dbx.md` exists.
- [ ] `dbx/README.md` is single-entry user guide (>200 lines, UTF-8 clean).
- [ ] `dbx/doc.go` exists with package-level doc.
- [ ] `examples/dbx-basic/` exists with README + main.go.
- [ ] `scripts/check-dbx-usage.sh` exists and is executable.

## Unit checks (no DB required)

```bash
go test ./dbx -count=1 -short
go test ./dbx -race -count=1 -short
go test ./dbx -cover -count=1 -short
```

Required behaviors:

- [ ] `New` returns `ErrNilConfig` when DSN is empty.
- [ ] `New` returns `ErrNilConfig` when Driver is empty.
- [ ] `New` returns `ErrUnknownDriver` for unregistered driver.
- [ ] All sentinel errors are matchable by `errors.Is` even when wrapped.
- [ ] `Config.Validate` returns `ErrNilConfig` for missing fields.
- [ ] `WithUnscoped` returns non-nil ctx.
- [ ] `InjectDB` / `InjectTx` do not panic.
- [ ] `RegisteredDrivers` returns a slice.
- [ ] `IsDriverRegistered` returns false for unknown driver.
- [ ] `RegisterDriver` panics on duplicate registration.
- [ ] `AssertAffected` returns `ErrNoEffect` when RowsAffected == 0 (verified via integration test).
- [ ] SafeUpsert returns `ErrUnsafeUpsert` when whitelist contains protected column.

## Integration checks (require Docker or DSN env vars)

### Postgres

```bash
# Option 1: testcontainers (needs Docker)
go test ./dbx -count=1 -run TestPG

# Option 2: external PG
KERNEL_DBX_PG_DSN=postgres://user:pass@localhost:5432/testdb?sslmode=disable \
  go test ./dbx -count=1 -run TestPG
```

Required PG scenarios:

- [ ] `TestPGCreateAndFindOne` — Create + FindOne by query
- [ ] `TestPGFindOneNoRows` — FindOne returns `ErrNoRows`
- [ ] `TestPGCreateDuplicateKey` — duplicate INSERT returns `ErrDuplicateKey`
- [ ] `TestPGSafeUpsert` — upsert updates whitelisted columns, leaves `owner_id` unchanged
- [ ] `TestPGSafeUpsertBlocksProtectedColumn` — whitelist with `owner_id` returns `ErrUnsafeUpsert`
- [ ] `TestPGSoftDelete` — Delete hides row; `WithUnscoped` reveals it
- [ ] `TestPGIncrement` — atomic counter increment
- [ ] `TestPGPaginate` — page 1 hasMore=true, page 3 hasMore=false
- [ ] `TestPGInTxCommit` — transaction commits on nil return
- [ ] `TestPGInTxRollback` — transaction rolls back on error return
- [ ] `TestPGInTxPanicRollback` — transaction rolls back on panic, re-panics
- [ ] `TestPGInjectDB` — `InjectDB` ctx picks up the injected `*gorm.DB`
- [ ] `TestPGSchemaNotReady` — missing table returns error (ideally `ErrSchemaNotReady`)

### MySQL

```bash
# Option 1: testcontainers (needs Docker)
go test ./dbx -count=1 -run TestMySQL

# Option 2: external MySQL
KERNEL_DBX_MYSQL_DSN='user:pass@tcp(localhost:3306)/testdb?parseTime=true&charset=utf8mb4' \
  go test ./dbx -count=1 -run TestMySQL
```

Required MySQL scenarios (mirror of PG):

- [ ] `TestMySQLCreateAndFindOne`
- [ ] `TestMySQLFindOneNoRows`
- [ ] `TestMySQLCreateDuplicateKey`
- [ ] `TestMySQLSafeUpsert`
- [ ] `TestMySQLSafeUpsertBlocksProtectedColumn`
- [ ] `TestMySQLSoftDelete`
- [ ] `TestMySQLIncrement`
- [ ] `TestMySQLPaginate`
- [ ] `TestMySQLInTxCommit`
- [ ] `TestMySQLInTxRollback`

## Coverage checks

```bash
go test ./dbx -cover -count=1 -short
```

Expected coverage:

- [ ] `dbx` package (unit tests only): >= 40%
- [ ] `dbx` package (with integration tests): >= 75%
- [ ] `dbx/postgres` package: trivial, no separate tests needed
- [ ] `dbx/mysql` package: trivial, no separate tests needed

## Example checks

```bash
go test ./dbx -run=Example -v
```

## Documentation checks

```bash
./scripts/check-module-docs.sh dbx
```

## Manual migration checks (from raw GORM / database/sql)

- [ ] Replace `gorm.Open(...)` with `dbx.New(dbx.Config{...})`.
- [ ] Replace `db.Where(...).First(&dest)` with `db.FindOne(ctx, &dest, query, args...)`.
- [ ] Replace `db.Where(...).Find(&dest)` with `db.FindMany(ctx, &dest, query, args...)`.
- [ ] Replace `db.Create(&row)` with `db.Create(ctx, &row)`.
- [ ] Replace `db.Save(&row)` with `db.Save(ctx, &row)`.
- [ ] Replace `db.Model(...).Where(...).Updates(map)` with `db.Update(ctx, &model, query, args, columns)`.
- [ ] Replace `db.Delete(...)` with `db.Delete(ctx, &model, query, args...)`.
- [ ] Replace `db.Transaction(func(tx *gorm.DB) error {...})` with `db.InTx(ctx, func(tx dbx.Tx) error {...})`.
- [ ] Replace `isUniqueViolation(err)` with `errors.Is(err, dbx.ErrDuplicateKey)`.
- [ ] Replace `gorm.ErrRecordNotFound` checks with `dbx.ErrNoRows`.
- [ ] Replace `Unscoped()` calls with `dbx.WithUnscoped(ctx)`.
- [ ] Replace `clause.OnConflict{ DoUpdates: ... }` with `db.SafeUpsert(row, allowedColumns)`.
- [ ] Replace `UpdateColumn(col, gorm.Expr("col + ?", delta))` with `db.Increment(...)`.
- [ ] Replace manual `limit+1` + `hasMore` pagination with `db.Paginate(...)`.
- [ ] Remove `r.db(ctx)` helpers — `db.GORM(ctx)` does it.
- [ ] Remove `mapXxxDBError` helpers — dbx auto-normalizes.
- [ ] All struct scan destinations have `gorm` tags with `column:`.

## Windows commands

Windows can use Make directly:

```powershell
make tools
make test-cmd
```

Or run the wrapper scripts:

```bat
scripts\tools.cmd
scripts\test-cmd.cmd
```
