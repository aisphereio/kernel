package dbrepo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aisphereio/kernel/dbx"
	"gorm.io/gorm"
)

type skillRow struct {
	ID        string
	TenantID  string    `gorm:"column:tenant_id"`
	OwnerID   string    `gorm:"column:owner_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func TestResourceConfigNormalize(t *testing.T) {
	cfg := (ResourceConfig{Table: "skills"}).normalize()
	if cfg.IDColumn != "id" || cfg.TenantColumn != "tenant_id" || cfg.MaxPageSize != defaultMaxPageSize {
		t.Fatalf("bad defaults: %+v", cfg)
	}
}

func TestSetColumnByGormTagAndSnakeName(t *testing.T) {
	row := &skillRow{}
	setStringColumn(row, "tenant_id", "t1")
	setStringColumn(row, "owner_id", "u1")
	setTimeColumn(row, "created_at", time.Unix(10, 0))
	if row.TenantID != "t1" || row.OwnerID != "u1" || row.CreatedAt.IsZero() {
		t.Fatalf("column injection failed: %+v", row)
	}
}

type fakeDB struct{ dbx.DB }

func (fakeDB) GORM(context.Context) *gorm.DB {
	panic("GORM must not be reached before scope validation")
}

func TestTenantScopedGetFailsClosedWithoutTenant(t *testing.T) {
	repo := MustResourceRepository[skillRow](fakeDB{}, ResourceConfig{Table: "skills", TenantScoped: true})
	_, err := repo.Get(context.Background(), "s1")
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}

func TestPatchRejectsProtectedFields(t *testing.T) {
	repo := MustResourceRepository[skillRow](fakeDB{}, ResourceConfig{Table: "skills", TenantScoped: true})
	err := repo.Patch(context.Background(), "s1", map[string]any{"tenant_id": "other"})
	if !errors.Is(err, ErrPatchFieldForbidden) {
		t.Fatalf("expected ErrPatchFieldForbidden, got %v", err)
	}
}
