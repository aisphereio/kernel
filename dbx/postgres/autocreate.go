// Package postgres autocreate.go — auto-create target database when missing.
//
// Used by open() when Config.AutoCreateDatabase is true and the initial
// gorm.Open fails with PG SQLSTATE 3D000 (invalid_catalog_name). The flow:
//
//  1. Parse the original DSN into (admin DSN pointing at the "postgres"
//     database, target database name).
//  2. Connect to the admin DSN.
//  3. Check pg_database for the target name (idempotency).
//  4. If missing, exec CREATE DATABASE "<name>" (CREATE DATABASE cannot
//     run inside a transaction; we use *sql.DB.Exec directly).
//  5. Close the admin connection. Caller retries the original gorm.Open.
//
// DSN parsing supports both URL form
// ("postgres://user:pass@host:port/dbname?params") and keyword/value form
// ("host=localhost port=5432 user=postgres dbname=aisphere_hub_dev").
// The URL form is detected by a "postgres://" or "postgresql://" prefix;
// everything else is treated as keyword/value.

package postgres

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// adminDatabaseName is the database that always exists on a Postgres cluster
// and that the connecting user is guaranteed to have CONNECT privilege on
// (assuming they have CONNECT on the target database too). It is created by
// initdb and never dropped.
const adminDatabaseName = "postgres"

// isDatabaseNotExist reports whether err is a Postgres "database does not
// exist" error (SQLSTATE 3D000). Also falls back to message-string matching
// for wrapped errors that lose the typed *pgconn.PgError.
func isDatabaseNotExist(err error) bool {
	if err == nil {
		return false
	}
	// var pgErr *pgconn.PgError
	if asErr := err; asErr != nil {
		// Use errors.As via local var to avoid pulling errors package import
		// when this file is compiled standalone in tests.
		if e := tryAsPgError(asErr); e != nil && e.Code == "3D000" {
			return true
		}
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "sqlstate 3d000") ||
		(strings.Contains(lower, "database") && strings.Contains(lower, "does not exist") && strings.Contains(lower, "fatal"))
}

// tryAsPgError is a thin errors.As wrapper. Kept inline to avoid importing
// "errors" when the file is compiled in isolation.
func tryAsPgError(err error) *pgconn.PgError {
	// Walk the error chain manually to support both wrapped and unwrapped
	// errors. pgconn.PgError implements Unwrap() when wrapped via fmt.Errorf.
	for current := err; current != nil; {
		if pgErr, ok := current.(*pgconn.PgError); ok {
			return pgErr
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := current.(unwrapper)
		if !ok {
			return nil
		}
		current = u.Unwrap()
	}
	return nil
}

// createDatabase connects to the admin database (postgres) and issues
// CREATE DATABASE "<name>" when the target does not yet exist.
//
// originalDSN is the user-supplied DSN that failed with 3D000. We parse it
// to extract (a) the target database name and (b) an admin DSN that points
// at the same server but with dbname=postgres.
//
// The CREATE DATABASE statement:
//   - Uses double-quoted identifier to prevent SQL injection from the
//     database name (which came from the DSN, but defense in depth).
//   - Does NOT specify OWNER, ENCODING, LC_COLLATE, LC_CTYPE, TEMPLATE, or
//     CONNECTION LIMIT. The target inherits cluster defaults. Operators
//     who need custom encoding must pre-provision the database out-of-band
//     and leave Config.AutoCreateDatabase false.
//   - Cannot run inside a transaction. We use *sql.DB.Exec directly rather
//     than gorm.DB.Exec to avoid GORM's implicit transaction wrapping.
//
// The function is idempotent: if pg_database already contains the target
// name (e.g. a concurrent auto-create won the race), it returns nil without
// issuing CREATE DATABASE.
func createDatabase(originalDSN string) error {
	adminDSN, dbName, err := parseForAdmin(originalDSN)
	if err != nil {
		return fmt.Errorf("parse DSN for auto-create: %w", err)
	}

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connect to admin database %q: %w", adminDatabaseName, err)
	}
	sqlDB, err := adminDB.DB()
	if err != nil {
		return fmt.Errorf("acquire underlying sql.DB: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Idempotency check: SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)
	var exists bool
	if err := adminDB.Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", dbName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database existence: %w", err)
	}
	if exists {
		// Database was created between our failed open and now (race).
		// Treat as success so the caller retries the original open.
		return nil
	}

	// CREATE DATABASE cannot run inside a transaction. Use the underlying
	// *sql.DB to bypass GORM's transaction wrapping.

	stmt := fmt.Sprintf(`CREATE DATABASE %s`, quoteIdentifier(dbName))
	if _, err := sqlDB.Exec(stmt); err != nil {
		return fmt.Errorf("execute %q: %w", stmt, err)
	}
	return nil
}

