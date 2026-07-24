package migrationx

import (
	"strings"
	"testing"
)

func TestSplitStatementsPreservesSemicolonsInSQLContexts(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		wantCount    int
		wantContains []string
	}{
		{
			name: "single quoted comment text",
			sql: `COMMENT ON COLUMN aihub_skillsets.revision IS
    'Monotonic SkillSet snapshot revision; incremented whenever metadata or members change.';
SELECT 1;`,
			wantCount: 2,
			wantContains: []string{
				"snapshot revision; incremented",
				"SELECT 1",
			},
		},
		{
			name: "escaped quote and quoted identifiers",
			sql: `SELECT 'it''s; safe';
SELECT "semi;colon" FROM t;
SELECT ` + "`mysql;identifier`" + ` FROM t;`,
			wantCount: 3,
			wantContains: []string{
				"it''s; safe",
				`"semi;colon"`,
				"`mysql;identifier`",
			},
		},
		{
			name: "postgres dollar quoted bodies",
			sql: `DO $$
BEGIN
  PERFORM 1;
END;
$$;
DO $body$
BEGIN
  PERFORM 2;
END;
$body$;`,
			wantCount: 2,
			wantContains: []string{
				"PERFORM 1;",
				"PERFORM 2;",
			},
		},
		{
			name: "line and nested block comments",
			sql: `-- leading comment; remains attached to the statement
SELECT 1 /* outer; /* nested; */ done; */;
SELECT 2; -- trailing comment; must not become a statement`,
			wantCount: 2,
			wantContains: []string{
				"nested;",
				"SELECT 2",
			},
		},
		{
			name:      "dollar signs inside identifiers",
			sql:       `SELECT foo$tag$bar; SELECT 2;`,
			wantCount: 2,
			wantContains: []string{
				"foo$tag$bar",
				"SELECT 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts := splitStatements(tt.sql)
			if len(stmts) != tt.wantCount {
				t.Fatalf("expected %d statements, got %d: %#v", tt.wantCount, len(stmts), stmts)
			}
			for i, want := range tt.wantContains {
				if !strings.Contains(stmts[i], want) {
					t.Fatalf("statement %d does not contain %q: %q", i, want, stmts[i])
				}
			}
		})
	}
}

func TestSplitStatementsKeepsGooseStatementBlock(t *testing.T) {
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
