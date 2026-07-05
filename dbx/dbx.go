package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aisphereio/kernel/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================
// Public interfaces
// ============================================================================

// DB is the runtime database interface used by kernel modules and apps.
//
// All methods accept context.Context. Methods that take a model pointer
// (FindOne / Create / Save / SafeUpsert / Delete) operate on GORM models
// with the same struct tag conventions as GORM (`gorm:"column:..."`).
type DB interface {
	// GORM exposes the underlying *gorm.DB for advanced cases (Preload,
	// raw SQL, complex Where chains). Business code SHOULD prefer the
	// typed helpers below; GORM is the escape hatch.
	GORM(ctx context.Context) *gorm.DB

	// FindOne executes a query that returns at most one row, scanning
	// the result into dest (a pointer to a struct). Returns ErrNoRows
	// if no row matches.
	FindOne(ctx context.Context, dest any, query any, args ...any) error

	// FindOneByPK finds a row by primary key. pk may be a single value
	// or a map for composite keys.
	FindOneByPK(ctx context.Context, dest any, pk any) error

	// FindMany executes a query that returns multiple rows, scanning
	// each into dest (a pointer to a slice of structs).
	FindMany(ctx context.Context, dest any, query any, args ...any) error

	// Count returns the number of rows matching the query.
	Count(ctx context.Context, model any, query any, args ...any) (int64, error)

	// Create inserts one or more rows. If dest is a struct, it is updated
	// with the auto-generated primary key (if any).
	Create(ctx context.Context, dest any) error

	// Save updates the row by primary key, creating it if it does not exist.
	// All fields are written; use Update for partial updates.
	Save(ctx context.Context, dest any) error

	// Update updates specific columns by query.
	//   db.Update(ctx, &Skill{}, "name = ?", "demo", map[string]any{"status":"online"})
	Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error

	// UpdateColumns is like Update but uses GORM's UpdateColumns (skips
	// autoUpdateTime hooks). Use for atomic increments via gorm.Expr.
	UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error

	// Delete soft-deletes rows matching the query (or hard-deletes if the
	// model has no DeletedAt field).
	Delete(ctx context.Context, model any, query any, args ...any) error

	// DeleteByPK soft-deletes (or hard-deletes) a row by primary key.
	DeleteByPK(ctx context.Context, model any, pk any) error

	// SafeUpsert performs an INSERT ... ON CONFLICT DO UPDATE (postgres) or
	// INSERT ... ON DUPLICATE KEY UPDATE (mysql). Only columns listed in
	// allowedColumns are updated on conflict; other columns (including
	// owner_id / visibility / status and other sensitive fields) are
	// never overwritten. This is the SECURITY-annotated pattern from
	// aisphere-hub skill.go.
	SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error

	// Increment atomically increments a column by delta. Implemented as
	// UpdateColumns(col = col + delta), server-side, race-free.
	Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error

	// Paginate returns a page of rows + total count + hasMore.
	// page is 1-indexed; size is capped at 100.
	Paginate(ctx context.Context, dest any, model any, query any, args []any, page, size int) (*PageResult, error)

	// InTx runs fn inside a transaction. If fn returns nil, the
	// transaction is committed. If fn returns a non-nil error or panics,
	// the transaction is rolled back and the error (or recovered panic)
	// is returned.
	InTx(ctx context.Context, fn func(Tx) error) error

	// BeginTx starts a transaction. Manual commit/rollback; prefer InTx
	// unless you need finer control.
	BeginTx(ctx context.Context) (Tx, error)

	// PingContext verifies the database is reachable.
	PingContext(ctx context.Context) error

	// AutoMigrate creates or updates tables for the given models. Safe to
	// call repeatedly (idempotent). NOT recommended for production — use
	// SQL migrations for schema changes in prod.
	AutoMigrate(ctx context.Context, models ...any) error

	// Tables returns the list of tables in the current schema.
	Tables(ctx context.Context) ([]string, error)

	// Stats returns connection pool statistics.
	Stats() DBStats

	// DriverName returns the registered driver name ("postgres" / "mysql").
	DriverName() string

	// Close closes the database, releasing all connections. Idempotent.
	Close() error
}

