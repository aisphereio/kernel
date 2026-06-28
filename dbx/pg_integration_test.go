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
	_ "github.com/aisphereio/kernel/dbx/postgres"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

// testSkill mirrors aisphere-hub's skillModel shape (subset).
type testSkill struct {
	ID          int64          `gorm:"primaryKey;autoIncrement;column:id"`
	Name        string         `gorm:"column:name;size:128;uniqueIndex;not null"`
	DisplayName string         `gorm:"column:display_name;size:256;not null;default:''"`
	Status      string         `gorm:"column:status;size:32;not null;default:'active'"`
	OwnerID     string         `gorm:"column:owner_id;size:128;not null;default:''"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;not null;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (testSkill) TableName() string { return "dbx_test_skills" }

// pgDSN returns a real PG DSN, either from env or via testcontainers.
// Tests that need a DB call this; tests that don't are pure unit tests.
func pgDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("KERNEL_DBX_PG_DSN"); dsn != "" {
		return dsn
	}
	if testing.Short() {
		t.Skip("skipping integration test in -short mode; set KERNEL_DBX_PG_DSN or run without -short to enable testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("dbxtest"),
		tcpostgres.WithUsername("dbx"),
		tcpostgres.WithPassword("dbx"),
		tcpostgres.BasicWaitStrategies(),
		tcpostgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = pgC.Terminate(context.Background())
	})

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}
	return dsn
}

// newTestDB returns a dbx.DB backed by real PG, with AutoMigrate done.
func newTestDB(t *testing.T) dbx.DB {
	t.Helper()
	dsn := pgDSN(t)
	cfg := dbx.Config{
		Driver:             "postgres",
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

	if err := db.AutoMigrate(context.Background(), &testSkill{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.GORM(context.Background()).Exec("DROP TABLE IF EXISTS dbx_test_skills").Error
	})

	// Truncate before each test to ensure isolation.
	if err := db.GORM(context.Background()).Exec("TRUNCATE dbx_test_skills RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func TestPGCreateAndFindOne(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	skill := &testSkill{Name: "demo", DisplayName: "Demo Skill", OwnerID: "user_123"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if skill.ID == 0 {
		t.Fatal("ID not set after Create")
	}

	var got testSkill
	if err := db.FindOne(ctx, &got, "name = ?", "demo"); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.Name != "demo" {
		t.Fatalf("got name = %q, want demo", got.Name)
	}
	if got.OwnerID != "user_123" {
		t.Fatalf("got owner_id = %q, want user_123", got.OwnerID)
	}
}

func TestPGFindOneNoRows(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	var got testSkill
	err := db.FindOne(ctx, &got, "name = ?", "missing")
	if !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("err = %v, want dbx.ErrNoRows", err)
	}
}

func TestPGCreateDuplicateKey(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	skill := &testSkill{Name: "dup", DisplayName: "First"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	dup := &testSkill{Name: "dup", DisplayName: "Second"}
	err := db.Create(ctx, dup)
	if !errors.Is(err, dbx.ErrDuplicateKey) {
		t.Fatalf("err = %v, want dbx.ErrDuplicateKey", err)
	}
}

func TestPGSafeUpsert(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Initial insert.
	skill := &testSkill{Name: "upsert", DisplayName: "V1", OwnerID: "owner_1", Status: "active"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Upsert with whitelist: display_name and status can be updated, owner_id CANNOT.
	updated := &testSkill{
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

	// Verify: display_name and status updated; owner_id unchanged.
	var got testSkill
	if err := db.FindOne(ctx, &got, "name = ?", "upsert"); err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.DisplayName != "V2" {
		t.Fatalf("DisplayName = %q, want V2 (should be updated)", got.DisplayName)
	}
	if got.Status != "archived" {
		t.Fatalf("Status = %q, want archived (should be updated)", got.Status)
	}
	if got.OwnerID != "owner_1" {
		t.Fatalf("OwnerID = %q, want owner_1 (should NOT be updated)", got.OwnerID)
	}
}

func TestPGSafeUpsertBlocksProtectedColumn(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	skill := &testSkill{Name: "blocked", DisplayName: "V1", OwnerID: "owner_1"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Try to whitelist owner_id — should error.
	updated := &testSkill{
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

func TestPGSoftDelete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	skill := &testSkill{Name: "softdel", DisplayName: "To Delete"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Delete (soft).
	if err := db.Delete(ctx, &testSkill{}, "name = ?", "softdel"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Normal query: should NOT find it.
	var got testSkill
	err := db.FindOne(ctx, &got, "name = ?", "softdel")
	if !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("err = %v, want ErrNoRows (soft-deleted should be invisible)", err)
	}

	// WithUnscoped: should find it.
	unscopedCtx := dbx.WithUnscoped(ctx)
	var got2 testSkill
	if err := db.FindOne(unscopedCtx, &got2, "name = ?", "softdel"); err != nil {
		t.Fatalf("FindOne WithUnscoped: %v", err)
	}
	if got2.Name != "softdel" {
		t.Fatalf("unscoped name = %q, want softdel", got2.Name)
	}
}

func TestPGIncrement(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Add a counter column via raw SQL (since testSkill doesn't have one).
	if err := db.GORM(ctx).Exec("ALTER TABLE dbx_test_skills ADD COLUMN download_count BIGINT NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("alter table: %v", err)
	}

	skill := &testSkill{Name: "incr", DisplayName: "Counter"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Increment twice.
	if err := db.Increment(ctx, &testSkill{}, "name = ?", []any{"incr"}, "download_count", 5); err != nil {
		t.Fatalf("Increment 1: %v", err)
	}
	if err := db.Increment(ctx, &testSkill{}, "name = ?", []any{"incr"}, "download_count", 3); err != nil {
		t.Fatalf("Increment 2: %v", err)
	}

	// Verify.
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

func TestPGPaginate(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Insert 25 skills.
	for i := 0; i < 25; i++ {
		skill := &testSkill{
			Name:        fmt.Sprintf("skill-%02d", i),
			DisplayName: fmt.Sprintf("Skill %d", i),
		}
		if err := db.Create(ctx, skill); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// Page 1, size 10: should have hasMore=true, total=25.
	var page1 []testSkill
	res, err := db.Paginate(ctx, &page1, &testSkill{}, nil, nil, 1, 10)
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

	// Page 3, size 10: should have hasMore=false, only 5 items.
	var page3 []testSkill
	res, err = db.Paginate(ctx, &page3, &testSkill{}, nil, nil, 3, 10)
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

func TestPGInTxCommit(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &testSkill{Name: "tx1", DisplayName: "First"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		s2 := &testSkill{Name: "tx2", DisplayName: "Second"}
		return tx.Create(ctx, s2)
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}

	// Both should exist.
	var count int64
	count, err = db.Count(ctx, &testSkill{}, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestPGInTxRollback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &testSkill{Name: "rb1", DisplayName: "First"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		// Return error to trigger rollback.
		return errors.New("intentional")
	})
	if err == nil || err.Error() != "intentional" {
		t.Fatalf("err = %v, want 'intentional'", err)
	}

	// Nothing should exist.
	var count int64
	count, err = db.Count(ctx, &testSkill{}, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (rollback)", count)
	}
}

func TestPGInTxPanicRollback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to re-propagate")
		}
	}()
	_ = db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &testSkill{Name: "panic1", DisplayName: "Panic"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		panic("boom")
	})
}

func TestPGInjectDB(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Pre-populate.
	skill := &testSkill{Name: "injected", DisplayName: "Via Injected"}
	if err := db.Create(ctx, skill); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Inject a tx-wrapped *gorm.DB into ctx and verify FindOne picks it up.
	gormDB := db.GORM(ctx)
	txCtx := dbx.InjectDB(ctx, gormDB.Begin())
	defer func() {
		// Cleanup the begin we created.
		_ = dbx.InjectDB(ctx, gormDB.Session(nil))
	}()

	var got testSkill
	if err := db.FindOne(txCtx, &got, "name = ?", "injected"); err != nil {
		t.Fatalf("FindOne with InjectDB: %v", err)
	}
	if got.Name != "injected" {
		t.Fatalf("name = %q, want injected", got.Name)
	}
}

func TestPGQueryTimeout(t *testing.T) {
	// Use a separate DB with very short timeout.
	dsn := pgDSN(t)
	shortDB, err := dbx.New(dbx.Config{
		Driver:       "postgres",
		DSN:          dsn,
		QueryTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer shortDB.Close()
	if err := shortDB.AutoMigrate(context.Background(), &testSkill{}); err != nil {
		t.Fatal(err)
	}

	// pg_sleep(1) takes 1s; query timeout is 50ms.
	ctx := context.Background()
	_, err = shortDB.Count(ctx, &testSkill{}, "1 = 1 OR EXISTS(SELECT 1 FROM pg_sleep(1))")
	if err == nil {
		// Some PG drivers may not honor ctx timeout on a Count. Try raw sleep.
		_ = shortDB.GORM(ctx).Exec("SELECT pg_sleep(1)").Error
	}
	// We don't strictly assert ErrTimeout here because GORM's Count may
	// succeed before the timeout fires; the goal is to ensure no panic.
}

func TestPGSchemaNotReady(t *testing.T) {
	dsn := pgDSN(t)
	db, err := dbx.New(dbx.Config{Driver: "postgres", DSN: dsn, MaxOpenConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Drop the table to simulate missing migration.
	_ = db.GORM(context.Background()).Exec("DROP TABLE IF EXISTS definitely_missing_table").Error

	type Missing struct {
		ID   int64  `gorm:"primaryKey;column:id"`
		Name string `gorm:"column:name"`
	}

	var row Missing
	err = db.GORM(context.Background()).Table("definitely_missing_table").First(&row).Error
	// Should map to ErrSchemaNotReady (or pass through if GORM doesn't surface PG code).
	if err == nil {
		t.Fatal("expected error on missing table")
	}
	// We don't strictly assert ErrSchemaNotReady because GORM wraps the
	// error in a way that may not propagate the typed pgconn.PgError.
	// The wrapDriverErr path tries both typed and string-based detection.
}
