package postgres

import (
	"time"

	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/logx"
	"gorm.io/gorm"
)

// registerCallbacks wires dbx's pre-actions onto the GORM instance:
//   - slow-query logging (if SlowQueryThreshold > 0)
//   - debug logging (if Debug)
//   - metrics (if MetricsEnabled) — TODO: requires metricsx; placeholder for now
//
// Audit hook is implemented separately in audit.go because it requires
// access to the auditx package, which may not be available in all builds.
func registerCallbacks(gormDB *gorm.DB, cfg dbx.Config) {
	// Slow-query + debug logging via After callbacks.
	if cfg.SlowQueryThreshold > 0 || cfg.Debug {
		registerQueryLogger(gormDB, cfg)
	}

	// Metrics are recorded by dbx's public operation wrapper. Direct GORM escape-hatch
	// queries still get slow/debug logs through these callbacks.

	// Audit: registered in audit.go via init() if Config.AuditEnabled.
}

// queryLog holds per-query timing info.
type queryLog struct {
	start   time.Time
	sqlText string
}

// registerQueryLogger installs Before/After callbacks that log SQL.
//
// We use a per-query goroutine-local map keyed by gorm.DB instance pointer
// to avoid races. This is acceptable because GORM sessions are short-lived
// and single-goroutine.
//
// Note: GORM does not expose a unique per-query ID; we approximate by
// measuring elapsed time in the After callback and comparing to the
// Before timestamp. This is not perfectly accurate but is good enough
// for slow-query logging.
func registerQueryLogger(gormDB *gorm.DB, cfg dbx.Config) {
	// We use GORM's Logger interface instead of callbacks for cleaner
	// SQL text. Override the silent logger set in open().
	//
	// Re-create the gorm.Config with a custom logger and re-open? No —
	// we already have a *gorm.DB. We can swap its logger via Session.
	//
	// The simplest approach: install a Callback that captures timing.
	gormDB.Callback().Create().Before("gorm:create").Register("dbx:before_create", func(tx *gorm.DB) {
		tx.InstanceSet("dbx:start_time", time.Now())
	})
	gormDB.Callback().Query().Before("gorm:query").Register("dbx:before_query", func(tx *gorm.DB) {
		tx.InstanceSet("dbx:start_time", time.Now())
	})
	gormDB.Callback().Update().Before("gorm:update").Register("dbx:before_update", func(tx *gorm.DB) {
		tx.InstanceSet("dbx:start_time", time.Now())
	})
	gormDB.Callback().Delete().Before("gorm:delete").Register("dbx:before_delete", func(tx *gorm.DB) {
		tx.InstanceSet("dbx:start_time", time.Now())
	})

	afterFn := func(tx *gorm.DB) {
		startAny, _ := tx.InstanceGet("dbx:start_time")
		start, ok := startAny.(time.Time)
		if !ok {
			return
		}
		elapsed := time.Since(start)

		// Slow query.
		logger := cfg.Logger
		if logger == nil {
			logger = logx.DefaultLogger()
		}
		logger = logger.Named("dbx.postgres")

		if cfg.SlowQueryThreshold > 0 && elapsed > cfg.SlowQueryThreshold {
			logger.Warn("slow db query",
				logx.String("driver", "postgres"),
				logx.Duration("elapsed", elapsed),
				logx.String("sql", summarizeSQL(tx.Statement.SQL.String())),
				logx.Int64("rows_affected", tx.Statement.RowsAffected),
				logx.Err(tx.Error),
			)
		}

		// Debug logging (every query).
		if cfg.Debug {
			logger.Debug("db query",
				logx.String("driver", "postgres"),
				logx.Duration("elapsed", elapsed),
				logx.String("sql", summarizeSQL(tx.Statement.SQL.String())),
				logx.Int64("rows_affected", tx.Statement.RowsAffected),
			)
		}
	}

	gormDB.Callback().Create().After("gorm:create").Register("dbx:after_create", afterFn)
	gormDB.Callback().Query().After("gorm:query").Register("dbx:after_query", afterFn)
	gormDB.Callback().Update().After("gorm:update").Register("dbx:after_update", afterFn)
	gormDB.Callback().Delete().After("gorm:delete").Register("dbx:after_delete", afterFn)
}

// summarizeSQL truncates SQL to a reasonable length for logging.
// 500 chars is enough to see the query shape without bloating log lines.
func summarizeSQL(sql string) string {
	const maxLen = 500
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "...(truncated)"
}
