package migrationx

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadDirGooseCompatibleSQL(t *testing.T) {
	fsys := fstest.MapFS{
		"000001_create_skills.sql": {Data: []byte(`-- +goose Up
CREATE TABLE skills(id text primary key);
-- +goose Down
DROP TABLE skills;
`)},
	}
	migs, err := LoadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 1 {
		t.Fatalf("expected one migration, got %d", len(migs))
	}
	if migs[0].Version != "000001" || migs[0].Name != "create_skills" {
		t.Fatalf("unexpected migration: %+v", migs[0])
	}
	if migs[0].Checksum == "" {
		t.Fatal("checksum should be set")
	}
}

func TestLoadDirSplitSQL(t *testing.T) {
	fsys := fstest.MapFS{
		"000001_create_skills.up.sql":   {Data: []byte(`CREATE TABLE skills(id text primary key);`)},
		"000001_create_skills.down.sql": {Data: []byte(`DROP TABLE skills;`)},
	}
	migs, err := LoadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if got := migs[0].DownSQL; got == "" {
		t.Fatal("down sql should be loaded")
	}
}

func TestSplitStatementsWithStatementBegin(t *testing.T) {
	stmts := splitStatements(`CREATE TABLE t(id text);
-- +goose StatementBegin
CREATE FUNCTION f() RETURNS void AS $$
BEGIN
  PERFORM 1;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE INDEX idx_t_id ON t(id);
`)
	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[1], "PERFORM 1;") {
		t.Fatalf("function body was split incorrectly: %q", stmts[1])
	}
}
