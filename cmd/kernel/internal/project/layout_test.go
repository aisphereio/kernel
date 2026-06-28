package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLayoutUsesRepoWhenProvided(t *testing.T) {
	got, err := resolveLayout("https://example.com/layout.git", t.TempDir())
	if err != nil {
		t.Fatalf("resolve layout: %v", err)
	}
	if got != "https://example.com/layout.git" {
		t.Fatalf("expected explicit repo to win, got %q", got)
	}
}

func TestResolveLayoutDefaultsToLocalLayout(t *testing.T) {
	root := t.TempDir()
	layout := filepath.Join(root, "layout")
	if err := os.Mkdir(layout, 0o755); err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(root, "services")
	if err := os.Mkdir(wd, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveLayout("", wd)
	if err != nil {
		t.Fatalf("resolve layout: %v", err)
	}
	if got != layout {
		t.Fatalf("expected local layout %q, got %q", layout, got)
	}
}

func TestDefaultScaffoldOptionsIncludeKernelCapabilities(t *testing.T) {
	opts := defaultScaffoldOptions()
	want := []string{"dbx", "cachex", "objectstorex", "authn", "authz", "auditx"}
	for _, feature := range want {
		if !opts.HasFeature(feature) {
			t.Fatalf("expected default feature %q in %#v", feature, opts.Features)
		}
	}
	if opts.DBDriver != "postgres" || opts.CacheDriver != "redis" || opts.ObjectStoreDriver != "minio" {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
}
