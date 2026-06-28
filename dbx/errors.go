package dbx

import (
	"errors"
	"time"
)

// ============================================================================
// Public errors
// ============================================================================

// Public sentinel errors. All support errors.Is even when wrapped via
// fmt.Errorf("%w: ...", sentinel, ...).

var (
	// ErrNoRows is returned by FindOne / FindOneByPK when no row matches.
	// It is a dbx-specific sentinel; GORM's gorm.ErrRecordNotFound is
	// automatically translated to this on every Find* call.
	ErrNoRows = newSentinel("dbx: no rows in result set")

	// ErrDuplicateKey is returned by Create / SafeUpsert when a unique
	// constraint is violated. Driver-specific detection:
	//   - postgres: pgconn.PgError Code == "23505"
	//   - mysql:    mysql.MySQLError Number == 1062
	ErrDuplicateKey = newSentinel("dbx: duplicate key")

	// ErrTimeout is returned when a query exceeds its context deadline or
	// the global QueryTimeout.
	ErrTimeout = newSentinel("dbx: query timed out")

	// ErrSchemaNotReady is returned when the underlying table does not
	// exist. This typically means migrations have not been applied.
	// Driver-specific detection:
	//   - postgres: pgconn.PgError Code == "42P01"
	//   - mysql:    mysql.MySQLError Number == 1146
	ErrSchemaNotReady = newSentinel("dbx: schema not ready (run migrations)")

	// ErrForeignKeyViolation is returned when a foreign-key constraint
	// is violated.
	//   - postgres: pgconn.PgError Code == "23503"
	//   - mysql:    mysql.MySQLError Number == 1452
	ErrForeignKeyViolation = newSentinel("dbx: foreign key violation")

	// ErrClosed is returned when a closed DB is used.
	ErrClosed = newSentinel("dbx: database is closed")

	// ErrNilConfig is returned by New when Config is missing required fields.
	ErrNilConfig = newSentinel("dbx: config is missing required fields")

	// ErrUnknownDriver is returned when the driver name has not been registered.
	ErrUnknownDriver = newSentinel("dbx: unknown driver (did you import dbx/postgres or dbx/mysql?)")

	// ErrTxRolledBack is returned when an operation is attempted on a rolled-back Tx.
	ErrTxRolledBack = newSentinel("dbx: transaction already rolled back")

	// ErrTxCommitted is returned when Commit is called twice on the same Tx.
	ErrTxCommitted = newSentinel("dbx: transaction already committed")

	// ErrUnscopedRequired is returned when a soft-delete-protected query
	// is attempted without WithUnscoped.
	ErrUnscopedRequired = newSentinel("dbx: query touches soft-deleted rows; use WithUnscoped to opt in")

	// ErrUnsafeUpsert is returned by SafeUpsert when the row contains a
	// protected column that is not in the allowedColumns whitelist.
	ErrUnsafeUpsert = newSentinel("dbx: SafeUpsert blocked a protected column")

	// ErrNoEffect is returned by AssertAffected when RowsAffected == 0.
	ErrNoEffect = newSentinel("dbx: operation affected 0 rows")
)

// ============================================================================
// errorSentinel type
// ============================================================================

// errorSentinel is a string-based error type. Wrapped via
// fmt.Errorf("%w", ...), it is still matchable by errors.Is.
type errorSentinel struct {
	msg string
}

func newSentinel(msg string) error {
	return errorSentinel{msg: msg}
}

func (e errorSentinel) Error() string { return e.msg }

// Is allows errors.Is(err, ErrXxx) to match by sentinel identity.
func (e errorSentinel) Is(target error) bool {
	if other, ok := target.(errorSentinel); ok {
		return e.msg == other.msg
	}
	return false
}

// ============================================================================
// Config
// ============================================================================

// Config holds the configuration for a DB connection pool.
//
// Fields use json tags so the struct can be loaded directly from configx via
// `cfg.Value("database").Scan(&dbCfg)`.
type Config struct {
	// Driver selects the registered driver: "postgres" or "mysql".
	Driver string `json:"driver"`

	// DSN is the data source name (driver-specific connection string).
	DSN string `json:"dsn"`

	// MaxOpenConns limits the number of open connections. 0 means unlimited.
	MaxOpenConns int `json:"max_open_conns"`

	// MaxIdleConns limits the number of idle connections. 0 means default (2).
	MaxIdleConns int `json:"max_idle_conns"`

	// ConnMaxLifetime is how long a connection may live before being recycled.
	// Zero means no limit.
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime_ns"`

	// ConnMaxIdleTime is how long an idle connection may live before being closed.
	// Zero means no limit.
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time_ns"`

	// QueryTimeout is the default timeout applied to queries when the context
	// has no deadline. Zero means no timeout (rely on context).
	QueryTimeout time.Duration `json:"query_timeout_ns"`

	// SlowQueryThreshold is the duration above which a query is logged as
	// slow via logx. Zero disables slow-query logging. Default 200ms.
	SlowQueryThreshold time.Duration `json:"slow_query_threshold_ns"`

	// AuditEnabled controls whether the auto audit hook is registered.
	// When true, BeforeCreate / AfterUpdate / BeforeDelete callbacks emit
	// audit events. Default false (opt-in to avoid surprises).
	AuditEnabled bool `json:"audit_enabled"`

	// MetricsEnabled controls whether per-query metrics are recorded.
	// Default false (opt-in).
	MetricsEnabled bool `json:"metrics_enabled"`

	// DryRun, when true, causes GORM to log SQL without executing it.
	// Useful for staging environment tests. Default false.
	DryRun bool `json:"dry_run"`

	// Debug, when true, logs every SQL statement via logx at Debug level.
	// Default false.
	Debug bool `json:"debug"`
}

// Validate returns ErrNilConfig if required fields are missing.
func (c Config) Validate() error {
	if c.Driver == "" || c.DSN == "" {
		return ErrNilConfig
	}
	return nil
}

// withDefaults returns a copy with zero-value fields replaced by sensible
// defaults.
func (c Config) withDefaults() Config {
	out := c
	if out.SlowQueryThreshold == 0 && out.Debug {
		// In debug mode, default slow threshold to a low value so it's visible.
		out.SlowQueryThreshold = 50 * time.Millisecond
	}
	return out
}

// ensure errors is referenced (used by errors.Is in callers).
var _ = errors.Is
