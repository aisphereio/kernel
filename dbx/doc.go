// Package dbx provides the unified, batteries-included database API for
// Aisphere Kernel.
//
// dbx is the ONLY database abstraction that business code (handler / service /
// repository / worker) should depend on. Direct usage of `database/sql`,
// `gorm.io/gorm` (the *gorm.DB type), or driver-specific packages in business
// code is forbidden and will fail CI.
//
// # Design principle
//
// dbx exposes a stable DB / Tx interface backed by GORM, with eight
// "pre-actions" built in so business code does not have to repeat the same
// boilerplate in every repository method:
//
//  1. Error normalization — gorm.ErrRecordNotFound, pgconn.PgError 23505,
//     mysql 1062 are automatically wrapped into dbx.ErrNoRows /
//     dbx.ErrDuplicateKey so business code can use errors.Is without
//     importing driver packages.
//  2. Soft-delete safety — Unscoped queries require an explicit
//     WithUnscoped(ctx) opt-in; accidental "select deleted rows" is blocked.
//  3. OnConflict whitelist — SafeUpsert(model, row, allowedColumns) refuses
//     to overwrite sensitive columns (owner_id / visibility / status / ...).
//  4. Context propagation — DBFromContext(ctx) returns the request-scoped
//     *gorm.DB (with tx / trace / request_id); InjectDB injects it.
//     Repository methods stop repeating the same db(ctx) helper.
//  5. Audit hook — BeforeCreate / AfterUpdate / BeforeDelete callbacks
//     automatically record audit events via auditx; no per-method plumbing.
//  6. Global QueryTimeout — Config.QueryTimeout applies to every query
//     whose context lacks a deadline; no per-query WithTimeout boilerplate.
//  7. Slow-query log — queries exceeding SlowQueryThreshold (default 200ms)
//     are logged via logx with SQL text, duration, and caller.
//  8. Metrics — every query is recorded as a Prometheus histogram with
//     labels driver / operation / status; no manual instrumentation.
//
// dbx does NOT depend on errorx (avoids cyclic dependency); business code
// converts dbx errors to errorx at the repository layer boundary.
//
// # 30-second quickstart
//
//	import (
//	    "github.com/aisphereio/kernel/configx"
//	    "github.com/aisphereio/kernel/configx/file"
//	    "github.com/aisphereio/kernel/dbx"
//	    _ "github.com/aisphereio/kernel/dbx/postgres" // register "postgres" driver
//	)
//
//	var dbCfg dbx.Config
//	_ = cfg.Value("database").Scan(&dbCfg)
//
//	db, err := dbx.New(dbCfg)
//	if err != nil { return err }
//	defer db.Close()
//
//	// Create with auto audit + soft-delete + error normalization.
//	skill := &Skill{Name: "demo", OwnerID: "user_123"}
//	if err := db.Create(ctx, skill); err != nil {
//	    if errors.Is(err, dbx.ErrDuplicateKey) {
//	        return errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在")
//	    }
//	    return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED", ...)
//	}
//
//	// Find one (auto filters soft-deleted rows).
//	var skill Skill
//	if err := db.FindOne(ctx, &skill, "name = ?", "demo"); err != nil {
//	    if errors.Is(err, dbx.ErrNoRows) {
//	        return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在")
//	    }
//	    return errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED", ...)
//	}
//
//	// Safe upsert: whitelist allowed columns; owner_id never overwritten.
//	if err := db.SafeUpsert(ctx, &skill, []string{"display_name", "version"}); err != nil { ... }
//
//	// Transaction with auto audit.
//	err = db.InTx(ctx, func(tx dbx.Tx) error {
//	    if err := tx.Create(ctx, skillRow); err != nil { return err }
//	    if err := tx.Create(ctx, versionRow); err != nil { return err }
//	    return nil // commit
//	})
//
// # Drivers
//
// dbx does not import any driver by default. Import the driver subpackage
// to register it:
//
//	import _ "github.com/aisphereio/kernel/dbx/postgres"  // registers "postgres"
//	import _ "github.com/aisphereio/kernel/dbx/mysql"     // registers "mysql"
//
// After import, set Config.Driver = "postgres" (or "mysql") and dbx picks
// the registered driver automatically.
//
// # Safe helpers
//
// dbx provides semantic helpers that encode aisphere-hub's hard-won security
// lessons so they cannot be re-broken by a careless copy-paste:
//
//	db.SafeUpsert(ctx, row, allowedColumns)   // OnConflict + whitelist
//	db.WithUnscoped(ctx)                       // opt-in to read soft-deleted
//	db.AssertAffected(res, err)                // 0 rows → ErrNoRows
//	db.Increment(ctx, model, where, column, delta)  // atomic counter
//	db.Paginate(ctx, model, page, size, &out)  // limit+1 + hasMore + total
//
// # Forbidden patterns
//
// Do not import `database/sql` in business code. Do not import driver
// packages (`gorm.io/driver/postgres`, `gorm.io/driver/mysql`) in business
// code. Do not import `gorm.io/gorm` directly in business code (use dbx.DB
// and dbx.Tx interfaces). Do not call Unscoped without WithUnscoped.
//
// # Documentation
//
// See dbx/README.md for the user guide, docs/ai/dbx.md for the AI coding
// recipe, docs/contracts/dbx.md for behaviors that cannot change without a
// major version bump.
package dbx
