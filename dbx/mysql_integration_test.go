//go:build integration
// +build integration

package dbx_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aisphereio/kernel/dbx"
	_ "github.com/aisphereio/kernel/dbx/mysql"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	"gorm.io/gorm"
)

// testSkillMySQL mirrors the postgres testSkill but uses MySQL-compatible
// column types (e.g. no ILIKE; bigint instead of int8).
type testSkillMySQL struct {
	ID          int64          `gorm:"primaryKey;autoIncrement;column:id"`
	Name        string         `gorm:"column:name;size:128;uniqueIndex;not null"`
	DisplayName string         `gorm:"column:display_name;size:256;not null;default:''"`
	Status      string         `gorm:"column:status;size:32;not null;default:'active'"`
	OwnerID     string         `gorm:"column:owner_id;size:128;not null;default:''"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;not null;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (testSkillMySQL) TableName() string { return "dbx_test_skills" }

// mysqlDSN returns a real MySQL DSN, either from env or via testcontainers.
func mysqlDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("KERNEL_DBX_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	if testing.Short() {
		t.Skip("skipping integration test in -short mode; set KERNEL_DBX_MYSQL_DSN or run without -short to enable testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mysqlC, err := tcmysql.Run(ctx,
		"mysql:8.0",
		tcmysql.WithDatabase("dbxtest"),
		tcmysql.WithUsername("dbx"),
		tcmysql.WithPassword("dbx"),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	t.Cleanup(func() {
		_ = mysqlC.Terminate(context.Background())
	})

	dsn, err := mysqlC.ConnectionString(ctx, "charset=utf8mb4&parseTime=true")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}
	return dsn
}

// newTestDBMySQL mirrors newTestDB but for MySQL.
func newTestDBMySQL(t *testing.T) dbx.DB {
	t.Helper()
	dsn := mysqlDSN(t)
	cfg := dbx.Config{
		Driver:             "mysql",
		DSN:                dsn,
		MaxOpenConns:       8,
		MaxIdleConns:       4,
		ConnMaxLifetime:    30 * time.Second,
		QueryTimeout:       5 * time.Second,
		SlowQueryThreshold: 100 * time.Millisecond,
	}
	db, err := dbx.New(cfg)
	if err != nil {
		t.Fatalf("dbx.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.AutoMigrate(context.Background(), &testSkillMySQL{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.GORM(context.Background()).Exec("DROP TABLE IF EXISTS dbx_test_skills").Error
	})

	if err := db.GORM(context.Background()).Exec("TRUNCATE TABLE dbx_test_skills").Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func TestMySQLCreateAndFindOne(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	skill := &testSkillMySQL{Name: "demo", DisplayName: "Demo Skill", OwnerID: "user_123"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if skill.ID == 0 {
		t.Fatal("ID not set after Create")
	}

	var got testSkillMySQL
	if err := db.FindOne(ctx, &got, "name = ?", "demo"); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.Name != "demo" {
		t.Fatalf("got name = %q, want demo", got.Name)
	}
}

func TestMySQLFindOneNoRows(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	var got testSkillMySQL
	err := db.FindOne(ctx, &got, "name = ?", "missing")
	if !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("err = %v, want dbx.ErrNoRows", err)
	}
}

func TestMySQLCreateDuplicateKey(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	skill := &testSkillMySQL{Name: "dup", DisplayName: "First"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	dup := &testSkillMySQL{Name: "dup", DisplayName: "Second"}
	err := db.Create(ctx, dup)
	if !errors.Is(err, dbx.ErrDuplicateKey) {
		t.Fatalf("err = %v, want dbx.ErrDuplicateKey", err)
	}
}

func TestMySQLSafeUpsert(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	skill := &testSkillMySQL{Name: "upsert", DisplayName: "V1", OwnerID: "owner_1", Status: "active"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated := &testSkillMySQL{
		ID:          skill.ID,
		Name:        "upsert",
		DisplayName: "V2",
		OwnerID:     "attacker_2",
		Status:      "archived",
	}
	err := db.SafeUpsert(ctx, updated, []string{"display_name", "status"})
	if err != nil {
		t.Fatalf("SafeUpsert: %v", err)
	}

	var got testSkillMySQL
	if err := db.FindOne(ctx, &got, "name = ?", "upsert"); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.DisplayName != "V2" {
		t.Fatalf("DisplayName = %q, want V2", got.DisplayName)
	}
	if got.Status != "archived" {
		t.Fatalf("Status = %q, want archived", got.Status)
	}
	if got.OwnerID != "owner_1" {
		t.Fatalf("OwnerID = %q, want owner_1 (should NOT be updated)", got.OwnerID)
	}
}

func TestMySQLSafeUpsertBlocksProtectedColumn(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	skill := &testSkillMySQL{Name: "blocked", DisplayName: "V1", OwnerID: "owner_1"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated := &testSkillMySQL{
		ID:          skill.ID,
		Name:        "blocked",
		DisplayName: "V2",
		OwnerID:     "attacker",
	}
	err := db.SafeUpsert(ctx, updated, []string{"display_name", "owner_id"})
	if !errors.Is(err, dbx.ErrUnsafeUpsert) {
		t.Fatalf("err = %v, want dbx.ErrUnsafeUpsert", err)
	}
}

func TestMySQLSoftDelete(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	skill := &testSkillMySQL{Name: "softdel", DisplayName: "To Delete"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := db.Delete(ctx, &testSkillMySQL{}, "name = ?", "softdel"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var got testSkillMySQL
	err := db.FindOne(ctx, &got, "name = ?", "softdel")
	if !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("err = %v, want ErrNoRows", err)
	}

	unscopedCtx := dbx.WithUnscoped(ctx)
	var got2 testSkillMySQL
	if err := db.FindOne(unscopedCtx, &got2, "name = ?", "softdel"); err != nil {
		t.Fatalf("FindOne WithUnscoped: %v", err)
	}
	if got2.Name != "softdel" {
		t.Fatalf("unscoped name = %q, want softdel", got2.Name)
	}
}

func TestMySQLIncrement(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	if err := db.GORM(ctx).Exec("ALTER TABLE dbx_test_skills ADD COLUMN download_count BIGINT NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("alter table: %v", err)
	}

	skill := &testSkillMySQL{Name: "incr", DisplayName: "Counter"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := db.Increment(ctx, &testSkillMySQL{}, "name = ?", []any{"incr"}, "download_count", 5); err != nil {
		t.Fatalf("Increment 1: %v", err)
	}
	if err := db.Increment(ctx, &testSkillMySQL{}, "name = ?", []any{"incr"}, "download_count", 3); err != nil {
		t.Fatalf("Increment 2: %v", err)
	}

	var got struct {
		Name          string `gorm:"column:name"`
		DownloadCount int64  `gorm:"column:download_count"`
	}
	if err := db.GORM(ctx).Raw("SELECT name, download_count FROM dbx_test_skills WHERE name = ?", "incr").Scan(&got).Error; err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if got.DownloadCount != 8 {
		t.Fatalf("DownloadCount = %d, want 8", got.DownloadCount)
	}
}

func TestMySQLPaginate(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		skill := &testSkillMySQL{
			Name:        fmt.Sprintf("skill-%02d", i),
			DisplayName: fmt.Sprintf("Skill %d", i),
		}
		if err := db.Create(ctx, skill); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	var page1 []testSkillMySQL
	res, err := db.Paginate(ctx, &page1, &testSkillMySQL{}, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("Paginate 1: %v", err)
	}
	if len(page1) != 10 {
		t.Fatalf("len(page1) = %d, want 10", len(page1))
	}
	if res.Total != 25 {
		t.Fatalf("Total = %d, want 25", res.Total)
	}
	if !res.HasMore {
		t.Fatal("HasMore = false, want true")
	}

	var page3 []testSkillMySQL
	res, err = db.Paginate(ctx, &page3, &testSkillMySQL{}, nil, nil, 3, 10)
	if err != nil {
		t.Fatalf("Paginate 3: %v", err)
	}
	if len(page3) != 5 {
		t.Fatalf("len(page3) = %d, want 5", len(page3))
	}
	if res.HasMore {
		t.Fatal("HasMore = true, want false")
	}
}

func TestMySQLInTxCommit(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	err := db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &testSkillMySQL{Name: "tx1", DisplayName: "First"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		s2 := &testSkillMySQL{Name: "tx2", DisplayName: "Second"}
		return tx.Create(ctx, s2)
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}

	var count int64
	count, err = db.Count(ctx, &testSkillMySQL{}, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestMySQLInTxRollback(t *testing.T) {
	db := newTestDBMySQL(t)
	ctx := context.Background()

	err := db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &testSkillMySQL{Name: "rb1", DisplayName: "First"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		return errors.New("intentional")
	})
	if err == nil || err.Error() != "intentional" {
		t.Fatalf("err = %v, want 'intentional'", err)
	}

	var count int64
	count, err = db.Count(ctx, &testSkillMySQL{}, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (rollback)", count)
	}
}
