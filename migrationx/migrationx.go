// Package migrationx provides Kernel's SQL migration integration layer.
//
// It intentionally keeps SQL as the source of truth. Proto describes API and
// access contracts; database structure lives in migrations/ and is applied or
// validated by Kernel boot.
package migrationx

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aisphereio/kernel/dbx"
)

const (
	ModeDisabled    = "disabled"
	ModeDevApply    = "dev_apply"
	ModeApply       = "apply"
	ModeValidate    = "validate"
	ModeGORMDevAuto = "gorm_dev_auto"

	defaultTable = "kernel_schema_migrations"
)

// Config controls service startup migration behavior.
type Config struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	Engine        string `json:"engine" yaml:"engine"`
	Dir           string `json:"dir" yaml:"dir"`
	Table         string `json:"table" yaml:"table"`
	Mode          string `json:"mode" yaml:"mode"`
	FailOnPending bool   `json:"fail_on_pending" yaml:"fail_on_pending"`

	// AllowConcurrent explicitly allows startup-time applying migrations while
	// the deployment has more than one replica. Leave false for production unless
	// an external migration job or database advisory lock strategy is in place.
	AllowConcurrent bool `json:"allow_concurrent" yaml:"allow_concurrent"`
}

func (c Config) Normalize() Config {
	if c.Mode == "" {
		if c.Enabled {
			c.Mode = ModeValidate
		} else {
			c.Mode = ModeDisabled
		}
	}
	if c.Engine == "" {
		c.Engine = "goose"
	}
	if c.Table == "" {
		c.Table = defaultTable
	}
	if c.Dir == "" {
		c.Dir = "migrations"
	}
	return c
}

// Migration is one discovered SQL migration.
type Migration struct {
	Version  string
	Name     string
	Path     string
	UpSQL    string
	DownSQL  string
	Checksum string
}

// Status is one migration's observed state.
type Status struct {
	Version  string
	Name     string
	Applied  bool
	Checksum string
}

// Apply opens the configured migration directory and applies or validates it.
func Apply(ctx context.Context, db dbx.DB, cfg Config) error {
	cfg = cfg.Normalize()
	switch cfg.Mode {
	case ModeDisabled, ModeGORMDevAuto:
		return nil
	case ModeDevApply, ModeApply, ModeValidate:
	default:
		return fmt.Errorf("migrationx: unsupported mode %q", cfg.Mode)
	}
	if db == nil {
		return fmt.Errorf("migrationx: nil db")
	}
	migs, err := LoadDir(os.DirFS(cfg.Dir), ".")
	if err != nil {
		return err
	}
	return ApplyMigrations(ctx, db, cfg, migs)
}

// LoadDir loads goose-compatible SQL migrations from an fs.FS.
//
// Supported forms:
//
//	000001_create_skills.sql       with -- +goose Up / -- +goose Down sections
//	000001_create_skills.up.sql    split up file
//	000001_create_skills.down.sql  split down file
func LoadDir(fsys fs.FS, root string) ([]Migration, error) {
	if root == "" {
		root = "."
	}
	byVersion := map[string]*Migration{}
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		ver, name, kind := parseFileName(base)
		if ver == "" {
			return fmt.Errorf("migrationx: invalid migration filename %q", base)
		}
		m := byVersion[ver]
		if m == nil {
			m = &Migration{Version: ver, Name: name, Path: path}
			byVersion[ver] = m
		}
		sqlText := string(b)
		switch kind {
		case "up":
			m.UpSQL = sqlText
		case "down":
			m.DownSQL = sqlText
		default:
			up, down := parseGooseSections(sqlText)
			m.UpSQL = up
			m.DownSQL = down
		}
		if m.Name == "" {
			m.Name = name
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.UpSQL) == "" {
			return nil, fmt.Errorf("migrationx: migration %s has empty up sql", m.Version)
		}
		sum := sha256.Sum256([]byte(m.UpSQL))
		m.Checksum = hex.EncodeToString(sum[:])
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func parseFileName(base string) (version, name, kind string) {
	base = strings.TrimSuffix(base, ".sql")
	if strings.HasSuffix(base, ".up") {
		kind = "up"
		base = strings.TrimSuffix(base, ".up")
	} else if strings.HasSuffix(base, ".down") {
		kind = "down"
		base = strings.TrimSuffix(base, ".down")
	}
	parts := strings.SplitN(base, "_", 2)
	version = parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	return version, name, kind
}

func parseGooseSections(s string) (up, down string) {
	lines := strings.Split(s, "\n")
	section := ""
	var upb, downb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.EqualFold(trimmed, "-- +goose up"):
			section = "up"
			continue
		case strings.EqualFold(trimmed, "-- +goose down"):
			section = "down"
			continue
		}
		switch section {
		case "up":
			upb.WriteString(line)
			upb.WriteByte('\n')
		case "down":
			downb.WriteString(line)
			downb.WriteByte('\n')
		}
	}
	if upb.Len() == 0 {
		return s, ""
	}
	return upb.String(), downb.String()
}