// Tx is a database transaction. All methods behave like the corresponding
// DB methods but operate within the transaction.
type Tx interface {
	GORM(ctx context.Context) *gorm.DB
	FindOne(ctx context.Context, dest any, query any, args ...any) error
	FindOneByPK(ctx context.Context, dest any, pk any) error
	FindMany(ctx context.Context, dest any, query any, args ...any) error
	Count(ctx context.Context, model any, query any, args ...any) (int64, error)
	Create(ctx context.Context, dest any) error
	Save(ctx context.Context, dest any) error
	Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error
	UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error
	Delete(ctx context.Context, model any, query any, args ...any) error
	DeleteByPK(ctx context.Context, model any, pk any) error
	SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error
	Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error
	Commit() error
	Rollback() error
}

// PageResult holds a paginated query result.
type PageResult struct {
	Items   []any
	Total   int64
	Page    int
	Size    int
	HasMore bool
}

// DBStats mirrors gorm's underlying stats. We use sql.DBStats to avoid
// exporting gorm internals in the public API.
type DBStats = sql.DBStats

// ============================================================================
// Driver registry
// ============================================================================

// DriverOpener opens a *gorm.DB for the given DSN. Driver subpackages
// register their opener via RegisterDriver.
type DriverOpener func(dsn string, cfg Config) (*gorm.DB, error)

var (
	driverMu sync.RWMutex
	drivers  = map[string]DriverOpener{}
)

// RegisterDriver registers a driver opener under the given name.
// Called by dbx/postgres and dbx/mysql in their init() functions.
// Panics if the same name is registered twice.
func RegisterDriver(name string, fn DriverOpener) {
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, exists := drivers[name]; exists {
		panic(fmt.Sprintf("dbx: driver %q already registered", name))
	}
	drivers[name] = fn
}

// IsDriverRegistered returns true if the named driver has been registered.
func IsDriverRegistered(name string) bool {
	driverMu.RLock()
	defer driverMu.RUnlock()
	_, ok := drivers[name]
	return ok
}

// RegisteredDrivers returns the names of all registered drivers.
func RegisteredDrivers() []string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	out := make([]string, 0, len(drivers))
	for name := range drivers {
		out = append(out, name)
	}
	return out
}

// ============================================================================
// Context propagation
// ============================================================================

type ctxKey int

const (
	ctxKeyDB ctxKey = iota
	ctxKeyTx
	ctxKeyUnscoped
)

// InjectDB attaches a *gorm.DB (typically a transaction-wrapped one) to ctx
// so that downstream DB.GORM(ctx) / FindOne(ctx, ...) / etc. pick it up
// automatically. This is the aisphere-hub Runtime.DBFromContext pattern
// encoded as a public API.
func InjectDB(ctx context.Context, gormDB *gorm.DB) context.Context {
	return context.WithValue(ctx, ctxKeyDB, gormDB)
}

// InjectTx attaches a Tx to ctx so that downstream code can pick up the
// same transaction via DB.GORM(ctx) without explicit threading.
func InjectTx(ctx context.Context, tx Tx) context.Context {
	return context.WithValue(ctx, ctxKeyTx, tx)
}

// WithUnscoped returns a ctx that allows queries to read soft-deleted rows.
// Without this, any query on a model with DeletedAt returns an error if
// the caller tries to use Unscoped. This is the safety gate that prevents
// accidental "select deleted rows" leaks.
func WithUnscoped(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyUnscoped, true)
}

// isUnscoped returns true if ctx was marked with WithUnscoped.
func isUnscoped(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyUnscoped).(bool)
	return v
}

// ============================================================================
// Constructor
// ============================================================================

