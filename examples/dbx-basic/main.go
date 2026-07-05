// Package main demonstrates the basic dbx flow with Postgres, mirroring
// aisphere-hub's skill.go patterns.
//
// This example requires a running Postgres instance. Set KERNEL_DBX_PG_DSN:
//
//	export KERNEL_DBX_PG_DSN=postgres://user:pass@localhost:5432/testdb?sslmode=disable
//	go run ./examples/dbx-basic
//
// If the env var is not set, the example prints usage and exits.
//
// Expected output (with a real DB):
//
//	opened db (driver=postgres)
//	migrated dbx_basic_skills table
//	created skill id=1 name=demo owner=user_123
//	found skill id=1 name=demo owner=user_123
//	upserted skill: display_name=Demo V2, owner_id=user_123 (unchanged)
//	incremented download_count to 5
//	paginated: page=1 size=10 total=12 hasMore=true
//	soft-deleted skill demo
//	unscoped query found soft-deleted skill: demo
//	dropped table dbx_basic_skills
//	done
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aisphereio/kernel/dbx"
	_ "github.com/aisphereio/kernel/dbx/postgres"
	"gorm.io/gorm"
)

// Skill mirrors aisphere-hub/internal/data/skillModel (subset).
type Skill struct {
	ID            int64          `gorm:"primaryKey;autoIncrement;column:id"`
	Name          string         `gorm:"column:name;size:128;uniqueIndex;not null"`
	DisplayName   string         `gorm:"column:display_name;size:256;not null;default:''"`
	OwnerID       string         `gorm:"column:owner_id;size:128;not null;default:''"`
	Status        string         `gorm:"column:status;size:32;not null;default:'active'"`
	DownloadCount int64          `gorm:"column:download_count;not null;default:0"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null;autoUpdateTime"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Skill) TableName() string { return "dbx_basic_skills" }

func main() {
	dsn := os.Getenv("KERNEL_DBX_PG_DSN")
	if dsn == "" {
		fmt.Println("set KERNEL_DBX_PG_DSN to enable this example")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  export KERNEL_DBX_PG_DSN=postgres://user:pass@localhost:5432/testdb?sslmode=disable")
		fmt.Println("  go run ./examples/dbx-basic")
		return
	}

	db, err := dbx.New(dbx.Config{
		Driver:             "postgres",
		DSN:                dsn,
		MaxOpenConns:       4,
		MaxIdleConns:       2,
		ConnMaxLifetime:    30 * time.Second,
		QueryTimeout:       5 * time.Second,
		SlowQueryThreshold: 200 * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	fmt.Printf("opened db (driver=%s)\n", db.DriverName())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// AutoMigrate (dev only — production uses SQL migrations).
	if err := db.AutoMigrate(ctx, &Skill{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Println("migrated dbx_basic_skills table")
	defer func() {
		_ = db.GORM(ctx).Exec("DROP TABLE IF EXISTS dbx_basic_skills").Error
		fmt.Println("dropped table dbx_basic_skills")
	}()

	// Truncate for clean start.
	if err := db.GORM(ctx).Exec("TRUNCATE dbx_basic_skills RESTART IDENTITY CASCADE").Error; err != nil {
		log.Fatalf("truncate: %v", err)
	}

	// Create.
	skill := &Skill{Name: "demo", DisplayName: "Demo Skill", OwnerID: "user_123"}
	if err := db.Create(ctx, skill); err != nil {
		if errors.Is(err, dbx.ErrDuplicateKey) {
			fmt.Println("skill already exists (duplicate key handled)")
		} else {
			log.Fatalf("create: %v", err)
		}
	}
	fmt.Printf("created skill id=%d name=%s owner=%s\n", skill.ID, skill.Name, skill.OwnerID)

	// FindOne.
	var found Skill
	if err := db.FindOne(ctx, &found, "name = ?", "demo"); err != nil {
		log.Fatalf("find: %v", err)
	}
	fmt.Printf("found skill id=%d name=%s owner=%s\n", found.ID, found.Name, found.OwnerID)

	// FindOne missing → ErrNoRows.
	var missing Skill
	if err := db.FindOne(ctx, &missing, "name = ?", "nope"); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			fmt.Println("ErrNoRows handled: skill 'nope' correctly reported missing")
		} else {
			log.Fatalf("unexpected error: %v", err)
		}
	}

	// SafeUpsert: whitelist display_name and status; owner_id is protected.
	updated := &Skill{
		ID:          found.ID,
		Name:        "demo",
		DisplayName: "Demo V2",
		OwnerID:     "attacker_456", // will NOT be overwritten
		Status:      "archived",
	}
	if err := db.SafeUpsert(ctx, updated, []string{"display_name", "status"}); err != nil {
		log.Fatalf("safe upsert: %v", err)
	}
	var afterUpsert Skill
	_ = db.FindOne(ctx, &afterUpsert, "name = ?", "demo")
	fmt.Printf("upserted skill: display_name=%s, owner_id=%s (unchanged)\n",
		afterUpsert.DisplayName, afterUpsert.OwnerID)

	// SafeUpsert with protected column in whitelist → ErrUnsafeUpsert.
	if err := db.SafeUpsert(ctx, updated, []string{"display_name", "owner_id"}); err != nil {
		if errors.Is(err, dbx.ErrUnsafeUpsert) {
			fmt.Println("ErrUnsafeUpsert handled: owner_id correctly blocked from whitelist")
		}
	}

	// Increment.
	if err := db.Increment(ctx, &Skill{}, "name = ?", []any{"demo"}, "download_count", 5); err != nil {
		log.Fatalf("increment: %v", err)
	}
	var afterInc Skill
	_ = db.FindOne(ctx, &afterInc, "name = ?", "demo")
	fmt.Printf("incremented download_count to %d\n", afterInc.DownloadCount)

	// Insert 11 more skills for pagination demo.
	for i := 1; i <= 11; i++ {
		s := &Skill{
			Name:        fmt.Sprintf("skill-%02d", i),
			DisplayName: fmt.Sprintf("Skill %d", i),
			OwnerID:     "user_123",
		}
		_ = db.Create(ctx, s)
	}

	// Paginate.
	var page []Skill
	res, err := db.Paginate(ctx, &page, &Skill{}, nil, nil, 1, 10)
	if err != nil {
		log.Fatalf("paginate: %v", err)
	}
	fmt.Printf("paginated: page=%d size=%d total=%d hasMore=%v\n",
		res.Page, res.Size, res.Total, res.HasMore)

	// Transaction with auto commit/rollback.
	err = db.InTx(ctx, func(tx dbx.Tx) error {
		s1 := &Skill{Name: "tx-a", DisplayName: "TX A"}
		if err := tx.Create(ctx, s1); err != nil {
			return err
		}
		s2 := &Skill{Name: "tx-b", DisplayName: "TX B"}
		return tx.Create(ctx, s2)
	})
	if err != nil {
		log.Fatalf("InTx: %v", err)
	}
	fmt.Println("transaction committed: 2 skills created")

	// Soft delete.
	if err := db.Delete(ctx, &Skill{}, "name = ?", "demo"); err != nil {
		log.Fatalf("delete: %v", err)
	}
	fmt.Println("soft-deleted skill demo")

	// Normal query: should not find it.
	var deleted Skill
	if err := db.FindOne(ctx, &deleted, "name = ?", "demo"); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			fmt.Println("normal query correctly hides soft-deleted skill")
		}
	}

	// Unscoped query: should find it.
	unscopedCtx := dbx.WithUnscoped(ctx)
	var unscoped Skill
	if err := db.FindOne(unscopedCtx, &unscoped, "name = ?", "demo"); err != nil {
		log.Fatalf("unscoped find: %v", err)
	}
	fmt.Printf("unscoped query found soft-deleted skill: %s\n", unscoped.Name)

	fmt.Println("done")
}
