// Package dbrepo provides opinionated repository helpers on top of dbx.
//
// It is intentionally table-configured instead of proto-schema-driven: SQL
// migration files remain the database source of truth, while repositories give AI
// agents a safe CRUD vocabulary for common resource tables.
package dbrepo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/dbx"
	"gorm.io/gorm"
)

const defaultMaxPageSize = 100

var (
	ErrTenantRequired      = errors.New("dbrepo: tenant is required")
	ErrOwnerRequired       = errors.New("dbrepo: owner subject is required")
	ErrPatchFieldForbidden = errors.New("dbrepo: patch field is forbidden")
)

// ResourceConfig describes Kernel's safe CRUD conventions for one table.
type ResourceConfig struct {
	Resource string
	Table    string

	IDColumn      string
	TenantColumn  string
	OwnerColumn   string
	DeletedColumn string
	CreatedColumn string
	UpdatedColumn string

	TenantScoped bool
	OwnerScoped  bool
	SoftDelete   bool
	Timestamps   bool

	AllowedFilters     []string
	AllowedSorts       []string
	AllowedPatchFields []string
	BlockedPatchFields []string
	MaxPageSize        int
}

func (c ResourceConfig) normalize() ResourceConfig {
	if c.IDColumn == "" {
		c.IDColumn = "id"
	}
	if c.TenantColumn == "" {
		c.TenantColumn = "tenant_id"
	}
	if c.OwnerColumn == "" {
		c.OwnerColumn = "owner_id"
	}
	if c.DeletedColumn == "" {
		c.DeletedColumn = "deleted_at"
	}
	if c.CreatedColumn == "" {
		c.CreatedColumn = "created_at"
	}
	if c.UpdatedColumn == "" {
		c.UpdatedColumn = "updated_at"
	}
	if c.MaxPageSize <= 0 {
		c.MaxPageSize = defaultMaxPageSize
	}
	return c
}

func (c ResourceConfig) validate() error {
	if c.Table == "" {
		return fmt.Errorf("dbrepo: table is required")
	}
	if c.IDColumn == "" {
		return fmt.Errorf("dbrepo: id column is required")
	}
	return nil
}

// Query is the safe list contract. Filters and sorts are checked against the
// ResourceConfig allowlists before reaching the DB.
type Query struct {
	Page    int
	Size    int
	Filters map[string]any
	Sort    string
	Desc    bool
}

// Page is a typed page result.
type Page[T any] struct {
	Items   []T
	Total   int64
	Page    int
	Size    int
	HasMore bool
}

// ResourceRepository implements tenant-aware, owner-aware, soft-delete-aware CRUD
// for a conventional resource table. It deliberately fails closed for tenant or
// owner scoped tables when ctx is missing the required scope metadata.
type ResourceRepository[T any] struct {
	db  dbx.DB
	cfg ResourceConfig
}

func NewResourceRepository[T any](db dbx.DB, cfg ResourceConfig) (*ResourceRepository[T], error) {
	cfg = cfg.normalize()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, fmt.Errorf("dbrepo: nil db")
	}
	return &ResourceRepository[T]{db: db, cfg: cfg}, nil
}

func MustResourceRepository[T any](db dbx.DB, cfg ResourceConfig) *ResourceRepository[T] {
	r, err := NewResourceRepository[T](db, cfg)
	if err != nil {
		panic(err)
	}
	return r
}

func (r *ResourceRepository[T]) Get(ctx context.Context, id any) (*T, error) {
	base, err := r.base(ctx)
	if err != nil {
		return nil, err
	}
	var row T
	q := base.Where(r.cfg.IDColumn+" = ?", id)
	if err := q.First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *ResourceRepository[T]) List(ctx context.Context, q Query) (*Page[T], error) {
	q = r.normalizeQuery(q)
	dbq, err := r.base(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range q.Filters {
		if !allowed(k, r.cfg.AllowedFilters) {
			return nil, fmt.Errorf("dbrepo: filter %q is not allowed", k)
		}
		dbq = dbq.Where(k+" = ?", v)
	}
	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, err
	}
	if q.Sort != "" {
		if !allowed(q.Sort, r.cfg.AllowedSorts) {
			return nil, fmt.Errorf("dbrepo: sort %q is not allowed", q.Sort)
		}
		dir := "ASC"
		if q.Desc {
			dir = "DESC"
		}
		dbq = dbq.Order(q.Sort + " " + dir)
	}
	var items []T
	if err := dbq.Offset((q.Page - 1) * q.Size).Limit(q.Size).Find(&items).Error; err != nil {
		return nil, err
	}
	return &Page[T]{Items: items, Total: total, Page: q.Page, Size: q.Size, HasMore: int64(q.Page*q.Size) < total}, nil
}

