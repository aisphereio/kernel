package migrationx

import (
	"testing"

	"github.com/pressly/goose/v3"
)

func TestNormalizeDefaultsToRealGoose(t *testing.T) {
	cfg := (Config{Enabled: true}).Normalize()
	if cfg.Engine != EngineGoose {
		t.Fatalf("expected %q engine, got %q", EngineGoose, cfg.Engine)
	}
	if cfg.Mode != ModeValidate {
		t.Fatalf("expected validate mode, got %q", cfg.Mode)
	}
}

func TestNormalizeFailsClosedConcurrentFlag(t *testing.T) {
	cfg := (Config{Enabled: true, Mode: ModeApply, AllowConcurrent: true}).Normalize()
	if cfg.AllowConcurrent {
		t.Fatal("AllowConcurrent must remain disabled until a concrete migration locker is configured")
	}
}

func TestGooseDialect(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		want   goose.Dialect
		ok     bool
	}{
		{name: "postgres", driver: "postgres", want: goose.DialectPostgres, ok: true},
		{name: "pgx", driver: "pgx", want: goose.DialectPostgres, ok: true},
		{name: "mysql", driver: "mysql", want: goose.DialectMySQL, ok: true},
		{name: "unsupported", driver: "sqlite", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gooseDialect(tt.driver)
			if tt.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Fatalf("got dialect %q want %q", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected unsupported driver error, got dialect %q", got)
			}
		})
	}
}