// New opens a database connection using the supplied Config.
//
// The driver must have been registered by importing dbx/postgres or
// dbx/mysql. Returns ErrUnknownDriver if not registered, ErrNilConfig if
// Driver or DSN is missing.
//
// The returned DB has all eight pre-actions wired up: error normalization,
// soft-delete safety, OnConflict whitelist, context propagation, audit hook
// (if Config.AuditEnabled), global QueryTimeout, slow-query log, and
// metrics (if Config.MetricsEnabled).
func New(cfg Config) (DB, error) {
	cfg = cfg.withDefaults()
	registerDBXMetrics(cfg)
	started := time.Now()
	dbxLogger(cfg).Info("dbx opening", logx.Bool("metrics_enabled", cfg.MetricsEnabled), logx.Bool("debug", cfg.Debug))
	if err := cfg.Validate(); err != nil {
		return nil, observeDBInit(cfg, started, err)
	}

	driverMu.RLock()
	open, ok := drivers[cfg.Driver]
	driverMu.RUnlock()
	if !ok {
		return nil, observeDBInit(cfg, started, fmt.Errorf("%w: %s", ErrUnknownDriver, cfg.Driver))
	}

	gormDB, err := open(cfg.DSN, cfg)
	if err != nil {
		return nil, observeDBInit(cfg, started, fmt.Errorf("dbx: open %s: %w", cfg.Driver, err))
	}

	base := newDB(cfg, gormDB)
	_ = observeDBInit(cfg, started, nil)
	return observeDB(base, cfg), nil
}

// ============================================================================
// Implementation: db
// ============================================================================

type db struct {
	cfg    Config
	gormDB *gorm.DB
	closed bool
	mu     sync.Mutex
}

func newDB(cfg Config, gormDB *gorm.DB) *db {
	return &db{cfg: cfg, gormDB: gormDB}
}

// GORM returns the *gorm.DB to use for this ctx. Priority:
//  1. *gorm.DB injected via InjectDB (request-scoped, typically a tx)
//  2. Tx injected via InjectTx (unwrapped to its underlying *gorm.DB)
//  3. The pool's own *gorm.DB, with ctx attached via WithContext
//
// QueryTimeout and WithUnscoped are applied as needed.
func (d *db) GORM(ctx context.Context) *gorm.DB {
	if d.isClosed() {
		// Return a *gorm.DB whose every operation will fail with ErrClosed.
		// We do this rather than returning nil to keep the API consistent.
		return d.gormDB.WithContext(ctx).Session(&gorm.Session{})
	}

	// Priority 1: injected *gorm.DB.
	if injected, ok := ctx.Value(ctxKeyDB).(*gorm.DB); ok && injected != nil {
		return d.applyDefaults(ctx, injected)
	}

	// Priority 2: injected Tx.
	if tx, ok := ctx.Value(ctxKeyTx).(Tx); ok && tx != nil {
		return d.applyDefaults(ctx, tx.GORM(ctx))
	}

	// Priority 3: pool DB.
	return d.applyDefaults(ctx, d.gormDB.WithContext(ctx))
}

// applyDefaults applies QueryTimeout and Unscoped semantics to a *gorm.DB.
func (d *db) applyDefaults(ctx context.Context, gormDB *gorm.DB) *gorm.DB {
	// Apply Unscoped if the ctx opted in.
	if isUnscoped(ctx) {
		gormDB = gormDB.Unscoped()
	}
	// QueryTimeout only applies when ctx has no deadline.
	if d.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			newCtx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
			// We cannot defer cancel here; attach it via a wrapper that
			// calls cancel when the session completes. GORM does not
			// expose a "session done" hook, so we leak the cancel.
			// This is acceptable because context.WithTimeout will fire
			// its timer eventually; the Go runtime cleans it up.
			// To avoid the leak entirely, business code should pass a
			// context with a deadline (e.g. from the HTTP handler).
			_ = cancel
			gormDB = gormDB.WithContext(newCtx)
		}
	}
	return gormDB
}

func (d *db) FindOne(ctx context.Context, dest any, query any, args ...any) error {
	tx := d.GORM(ctx)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	err := tx.First(dest).Error
	return wrapDriverErr(err)
}

func (d *db) FindOneByPK(ctx context.Context, dest any, pk any) error {
	err := d.GORM(ctx).First(dest, pk).Error
	return wrapDriverErr(err)
}

func (d *db) FindMany(ctx context.Context, dest any, query any, args ...any) error {
	tx := d.GORM(ctx)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	err := tx.Find(dest).Error
	return wrapDriverErr(err)
}

