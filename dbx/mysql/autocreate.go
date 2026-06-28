// Package mysql autocreate.go — auto-create target database when missing.
//
// Used by open() when Config.AutoCreateDatabase is true and the initial
// gorm.Open fails with MySQL error 1049 (ER_BAD_DB_ERROR). The flow:
//
//  1. Parse the original DSN into (admin DSN without dbname, target
//     database name).
//  2. Connect to the admin DSN.
//  3. Check information_schema.schemata for the target name (idempotency).
//  4. If missing, exec CREATE DATABASE `<name>` (CREATE DATABASE cannot
//     run inside a transaction; we use *sql.DB.Exec directly).
//  5. Close the admin connection. Caller retries the original gorm.Open.
//
// DSN format follows go-sql-driver/mysql conventions:
//
//   [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...]
//
// The database name is the path segment after the leading slash. To build
// the admin DSN, we strip the dbname segment (and any trailing path), so
// the connection is made without a default database — MySQL allows this
// and the user must have CONNECT privilege on the server.

package mysql

import (
	"errors"
	"fmt"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// isDatabaseNotExist reports whether err is a MySQL "database does not
// exist" error (ER_BAD_DB_ERROR, number 1049). Also falls back to
// message-string matching for wrapped errors.
func isDatabaseNotExist(err error) bool {
	if err == nil {
		return false
	}
	var myErr *mysqldriver.MySQLError
	if errors.As(err, &myErr) {
		return myErr.Number == 1049
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "error 1049") ||
		strings.Contains(lower, "unknown database")
}

// createDatabase connects to the MySQL server without a default database
// and issues CREATE DATABASE `<name>` when the target does not yet exist.
//
// originalDSN is the user-supplied DSN that failed with 1049. We parse it
// to extract (a) the target database name and (b) an admin DSN with no
// default database.
//
// The CREATE DATABASE statement:
//   - Uses backtick-quoted identifier to prevent SQL injection.
//   - Does NOT specify CHARACTER SET, COLLATE, ENCRYPTION, or other
//     options. The target inherits server defaults. Operators who need
//     custom charset must pre-provision the database out-of-band and
//     leave Config.AutoCreateDatabase false.
//   - Cannot run inside a transaction. We use *sql.DB.Exec directly.
//
// The function is idempotent: if information_schema.schemata already
// contains the target name, it returns nil without issuing CREATE DATABASE.
func createDatabase(originalDSN string) error {
	adminDSN, dbName, err := parseForAdmin(originalDSN)
	if err != nil {
		return fmt.Errorf("parse DSN for auto-create: %w", err)
	}

	adminDB, err := gorm.Open(mysql.Open(adminDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connect to mysql server without default database: %w", err)
	}

	// CREATE DATABASE cannot run inside a transaction. Use the underlying
	// *sql.DB to bypass GORM's transaction wrapping.
	sqlDB, err := adminDB.DB()
	if err != nil {
		return fmt.Errorf("acquire underlying sql.DB: %w", err)
	}

	defer func() { _ = sqlDB.Close() }()

	// Idempotency check.
	var exists bool
	if err := adminDB.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = ?)", dbName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}
	if exists {
		return nil
	}

	stmt := fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbName))
	if _, err := sqlDB.Exec(stmt); err != nil {
		return fmt.Errorf("execute %q: %w", stmt, err)
	}
	return nil
}

// parseForAdmin splits the original DSN into (admin DSN, target database
// name). The admin DSN has no default database.
//
// go-sql-driver/mysql DSN format:
//
//	[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...]
//
// Examples:
//
//	user:pass@tcp(host:3306)/mydb?charset=utf8mb4
//	    admin: user:pass@tcp(host:3306)/?charset=utf8mb4
//	    db:    mydb
//
//	user@unix(/tmp/mysql.sock)/mydb
//	    admin: user@unix(/tmp/mysql.sock)/
//	    db:    mydb
//
// Returns an error if the DSN has no dbname segment (e.g. "user:pass@tcp(host:3306)"
// with no trailing slash). In that case the connection would succeed without
// a default database anyway, so the original gorm.Open would not have hit
// 1049 and we would not be here.
func parseForAdmin(originalDSN string) (adminDSN, dbName string, err error) {
	trimmed := strings.TrimSpace(originalDSN)
	if trimmed == "" {
		return "", "", fmt.Errorf("empty DSN")
	}
	// Find the first '/' after the optional "user[:pass]@proto(addr)" prefix.
	// The slash separates the address part from the dbname part.
	slash := strings.Index(trimmed, "/")
	if slash < 0 {
		return "", "", fmt.Errorf("DSN has no '/' separator (expected user@proto(addr)/dbname): %s", trimmed)
	}
	prefix := trimmed[:slash+1] // includes the trailing slash
	rest := trimmed[slash+1:]   // "dbname?params" or "?params" or ""

	// rest is "dbname?params" or "?params" or "dbname" or ""
	var params string
	// Split dbname from params at the first '?'.
	q := strings.Index(rest, "?")
	if q >= 0 {
		dbName = rest[:q]
		params = rest[q:] // includes the '?'
	} else {
		dbName = rest
		params = ""
	}
	if dbName == "" {
		// DSN was "user@host/?" — no dbname. But then the original gorm.Open
		// would not have hit 1049 (no default database means MySQL does not
		// try to USE a database). This branch should never execute in normal
		// flow; we return an error so the caller sees the mismatch.
		return "", "", fmt.Errorf("DSN has no database name after '/' (auto-create cannot proceed without a target name): %s", trimmed)
	}

	// Admin DSN: prefix + params (no dbname between slash and '?').
	adminDSN = prefix + params
	return adminDSN, dbName, nil
}

// quoteIdentifier wraps a MySQL identifier in backticks, doubling any
// embedded backticks per MySQL identifier-quoting rules. This prevents
// SQL injection even when the identifier comes from an untrusted source.
//
// Example: my`db -> `my“db`
func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
