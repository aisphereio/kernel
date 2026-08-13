package migrationx

import (
	"context"
	"fmt"
	"os"

	"github.com/aisphereio/kernel/dbx"
	"github.com/pressly/goose/v3"
)

// applyGoose delegates SQL parsing and execution to the real goose provider.
// Kernel deliberately does not split SQL statements here: migrations may use
// functions, triggers, procedural blocks, views, or other dialect-specific SQL
// that must retain exact database semantics.
func applyGoose(ctx context.Context, db dbx.DB, cfg Config) error {
	gormDB := db.GORM(ctx)
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("migrationx: get sql db: %w", err)
	}

	dialect, err := gooseDialect(db.DriverName())
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(
		dialect,
		sqlDB,
		os.DirFS(cfg.Dir),
		goose.WithTableName(cfg.Table),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("migrationx: create goose provider: %w", err)
	}

	switch cfg.Mode {
	case ModeValidate:
		pending, err := provider.HasPending(ctx)
		if err != nil {
			return fmt.Errorf("migrationx: validate goose migrations: %w", err)
		}
		if pending && cfg.FailOnPending {
			return fmt.Errorf("migrationx: pending migrations")
		}
		return nil
	case ModeApply, ModeDevApply:
		if _, err := provider.Up(ctx); err != nil {
			return fmt.Errorf("migrationx: apply goose migrations: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("migrationx: goose does not support mode %q", cfg.Mode)
	}
}

func gooseDialect(driver string) (goose.Dialect, error) {
	switch driver {
	case "postgres", "pgx":
		return goose.DialectPostgres, nil
	case "mysql":
		return goose.DialectMySQL, nil
	default:
		return "", fmt.Errorf("migrationx: goose does not support dbx driver %q", driver)
	}
}