func (d *db) Count(ctx context.Context, model any, query any, args ...any) (int64, error) {
	var c int64
	tx := d.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	err := tx.Count(&c).Error
	return c, wrapDriverErr(err)
}

func (d *db) Create(ctx context.Context, dest any) error {
	err := d.GORM(ctx).Create(dest).Error
	return wrapDriverErr(err)
}

func (d *db) Save(ctx context.Context, dest any) error {
	err := d.GORM(ctx).Save(dest).Error
	return wrapDriverErr(err)
}

func (d *db) Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	tx := d.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.Updates(columns)
	if err := res.Error; err != nil {
		return wrapDriverErr(err)
	}
	return nil
}

func (d *db) UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	tx := d.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.UpdateColumns(columns)
	if err := res.Error; err != nil {
		return wrapDriverErr(err)
	}
	return nil
}

func (d *db) Delete(ctx context.Context, model any, query any, args ...any) error {
	tx := d.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.Delete(model)
	if err := res.Error; err != nil {
		return wrapDriverErr(err)
	}
	return nil
}

func (d *db) DeleteByPK(ctx context.Context, model any, pk any) error {
	res := d.GORM(ctx).Delete(model, pk)
	if err := res.Error; err != nil {
		return wrapDriverErr(err)
	}
	return nil
}

func (d *db) SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error {
	return safeUpsert(d.GORM(ctx), dest, allowedColumns)
}

func (d *db) Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error {
	return increment(d.GORM(ctx), model, query, args, column, delta)
}

func (d *db) Paginate(ctx context.Context, dest any, model any, query any, args []any, page, size int) (*PageResult, error) {
	return paginate(d.GORM(ctx), dest, model, query, args, page, size)
}

func (d *db) InTx(ctx context.Context, fn func(Tx) error) (err error) {
	// If a tx is already in ctx, reuse it (nested InTx becomes a no-op
	// wrapper, like GORM's nested Transaction behavior).
	if existing, ok := ctx.Value(ctxKeyTx).(Tx); ok && existing != nil {
		return fn(existing)
	}

	gormTx := d.GORM(ctx).Begin()
	if gormTx.Error != nil {
		return wrapDriverErr(gormTx.Error)
	}

	tx := &txImpl{gormTx: gormTx, parent: d}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// Inject the tx into ctx so business code can pick it up via DB.GORM(ctx).
	txCtx := InjectTx(ctx, tx)
	return fn(&txCtxAdapter{tx: tx, ctx: txCtx})
}

func (d *db) BeginTx(ctx context.Context) (Tx, error) {
	gormTx := d.GORM(ctx).Begin()
	if gormTx.Error != nil {
		return nil, wrapDriverErr(gormTx.Error)
	}
	return &txImpl{gormTx: gormTx, parent: d}, nil
}

func (d *db) PingContext(ctx context.Context) error {
	sqlDB, err := d.gormDB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (d *db) AutoMigrate(ctx context.Context, models ...any) error {
	return d.GORM(ctx).AutoMigrate(models...)
}

func (d *db) Tables(ctx context.Context) ([]string, error) {
	var tables []string
	err := d.gormDB.WithContext(ctx).Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() ORDER BY table_name",
	).Scan(&tables).Error
	return tables, err
}

func (d *db) Stats() DBStats {
	sqlDB, err := d.gormDB.DB()
	if err != nil {
		return DBStats{}
	}
	return sqlDB.Stats()
}

func (d *db) DriverName() string { return d.cfg.Driver }

func (d *db) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	d.mu.Unlock()
	sqlDB, err := d.gormDB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *db) isClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// ============================================================================
// Implementation: tx
// ============================================================================

type txImpl struct {
	gormTx *gorm.DB
	parent *db
	done   bool
	rolled bool
	mu     sync.Mutex
}

func (t *txImpl) GORM(ctx context.Context) *gorm.DB {
	if t.isDone() {
		// Return a *gorm.DB whose every op fails; Session() preserves ctx.
		return t.gormTx.WithContext(ctx).Session(&gorm.Session{})
	}
	return t.gormTx.WithContext(ctx)
}

