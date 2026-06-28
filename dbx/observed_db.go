package dbx

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type observedDB struct {
	next DB
	cfg  Config
}

func observeDB(next DB, cfg Config) DB {
	if next == nil {
		return nil
	}
	return &observedDB{next: next, cfg: cfg}
}

func (d *observedDB) GORM(ctx context.Context) *gorm.DB { return d.next.GORM(ctx) }

func (d *observedDB) FindOne(ctx context.Context, dest any, query any, args ...any) error {
	start := time.Now()
	err := d.next.FindOne(ctx, dest, query, args...)
	return observeDBOperation(d.cfg, ctx, "find_one", start, err)
}

func (d *observedDB) FindOneByPK(ctx context.Context, dest any, pk any) error {
	start := time.Now()
	err := d.next.FindOneByPK(ctx, dest, pk)
	return observeDBOperation(d.cfg, ctx, "find_one_by_pk", start, err)
}

func (d *observedDB) FindMany(ctx context.Context, dest any, query any, args ...any) error {
	start := time.Now()
	err := d.next.FindMany(ctx, dest, query, args...)
	return observeDBOperation(d.cfg, ctx, "find_many", start, err)
}

func (d *observedDB) Count(ctx context.Context, model any, query any, args ...any) (int64, error) {
	start := time.Now()
	count, err := d.next.Count(ctx, model, query, args...)
	return count, observeDBOperation(d.cfg, ctx, "count", start, err)
}

func (d *observedDB) Create(ctx context.Context, dest any) error {
	start := time.Now()
	err := d.next.Create(ctx, dest)
	return observeDBOperation(d.cfg, ctx, "create", start, err)
}

func (d *observedDB) Save(ctx context.Context, dest any) error {
	start := time.Now()
	err := d.next.Save(ctx, dest)
	return observeDBOperation(d.cfg, ctx, "save", start, err)
}

func (d *observedDB) Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	start := time.Now()
	err := d.next.Update(ctx, model, query, args, columns)
	return observeDBOperation(d.cfg, ctx, "update", start, err)
}

func (d *observedDB) UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	start := time.Now()
	err := d.next.UpdateColumns(ctx, model, query, args, columns)
	return observeDBOperation(d.cfg, ctx, "update_columns", start, err)
}

func (d *observedDB) Delete(ctx context.Context, model any, query any, args ...any) error {
	start := time.Now()
	err := d.next.Delete(ctx, model, query, args...)
	return observeDBOperation(d.cfg, ctx, "delete", start, err)
}

func (d *observedDB) DeleteByPK(ctx context.Context, model any, pk any) error {
	start := time.Now()
	err := d.next.DeleteByPK(ctx, model, pk)
	return observeDBOperation(d.cfg, ctx, "delete_by_pk", start, err)
}

func (d *observedDB) SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error {
	start := time.Now()
	err := d.next.SafeUpsert(ctx, dest, allowedColumns)
	return observeDBOperation(d.cfg, ctx, "safe_upsert", start, err)
}

func (d *observedDB) Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error {
	start := time.Now()
	err := d.next.Increment(ctx, model, query, args, column, delta)
	return observeDBOperation(d.cfg, ctx, "increment", start, err)
}

func (d *observedDB) Paginate(ctx context.Context, dest any, model any, query any, args []any, page, size int) (*PageResult, error) {
	start := time.Now()
	result, err := d.next.Paginate(ctx, dest, model, query, args, page, size)
	return result, observeDBOperation(d.cfg, ctx, "paginate", start, err)
}

func (d *observedDB) InTx(ctx context.Context, fn func(Tx) error) error {
	start := time.Now()
	err := d.next.InTx(ctx, func(tx Tx) error {
		return fn(&observedTx{next: tx, cfg: d.cfg})
	})
	return observeDBOperationRaw(d.cfg, ctx, "transaction", start, err)
}

func (d *observedDB) BeginTx(ctx context.Context) (Tx, error) {
	start := time.Now()
	tx, err := d.next.BeginTx(ctx)
	if err != nil {
		return nil, observeDBOperation(d.cfg, ctx, "begin_tx", start, err)
	}
	_ = observeDBOperation(d.cfg, ctx, "begin_tx", start, nil)
	return &observedTx{next: tx, cfg: d.cfg}, nil
}

func (d *observedDB) PingContext(ctx context.Context) error {
	start := time.Now()
	err := d.next.PingContext(ctx)
	return observeDBOperation(d.cfg, ctx, "ping", start, err)
}

