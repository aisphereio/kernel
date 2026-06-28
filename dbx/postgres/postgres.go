// Package postgres registers the "postgres" driver for dbx.
//
// Import this package once in your main function to enable postgres:
//
//	import _ "github.com/aisphereio/kernel/dbx/postgres"
//
// Then use `dbx.Config{Driver: "postgres", DSN: "postgres://..."}` with dbx.New.
//
// The driver uses gorm.io/driver/postgres, which wraps pgx/stdlib. It
// supports all GORM features including OnConflict, soft-delete, and
// autoCreate/UpdateTime.
package postgres

import (
	"errors"
	"strings"

	"github.com/aisphereio/kernel/dbx"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const driverName = "postgres"

func init() {
	dbx.RegisterDriver(driverName, open)
	dbx.RegisterErrorMapper(mapError)
}

// open returns a *gorm.DB backed by gorm.io/driver/postgres.
func open(dsn string, cfg dbx.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
		DryRun:                                   cfg.DryRun,
	}

	gormDB, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	} else if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxOpenConns / 2)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	registerCallbacks(gormDB, cfg)

	return gormDB, nil
}

// mapError inspects a postgres error and returns the matching dbx sentinel.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Typed path: errors.As against *pgconn.PgError.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return dbx.ErrDuplicateKey
		case "23503":
			return dbx.ErrForeignKeyViolation
		case "42P01":
			return dbx.ErrSchemaNotReady
		}
	}

	// Fallback: message-string matching (covers wrapped errors and
	// drivers that lose the typed error).
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "duplicate key value violates unique constraint"),
		strings.Contains(lower, "sqlstate 23505"):
		return dbx.ErrDuplicateKey
	case strings.Contains(lower, "violates foreign key constraint"),
		strings.Contains(lower, "sqlstate 23503"):
		return dbx.ErrForeignKeyViolation
	case strings.Contains(lower, "relation") && strings.Contains(lower, "does not exist"),
		strings.Contains(lower, "sqlstate 42p01"):
		return dbx.ErrSchemaNotReady
	}

	return nil
}