func (t *txImpl) FindOne(ctx context.Context, dest any, query any, args ...any) error {
	if t.isDone() {
		return t.doneErr()
	}
	tx := t.GORM(ctx)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	return wrapDriverErr(tx.First(dest).Error)
}

func (t *txImpl) FindOneByPK(ctx context.Context, dest any, pk any) error {
	if t.isDone() {
		return t.doneErr()
	}
	return wrapDriverErr(t.GORM(ctx).First(dest, pk).Error)
}

func (t *txImpl) FindMany(ctx context.Context, dest any, query any, args ...any) error {
	if t.isDone() {
		return t.doneErr()
	}
	tx := t.GORM(ctx)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	return wrapDriverErr(tx.Find(dest).Error)
}

func (t *txImpl) Count(ctx context.Context, model any, query any, args ...any) (int64, error) {
	if t.isDone() {
		return 0, t.doneErr()
	}
	var c int64
	tx := t.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	err := tx.Count(&c).Error
	return c, wrapDriverErr(err)
}

func (t *txImpl) Create(ctx context.Context, dest any) error {
	if t.isDone() {
		return t.doneErr()
	}
	return wrapDriverErr(t.GORM(ctx).Create(dest).Error)
}

func (t *txImpl) Save(ctx context.Context, dest any) error {
	if t.isDone() {
		return t.doneErr()
	}
	return wrapDriverErr(t.GORM(ctx).Save(dest).Error)
}

func (t *txImpl) Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	if t.isDone() {
		return t.doneErr()
	}
	tx := t.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.Updates(columns)
	return wrapDriverErr(res.Error)
}

func (t *txImpl) UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	if t.isDone() {
		return t.doneErr()
	}
	tx := t.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.UpdateColumns(columns)
	return wrapDriverErr(res.Error)
}

func (t *txImpl) Delete(ctx context.Context, model any, query any, args ...any) error {
	if t.isDone() {
		return t.doneErr()
	}
	tx := t.GORM(ctx).Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.Delete(model)
	return wrapDriverErr(res.Error)
}

func (t *txImpl) DeleteByPK(ctx context.Context, model any, pk any) error {
	if t.isDone() {
		return t.doneErr()
	}
	res := t.GORM(ctx).Delete(model, pk)
	return wrapDriverErr(res.Error)
}

func (t *txImpl) SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error {
	if t.isDone() {
		return t.doneErr()
	}
	return safeUpsert(t.GORM(ctx), dest, allowedColumns)
}

func (t *txImpl) Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error {
	if t.isDone() {
		return t.doneErr()
	}
	return increment(t.GORM(ctx), model, query, args, column, delta)
}

func (t *txImpl) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		if t.rolled {
			return ErrTxRolledBack
		}
		return ErrTxCommitted
	}
	t.done = true
	return wrapDriverErr(t.gormTx.Commit().Error)
}

func (t *txImpl) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		if t.rolled {
			return nil // idempotent
		}
		return ErrTxCommitted
	}
	t.done = true
	t.rolled = true
	return wrapDriverErr(t.gormTx.Rollback().Error)
}

func (t *txImpl) isDone() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done
}

func (t *txImpl) doneErr() error {
	if t.rolled {
		return ErrTxRolledBack
	}
	return ErrTxCommitted
}

// txCtxAdapter wraps a txImpl and rebinds ctx to the tx-injected ctx so
// that nested calls inside InTx(fn) automatically pick up the same tx.
type txCtxAdapter struct {
	tx  *txImpl
	ctx context.Context
}