func (r *ResourceRepository[T]) Create(ctx context.Context, row *T) error {
	if row == nil {
		return fmt.Errorf("dbrepo: nil row")
	}
	now := time.Now()
	if r.cfg.TenantScoped {
		tenant := contextx.TenantFromContext(ctx)
		if tenant == "" {
			return ErrTenantRequired
		}
		setStringColumn(row, r.cfg.TenantColumn, tenant)
	}
	if r.cfg.OwnerScoped {
		owner := contextx.SubjectIDFromContext(ctx)
		if owner == "" {
			return ErrOwnerRequired
		}
		setStringColumn(row, r.cfg.OwnerColumn, owner)
	}
	if r.cfg.Timestamps {
		setTimeColumn(row, r.cfg.CreatedColumn, now)
		setTimeColumn(row, r.cfg.UpdatedColumn, now)
	}
	return r.db.GORM(ctx).Table(r.cfg.Table).Create(row).Error
}

func (r *ResourceRepository[T]) Patch(ctx context.Context, id any, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	clean := make(map[string]any, len(patch)+1)
	for k, v := range patch {
		if err := r.validatePatchField(k); err != nil {
			return err
		}
		clean[k] = v
	}
	if r.cfg.Timestamps {
		clean[r.cfg.UpdatedColumn] = time.Now()
	}
	base, err := r.base(ctx)
	if err != nil {
		return err
	}
	q := base.Where(r.cfg.IDColumn+" = ?", id)
	return q.Updates(clean).Error
}

func (r *ResourceRepository[T]) Delete(ctx context.Context, id any) error {
	base, err := r.base(ctx)
	if err != nil {
		return err
	}
	q := base.Where(r.cfg.IDColumn+" = ?", id)
	if r.cfg.SoftDelete {
		return q.Update(r.cfg.DeletedColumn, time.Now()).Error
	}
	return q.Delete(new(T)).Error
}

func (r *ResourceRepository[T]) base(ctx context.Context) (*gorm.DB, error) {
	tenant := ""
	owner := ""
	if r.cfg.TenantScoped {
		tenant = contextx.TenantFromContext(ctx)
		if tenant == "" {
			return nil, ErrTenantRequired
		}
	}
	if r.cfg.OwnerScoped {
		owner = contextx.SubjectIDFromContext(ctx)
		if owner == "" {
			return nil, ErrOwnerRequired
		}
	}
	q := r.db.GORM(ctx).Table(r.cfg.Table)
	if r.cfg.TenantScoped {
		q = q.Where(r.cfg.TenantColumn+" = ?", tenant)
	}
	if r.cfg.OwnerScoped {
		q = q.Where(r.cfg.OwnerColumn+" = ?", owner)
	}
	if r.cfg.SoftDelete {
		q = q.Where(r.cfg.DeletedColumn + " IS NULL")
	}
	return q, nil
}

func (r *ResourceRepository[T]) validatePatchField(field string) error {
	if field == "" {
		return fmt.Errorf("%w: empty field", ErrPatchFieldForbidden)
	}
	if isProtectedPatchField(field, r.cfg) || allowed(field, r.cfg.BlockedPatchFields) {
		return fmt.Errorf("%w: %s", ErrPatchFieldForbidden, field)
	}
	if len(r.cfg.AllowedPatchFields) > 0 && !allowed(field, r.cfg.AllowedPatchFields) {
		return fmt.Errorf("%w: %s", ErrPatchFieldForbidden, field)
	}
	return nil
}

func isProtectedPatchField(field string, cfg ResourceConfig) bool {
	switch field {
	case cfg.IDColumn, cfg.TenantColumn, cfg.OwnerColumn, cfg.CreatedColumn, cfg.DeletedColumn:
		return true
	}
	return false
}

func (r *ResourceRepository[T]) normalizeQuery(q Query) Query {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > r.cfg.MaxPageSize {
		q.Size = r.cfg.MaxPageSize
	}
	return q
}

func allowed(v string, xs []string) bool {
	if len(xs) == 0 {
		return false
	}
	sorted := sort.SliceIsSorted(xs, func(i, j int) bool { return xs[i] < xs[j] })
	if sorted {
		i := sort.SearchStrings(xs, v)
		return i < len(xs) && xs[i] == v
	}
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func setStringColumn(row any, column, value string) {
	if value != "" {
		setColumn(row, column, value)
	}
}
func setTimeColumn(row any, column string, value time.Time) { setColumn(row, column, value) }

func setColumn(row any, column string, value any) {
	rv := reflect.ValueOf(row)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < rv.NumField(); i++ {
		sf := rv.Type().Field(i)
		if !rv.Field(i).CanSet() {
			continue
		}
		if columnMatches(sf, column) {
			fv := rv.Field(i)
			vv := reflect.ValueOf(value)
			if vv.Type().AssignableTo(fv.Type()) {
				fv.Set(vv)
			}
			return
		}
	}
}

func columnMatches(sf reflect.StructField, column string) bool {
	if tag := sf.Tag.Get("gorm"); tag != "" {
		for _, part := range strings.Split(tag, ";") {
			if strings.HasPrefix(part, "column:") && strings.TrimPrefix(part, "column:") == column {
				return true
			}
		}
	}
	return toSnake(sf.Name) == column
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