// ApplyMigrations applies or validates a prepared migration set.
func ApplyMigrations(ctx context.Context, db dbx.DB, cfg Config, migrations []Migration) error {
	cfg = cfg.Normalize()
	g := db.GORM(ctx)
	sqlDB, err := g.DB()
	if err != nil {
		return err
	}
	if cfg.Mode == ModeValidate {
		return Validate(ctx, sqlDB, cfg, migrations)
	}
	if err := ensureTable(ctx, sqlDB, cfg.Table); err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, sqlDB, cfg.Table)
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if a, ok := applied[m.Version]; ok {
			if a.Checksum != "" && a.Checksum != m.Checksum {
				return fmt.Errorf("migrationx: checksum mismatch for version %s", m.Version)
			}
			continue
		}
		if err := execStatements(ctx, sqlDB, m.UpSQL); err != nil {
			return fmt.Errorf("migrationx: apply %s %s: %w", m.Version, m.Name, err)
		}
		if err := insertApplied(ctx, sqlDB, cfg.Table, m); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks pending and checksum drift without applying SQL.
func Validate(ctx context.Context, sqlDB *sql.DB, cfg Config, migrations []Migration) error {
	cfg = cfg.Normalize()
	if err := ensureTable(ctx, sqlDB, cfg.Table); err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, sqlDB, cfg.Table)
	if err != nil {
		return err
	}
	pending := make([]string, 0)
	for _, m := range migrations {
		a, ok := applied[m.Version]
		if !ok {
			pending = append(pending, m.Version+"_"+m.Name)
			continue
		}
		if a.Checksum != "" && a.Checksum != m.Checksum {
			return fmt.Errorf("migrationx: checksum mismatch for version %s", m.Version)
		}
	}
	if len(pending) > 0 && cfg.FailOnPending {
		return fmt.Errorf("migrationx: pending migrations: %s", strings.Join(pending, ", "))
	}
	return nil
}

type appliedMigration struct{ Version, Checksum string }

func ensureTable(ctx context.Context, db *sql.DB, table string) error {
	if table == "" {
		table = defaultTable
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
version VARCHAR(128) PRIMARY KEY,
name VARCHAR(256) NOT NULL,
checksum VARCHAR(128) NOT NULL,
applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`, table))
	return err
}

func appliedVersions(ctx context.Context, db *sql.DB, table string) (map[string]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT version, checksum FROM %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]appliedMigration{}
	for rows.Next() {
		var v, c string
		if err := rows.Scan(&v, &c); err != nil {
			return nil, err
		}
		out[v] = appliedMigration{Version: v, Checksum: c}
	}
	return out, rows.Err()
}

func insertApplied(ctx context.Context, db *sql.DB, table string, m Migration) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (version, name, checksum, applied_at) VALUES (%s, %s, %s, CURRENT_TIMESTAMP)",
		table, sqlLiteral(m.Version), sqlLiteral(m.Name), sqlLiteral(m.Checksum)))
	return err
}

func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func execStatements(ctx context.Context, db *sql.DB, sqlText string) error {
	for _, stmt := range splitStatements(sqlText) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitStatements(sqlText string) []string {
	// P0 simple runner with goose-style StatementBegin/StatementEnd support.
	// Complex PostgreSQL functions/triggers should be wrapped in:
	//   -- +goose StatementBegin
	//   ... semicolon rich SQL ...
	//   -- +goose StatementEnd
	// For production-grade dialect parsing, plug a real goose Migrator behind
	// Kernel migrationx instead of hand-writing SQL splitting in business code.
	var out []string
	var current strings.Builder
	inBlock := false
	flush := func() {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
			out = append(out, stmt)
		}
		current.Reset()
	}

	for _, line := range strings.Split(sqlText, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch lower {
		case "-- +goose statementbegin":
			flush()
			inBlock = true
			continue
		case "-- +goose statementend":
			inBlock = false
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		if inBlock {
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}
		parts := strings.Split(line, ";")
		for i, part := range parts {
			current.WriteString(part)
			if i < len(parts)-1 {
				flush()
			} else {
				current.WriteByte('\n')
			}
		}
	}
	flush()
	return out
}