func (d *observedDB) AutoMigrate(ctx context.Context, models ...any) error {
	start := time.Now()
	err := d.next.AutoMigrate(ctx, models...)
	return observeDBOperation(d.cfg, ctx, "auto_migrate", start, err)
}

func (d *observedDB) Tables(ctx context.Context) ([]string, error) {
	start := time.Now()
	tables, err := d.next.Tables(ctx)
	return tables, observeDBOperation(d.cfg, ctx, "tables", start, err)
}

func (d *observedDB) Stats() DBStats     { return d.next.Stats() }
func (d *observedDB) DriverName() string { return d.next.DriverName() }

func (d *observedDB) Close() error {
	start := time.Now()
	err := d.next.Close()
	return observeDBOperation(d.cfg, context.Background(), "close", start, err)
}

type observedTx struct {
	next Tx
	cfg  Config
}

func (t *observedTx) GORM(ctx context.Context) *gorm.DB { return t.next.GORM(ctx) }
func (t *observedTx) FindOne(ctx context.Context, dest any, query any, args ...any) error {
	start := time.Now()
	err := t.next.FindOne(ctx, dest, query, args...)
	return observeDBOperation(t.cfg, ctx, "tx_find_one", start, err)
}
func (t *observedTx) FindOneByPK(ctx context.Context, dest any, pk any) error {
	start := time.Now()
	err := t.next.FindOneByPK(ctx, dest, pk)
	return observeDBOperation(t.cfg, ctx, "tx_find_one_by_pk", start, err)
}
func (t *observedTx) FindMany(ctx context.Context, dest any, query any, args ...any) error {
	start := time.Now()
	err := t.next.FindMany(ctx, dest, query, args...)
	return observeDBOperation(t.cfg, ctx, "tx_find_many", start, err)
}
func (t *observedTx) Count(ctx context.Context, model any, query any, args ...any) (int64, error) {
	start := time.Now()
	count, err := t.next.Count(ctx, model, query, args...)
	return count, observeDBOperation(t.cfg, ctx, "tx_count", start, err)
}
func (t *observedTx) Create(ctx context.Context, dest any) error {
	start := time.Now()
	err := t.next.Create(ctx, dest)
	return observeDBOperation(t.cfg, ctx, "tx_create", start, err)
}
func (t *observedTx) Save(ctx context.Context, dest any) error {
	start := time.Now()
	err := t.next.Save(ctx, dest)
	return observeDBOperation(t.cfg, ctx, "tx_save", start, err)
}
func (t *observedTx) Update(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	start := time.Now()
	err := t.next.Update(ctx, model, query, args, columns)
	return observeDBOperation(t.cfg, ctx, "tx_update", start, err)
}
func (t *observedTx) UpdateColumns(ctx context.Context, model any, query any, args []any, columns map[string]any) error {
	start := time.Now()
	err := t.next.UpdateColumns(ctx, model, query, args, columns)
	return observeDBOperation(t.cfg, ctx, "tx_update_columns", start, err)
}
func (t *observedTx) Delete(ctx context.Context, model any, query any, args ...any) error {
	start := time.Now()
	err := t.next.Delete(ctx, model, query, args...)
	return observeDBOperation(t.cfg, ctx, "tx_delete", start, err)
}
func (t *observedTx) DeleteByPK(ctx context.Context, model any, pk any) error {
	start := time.Now()
	err := t.next.DeleteByPK(ctx, model, pk)
	return observeDBOperation(t.cfg, ctx, "tx_delete_by_pk", start, err)
}
func (t *observedTx) SafeUpsert(ctx context.Context, dest any, allowedColumns []string) error {
	start := time.Now()
	err := t.next.SafeUpsert(ctx, dest, allowedColumns)
	return observeDBOperation(t.cfg, ctx, "tx_safe_upsert", start, err)
}
func (t *observedTx) Increment(ctx context.Context, model any, query any, args []any, column string, delta int64) error {
	start := time.Now()
	err := t.next.Increment(ctx, model, query, args, column, delta)
	return observeDBOperation(t.cfg, ctx, "tx_increment", start, err)
}
func (t *observedTx) Commit() error {
	start := time.Now()
	err := t.next.Commit()
	return observeDBOperation(t.cfg, context.Background(), "tx_commit", start, err)
}
func (t *observedTx) Rollback() error {
	start := time.Now()
	err := t.next.Rollback()
	return observeDBOperation(t.cfg, context.Background(), "tx_rollback", start, err)
}
