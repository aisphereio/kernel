// Package mysql registers the "mysql" driver for dbx.
//
// Import this package once in your main function to enable mysql:
//
//	import _ "github.com/aisphereio/kernel/dbx/mysql"
//
// Then use `dbx.Config{Driver: "mysql", DSN: "user:pass@tcp(host:3306)/db"}` with dbx.New.
//
// The driver uses gorm.io/driver/mysql, which wraps go-sql-driver/mysql.
// It supports all GORM features including OnConflict (translated to
// INSERT ... ON DUPLICATE KEY UPDATE), soft-delete, and autoCreate/UpdateTime.
package mysql

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/dbx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const driverName = "mysql"

func init() {
	dbx.RegisterDriver(driverName, open)
	dbx.RegisterErrorMapper(mapError)
}

// open returns a *gorm.DB backed by gorm.io/driver/mysql.
//
// When cfg.AutoCreateDatabase is true and the initial connection fails
// because the target database does not exist (MySQL error 1049
// ER_BAD_DB_ERROR), open calls createDatabase to provision it, then
// retries the connection exactly once. Any other error (auth, network,
// wrong host) is returned without retry.
func open(dsn string, cfg dbx.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
		DryRun:                                   cfg.DryRun,
	}

	gormDB, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		if !cfg.AutoCreateDatabase || !isDatabaseNotExist(err) {
			return nil, err
		}
		if createErr := createDatabase(dsn); createErr != nil {
			return nil, fmt.Errorf("dbx: auto-create database failed: %w (original connect error: %v)", createErr, err)
		}
		gormDB, err = gorm.Open(mysql.Open(dsn), gormCfg)
		if err != nil {
			return nil, err
		}
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

// mapError inspects a mysql error and returns the matching dbx sentinel.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Typed path: errors.As against *mysql.MySQLError.
	var myErr *mysqldriver.MySQLError
	if errors.As(err, &myErr) {
		switch myErr.Number {
		case 1062:
			// ER_DUP_ENTRY
			return dbx.ErrDuplicateKey
		case 1452:
			// ER_NO_REFERENCED_ROW_2 (foreign key violation)
			return dbx.ErrForeignKeyViolation
		case 1146:
			// ER_NO_SUCH_TABLE
			return dbx.ErrSchemaNotReady
		case 1049:
			// ER_BAD_DB_ERROR
			return dbx.ErrDatabaseNotExist
		}
	}

	// Fallback: message-string matching.
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "duplicate entry"),
		strings.Contains(lower, "error 1062"):
		return dbx.ErrDuplicateKey
	case strings.Contains(lower, "foreign key constraint fails"),
		strings.Contains(lower, "error 1452"):
		return dbx.ErrForeignKeyViolation
	case strings.Contains(lower, "doesn't exist"),
		strings.Contains(lower, "error 1146"):
		return dbx.ErrSchemaNotReady
	case strings.Contains(lower, "error 1049"),
		strings.Contains(lower, "unknown database"):
		return dbx.ErrDatabaseNotExist
	}

	return nil
}