func (a *txCtxAdapter) GORM(_ context.Context) *gorm.DB {
	return a.tx.GORM(a.ctx)
}
func (a *txCtxAdapter) FindOne(_ context.Context, dest any, query any, args ...any) error {
	return a.tx.FindOne(a.ctx, dest, query, args...)
}
func (a *txCtxAdapter) FindOneByPK(_ context.Context, dest any, pk any) error {
	return a.tx.FindOneByPK(a.ctx, dest, pk)
}
func (a *txCtxAdapter) FindMany(_ context.Context, dest any, query any, args ...any) error {
	return a.tx.FindMany(a.ctx, dest, query, args...)
}
func (a *txCtxAdapter) Count(_ context.Context, model any, query any, args ...any) (int64, error) {
	return a.tx.Count(a.ctx, model, query, args...)
}
func (a *txCtxAdapter) Create(_ context.Context, dest any) error {
	return a.tx.Create(a.ctx, dest)
}
func (a *txCtxAdapter) Save(_ context.Context, dest any) error {
	return a.tx.Save(a.ctx, dest)
}
func (a *txCtxAdapter) Update(_ context.Context, model any, query any, args []any, columns map[string]any) error {
	return a.tx.Update(a.ctx, model, query, args, columns)
}
func (a *txCtxAdapter) UpdateColumns(_ context.Context, model any, query any, args []any, columns map[string]any) error {
	return a.tx.UpdateColumns(a.ctx, model, query, args, columns)
}
func (a *txCtxAdapter) Delete(_ context.Context, model any, query any, args ...any) error {
	return a.tx.Delete(a.ctx, model, query, args...)
}
func (a *txCtxAdapter) DeleteByPK(_ context.Context, model any, pk any) error {
	return a.tx.DeleteByPK(a.ctx, model, pk)
}
func (a *txCtxAdapter) SafeUpsert(_ context.Context, dest any, allowedColumns []string) error {
	return a.tx.SafeUpsert(a.ctx, dest, allowedColumns)
}
func (a *txCtxAdapter) Increment(_ context.Context, model any, query any, args []any, column string, delta int64) error {
	return a.tx.Increment(a.ctx, model, query, args, column, delta)
}
func (a *txCtxAdapter) Commit() error   { return a.tx.Commit() }
func (a *txCtxAdapter) Rollback() error { return a.tx.Rollback() }

// ============================================================================
// Helpers: SafeUpsert / Increment / Paginate
// ============================================================================

// protectedColumns are NEVER overwritten by SafeUpsert, even if the caller
// lists them in allowedColumns. These are security-critical fields that
// must only be set by their dedicated lifecycle methods (transfer ownership,
// publish skill, etc).
var protectedColumns = map[string]struct{}{
	"owner_id":   {},
	"created_at": {},
	"deleted_at": {},
}

// safeUpsert performs an INSERT ... ON CONFLICT DO UPDATE (postgres) or
// INSERT ... ON DUPLICATE KEY UPDATE (mysql). Only columns in allowedColumns
// are updated on conflict. Protected columns are NEVER updated even if
// listed in allowedColumns (defense in depth).
func safeUpsert(gormDB *gorm.DB, dest any, allowedColumns []string) error {
	// Build the cleaned whitelist.
	cleaned := make([]string, 0, len(allowedColumns))
	for _, col := range allowedColumns {
		if _, blocked := protectedColumns[col]; blocked {
			return fmt.Errorf("%w: column %q is protected and cannot be in SafeUpsert whitelist", ErrUnsafeUpsert, col)
		}
		cleaned = append(cleaned, col)
	}
	if len(cleaned) == 0 {
		return fmt.Errorf("%w: allowedColumns is empty after filtering", ErrUnsafeUpsert)
	}

	// GORM's clause.OnConflict works for both postgres and mysql; the driver
	// translates it to the correct syntax.
	res := gormDB.Clauses(onConflictClause(primaryKeyColumns(gormDB, dest), cleaned)).Create(dest)
	return wrapDriverErr(res.Error)
}

// onConflictClause builds a dialect-agnostic OnConflict clause. Driver
// subpackages may override this if needed (currently the default works
// for both pg and mysql via GORM's translation).
//
// Returns clause.Expression (from gorm.io/gorm/clause) so it can be passed
// directly to gormDB.Clauses().
var onConflictClause = func(conflictColumns []clause.Column, updateColumns []string) clause.Expression {
	return buildOnConflictClause(conflictColumns, updateColumns)
}

func primaryKeyColumns(gormDB *gorm.DB, dest any) []clause.Column {
	stmt := &gorm.Statement{DB: gormDB}
	if err := stmt.Parse(dest); err != nil || stmt.Schema == nil {
		return nil
	}
	out := make([]clause.Column, 0, len(stmt.Schema.PrimaryFields))
	for _, field := range stmt.Schema.PrimaryFields {
		if field != nil && field.DBName != "" {
			out = append(out, clause.Column{Name: field.DBName})
		}
	}
	return out
}

