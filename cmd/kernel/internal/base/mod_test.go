package base

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleVersion(t *testing.T) {
	v, err := ModuleVersion("golang.org/x/mod")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(v)
}

func TestModulePath(t *testing.T) {
	dir := t.TempDir()
	modFile := filepath.Join(dir, "go.mod")

	f, err := os.Create(modFile)
	if err != nil {
		t.Fatal(err)
	}

	mod := `module github.com/aisphereio/kernel

go 1.21`
	_, err = f.WriteString(mod)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	p, err := ModulePath(modFile)
	if err != nil {
		t.Fatal(err)
	}
	if p != "github.com/aisphereio/kernel" {
		t.Fatalf("want: %s, got: %s", "github.com/aisphereio/kernel", p)
	}
}
