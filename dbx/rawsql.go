package dbx

import (
	"context"
	"database/sql"
)

// ExecSQL executes raw SQL through the canonical dbx entrypoint.
//
// This is the supported direct-SQL escape hatch for generated model/RPC code
// and hot paths. It intentionally lives in dbx instead of a separate top-level
// sqlx package so Kernel has one database entrypoint.
func ExecSQL(ctx context.Context, db DB, query string, args ...any) error {
	if db == nil {
		return ErrNilDB
	}
	return wrapDriverErr(db.GORM(ctx).Exec(query, args...).Error)
}

// ScanSQL executes a raw SELECT and scans the result into dest.
func ScanSQL(ctx context.Context, db DB, dest any, query string, args ...any) error {
	if db == nil {
		return ErrNilDB
	}
	return wrapDriverErr(db.GORM(ctx).Raw(query, args...).Scan(dest).Error)
}

// RowsSQL executes a raw SELECT and returns database/sql rows. The caller must
// close the returned rows.
func RowsSQL(ctx context.Context, db DB, query string, args ...any) (*sql.Rows, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	rows, err := db.GORM(ctx).Raw(query, args...).Rows()
	return rows, wrapDriverErr(err)
}

// RowSQL executes a raw SELECT expected to return one row.
func RowSQL(ctx context.Context, db DB, query string, args ...any) *sql.Row {
	return db.GORM(ctx).Raw(query, args...).Row()
}

// InSQLTx runs fn inside the canonical dbx transaction path. Raw SQL helpers
// called with the supplied txCtx automatically use the same transaction because
// dbx injects Tx into context.
func InSQLTx(ctx context.Context, db DB, fn func(txCtx context.Context) error) error {
	if db == nil {
		return ErrNilDB
	}
	if fn == nil {
		return ErrNilTxFunc
	}
	return db.InTx(ctx, func(tx Tx) error {
		return fn(InjectTx(ctx, tx))
	})
}