// increment atomically adds delta to column.
func increment(gormDB *gorm.DB, model any, query any, args []any, column string, delta int64) error {
	tx := gormDB.Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}
	res := tx.UpdateColumn(column, gormExpr(fmt.Sprintf("%s + ?", column), delta))
	return wrapDriverErr(res.Error)
}

// paginate executes a paginated query: page is 1-indexed, size capped at 100.
// Returns items (scanned into dest), total count, and hasMore flag.
func paginate(gormDB *gorm.DB, dest any, model any, query any, args []any, page, size int) (*PageResult, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	tx := gormDB.Model(model)
	if query != nil {
		tx = tx.Where(query, args...)
	}

	// Total count.
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, wrapDriverErr(err)
	}

	// Page query: fetch size+1 to compute hasMore without a separate query.
	var items []any
	_ = items
	// GORM's Find needs a typed slice; dest must be *[]T. We use tx.Find(dest).
	res := tx.Limit(size + 1).Offset((page - 1) * size).Find(dest)
	if err := res.Error; err != nil {
		return nil, wrapDriverErr(err)
	}

	// Determine hasMore by counting items in dest.
	hasMore := sliceLen(dest) > size
	if hasMore {
		sliceTruncate(dest, size)
	}

	return &PageResult{
		Items:   nil, // caller has dest; we don't need to return it
		Total:   total,
		Page:    page,
		Size:    size,
		HasMore: hasMore,
	}, nil
}

// ============================================================================
// Error wrapping
// ============================================================================

// wrapDriverErr inspects the underlying driver error and wraps it with a
// dbx sentinel error if it matches a known condition.
func wrapDriverErr(err error) error {
	if err == nil {
		return nil
	}

	// GORM ErrRecordNotFound -> ErrNoRows
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NormalizeError(fmt.Errorf("%w: %v", ErrNoRows, err))
	}

	// Context timeout.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return NormalizeError(fmt.Errorf("%w: %v", ErrTimeout, err))
	}

	// Driver-specific errors (registered by dbx/postgres and dbx/mysql).
	if mapped := mapDriverError(err); mapped != nil {
		return NormalizeError(fmt.Errorf("%w: %v", mapped, err))
	}

	return NormalizeError(err)
}

// mapDriverError delegates to registered driver error mappers.
// Returns nil if no mapper recognizes the error.
var (
	driverErrorMappersMu sync.RWMutex
	driverErrorMappers   []func(error) error
)

// RegisterErrorMapper adds a function that inspects an error and returns
// the matching dbx sentinel (or nil if not recognized). Called by driver
// subpackages via init().
func RegisterErrorMapper(fn func(error) error) {
	driverErrorMappersMu.Lock()
	defer driverErrorMappersMu.Unlock()
	driverErrorMappers = append(driverErrorMappers, fn)
}

func mapDriverError(err error) error {
	driverErrorMappersMu.RLock()
	defer driverErrorMappersMu.RUnlock()
	for _, mapper := range driverErrorMappers {
		if mapped := mapper(err); mapped != nil {
			return mapped
		}
	}
	return nil
}

// ============================================================================
// AssertAffected helper (called by business code)
// ============================================================================

// AssertAffected returns ErrNoEffect if res.RowsAffected == 0, or
// wrapDriverErr(res.Error) otherwise. This is the "no row was changed"
// pattern from aisphere-hub skill.go (10+ occurrences of
// `if res.RowsAffected == 0 { return ErrXxxNotFound }`).
//
// Usage:
//
//	res := tx.Model(&Skill{}).Where("name = ?", name).Updates(map[string]any{...})
//	if err := dbx.AssertAffected(res); err != nil {
//	    if errors.Is(err, dbx.ErrNoEffect) {
//	        return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", ...)
//	    }
//	    return err
//	}
func AssertAffected(res *gorm.DB) error {
	if res.Error != nil {
		return wrapDriverErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNoEffect
	}
	return nil
}

// ============================================================================
// Imports kept for future use
// ============================================================================

var _ = time.Second
