package mysql

import (
	"time"

	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/logx"
	"gorm.io/gorm"
)

// registerCallbacks wires dbx's pre-actions onto the GORM instance:
//   - slow-query logging (if SlowQueryThreshold > 0)
//   - debug logging (if Debug)
//
// This mirrors dbx/postgres/callbacks.go; the implementations are
// duplicated to keep driver subpackages independent. A future refactor
// could move this to a shared dbx/callbacks package if it grows.
func registerCallbacks(gormDB *gorm.DB, cfg dbx.Config) {
	if cfg.SlowQueryThreshold > 0 || cfg.Debug {
		registerQueryLogger(gormDB, cfg)
	}
}

func registerQueryLogger(gormDB *gorm.DB, cfg dbx.Config) {
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

		if cfg.SlowQueryThreshold > 0 && elapsed > cfg.SlowQueryThreshold {
			logx.Warn("slow db query",
				"driver", "mysql",
				"elapsed_ms", elapsed.Milliseconds(),
				"sql", summarizeSQL(tx.Statement.SQL.String()),
				"rows_affected", tx.Statement.RowsAffected,
				"error", tx.Error,
			)
		}

		if cfg.Debug {
			logx.Debug("db query",
				"driver", "mysql",
				"elapsed_ms", elapsed.Milliseconds(),
				"sql", summarizeSQL(tx.Statement.SQL.String()),
				"rows_affected", tx.Statement.RowsAffected,
			)
		}
	}

	gormDB.Callback().Create().After("gorm:create").Register("dbx:after_create", afterFn)
	gormDB.Callback().Query().After("gorm:query").Register("dbx:after_query", afterFn)
	gormDB.Callback().Update().After("gorm:update").Register("dbx:after_update", afterFn)
	gormDB.Callback().Delete().After("gorm:delete").Register("dbx:after_delete", afterFn)
}

func summarizeSQL(sql string) string {
	const maxLen = 500
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "...(truncated)"
}

// ensure dbx import is referenced.
var _ = dbx.Config{}