// parseForAdmin splits the original DSN into (admin DSN, target database
// name). The admin DSN points at the same server with dbname=postgres.
//
// Supported DSN forms:
//
//  1. URL form:  postgres://user:pass@host:port/dbname?params
//     postgresql://user:pass@host:port/dbname?params
//  2. Keyword/value form: host=localhost port=5432 user=postgres
//     dbname=aisphere_hub_dev sslmode=disable
//
// For URL form, the database name is the first path segment after the
// leading slash. Query parameters are preserved. The path is rewritten to
// /postgres.
//
// For keyword/value form, the dbname= entry is extracted (or empty if not
// present) and replaced with dbname=postgres. Other entries are preserved
// verbatim.
//
// Returns an error if the database name is empty (no dbname in DSN means
// the server falls back to the user name — we cannot safely auto-create
// in that case because we would not know the target name).
func parseForAdmin(originalDSN string) (adminDSN, dbName string, err error) {
	trimmed := strings.TrimSpace(originalDSN)
	if trimmed == "" {
		return "", "", fmt.Errorf("empty DSN")
	}
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		return parseURLDSNForAdmin(trimmed)
	}
	return parseKVDSNForAdmin(trimmed)
}

func parseURLDSNForAdmin(dsn string) (adminDSN, dbName string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", fmt.Errorf("parse URL DSN: %w", err)
	}
	// Extract dbname from path. URL.Path is "/dbname" or just "/" or "".
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return "", "", fmt.Errorf("DSN has no database name in path: %s", dsn)
	}
	// dbname may contain a leading "?" or "/" suffix in some PG DSNs; take
	// the first segment only.
	if idx := strings.IndexAny(path, "?/"); idx >= 0 {
		path = path[:idx]
	}
	dbName = path
	if dbName == "" {
		return "", "", fmt.Errorf("DSN has empty database name in path: %s", dsn)
	}
	// Rewrite path to /postgres and rebuild the URL.
	u.Path = "/" + adminDatabaseName
	return u.String(), dbName, nil
}

func parseKVDSNForAdmin(dsn string) (adminDSN, dbName string, err error) {
	// keyword=value pairs separated by whitespace. Values may be quoted
	// with single quotes (PG libpq convention). For simplicity we do not
	// support backslash escapes inside single quotes — operators using
	// keyword/value DSNs rarely put special characters in values.
	pairs := splitKVPairs(dsn)
	if len(pairs) == 0 {
		return "", "", fmt.Errorf("DSN has no keyword=value pairs: %s", dsn)
	}
	dbName = pairs["dbname"]
	if dbName == "" {
		// libpq falls back to the user name when dbname is absent. We cannot
		// safely auto-create because we don't know the target name; require
		// explicit dbname.
		return "", "", fmt.Errorf("DSN has no dbname= entry (libpq would fall back to user name; auto-create requires explicit dbname): %s", dsn)
	}
	// Replace dbname with admin database.
	pairs["dbname"] = adminDatabaseName
	// Rebuild DSN.
	var sb strings.Builder
	first := true
	// Preserve a stable order by sorting keys for deterministic output.
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	// Stable sort (no sort import needed if we do insertion manually for
	// the small N; use sort.Strings for clarity).
	sortStrings(keys)
	for _, k := range keys {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		sb.WriteString(k)
		sb.WriteByte('=')
		v := pairs[k]
		if needsQuoting(v) {
			sb.WriteByte('\'')
			sb.WriteString(strings.ReplaceAll(v, "'", "\\'"))
			sb.WriteByte('\'')
		} else {
			sb.WriteString(v)
		}
	}
	return sb.String(), dbName, nil
}

// splitKVPairs parses a libpq keyword/value DSN into a map. Whitespace
// separates pairs; single-quoted values may contain whitespace.
func splitKVPairs(dsn string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(dsn) {
		// Skip whitespace.
		for i < len(dsn) && (dsn[i] == ' ' || dsn[i] == '\t') {
			i++
		}
		if i >= len(dsn) {
			break
		}
		// Read key.
		keyStart := i
		for i < len(dsn) && dsn[i] != '=' && dsn[i] != ' ' && dsn[i] != '\t' {
			i++
		}
		key := dsn[keyStart:i]
		// Skip whitespace before '='.
		for i < len(dsn) && (dsn[i] == ' ' || dsn[i] == '\t') {
			i++
		}
		if i >= len(dsn) || dsn[i] != '=' {
			break
		}
		i++ // skip '='
		// Skip whitespace after '='.
		for i < len(dsn) && (dsn[i] == ' ' || dsn[i] == '\t') {
			i++
		}
		// Read value.
		var value string
		if i < len(dsn) && dsn[i] == '\'' {
			// Quoted value: read until closing single quote.
			i++ // skip opening '
			valStart := i
			for i < len(dsn) && dsn[i] != '\'' {
				i++
			}
			value = dsn[valStart:i]
			if i < len(dsn) {
				i++ // skip closing '
			}
		} else {
			valStart := i
			for i < len(dsn) && dsn[i] != ' ' && dsn[i] != '\t' {
				i++
			}
			value = dsn[valStart:i]
		}
		if key != "" {
			out[key] = value
		}
	}
	return out
}

func needsQuoting(v string) bool {
	if v == "" {
		return true
	}
	for _, r := range v {
		if r == ' ' || r == '\t' || r == '\'' || r == '\\' {
			return true
		}
	}
	return false
}

// quoteIdentifier wraps a Postgres identifier in double quotes, doubling
// any embedded double quotes per the SQL standard. This prevents SQL
// injection even when the identifier comes from an untrusted source (the
// DSN database name is typically trusted, but defense in depth is cheap).
//
// Example: my"db -> "my""db"
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// sortStrings is a minimal stable sort for small slices to avoid importing
// "sort" (which would be overkill for the typical 5-10 key/value pairs in
// a DSN). Insertion sort is O(n²) but n is tiny.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
