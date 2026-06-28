package base

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepo(t *testing.T) {
	urls := []string{
		// ssh://[user@]host.xz[:port]/path/to/repo.git/
		"ssh://git@github.com:7875/aisphereio/kernel.git",
		// git://host.xz[:port]/path/to/repo.git/
		"git://github.com:7875/aisphereio/kernel.git",
		// http[s]://host.xz[:port]/path/to/repo.git/
		"https://github.com:7875/aisphereio/kernel.git",
		// ftp[s]://host.xz[:port]/path/to/repo.git/
		"ftps://github.com:7875/aisphereio/kernel.git",
		//[user@]host.xz:path/to/repo.git/
		"git@github.com:aisphereio/kernel.git",
		// ssh://[user@]host.xz[:port]/~[user]/path/to/repo.git/
		"ssh://git@github.com:7875/aisphereio/kernel.git",
		// git://host.xz[:port]/~[user]/path/to/repo.git/
		"git://github.com:7875/aisphereio/kernel.git",
		//[user@]host.xz:/~[user]/path/to/repo.git/
		"git@github.com:aisphereio/kernel.git",
		///path/to/repo.git/
		"//github.com/aisphereio/kernel.git",
		// file:///path/to/repo.git/
		"file://./github.com/aisphereio/kernel.git",
	}
	for _, url := range urls {
		dir := repoDir(url)
		if dir != "github.com/aisphereio" && dir != "/aisphereio" {
			t.Fatal(url, "repoDir test failed", dir)
		}
	}
}

func TestRepoClone(t *testing.T) {
	t.Skip("network clone test is kept as a manual smoke test")

	r := NewRepo("https://github.com/aisphereio/service-layout.git", "")
	if err := r.Clone(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.CopyTo(context.Background(), "/tmp/test_repo", "github.com/aisphereio/kernel-layout", nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll("/tmp/test_repo")
	})
}

func TestRepoCopyToLocalPath(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "app")

	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module github.com/aisphereio/kernel-layout\n\ngo 1.25.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(src, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "api", "todo", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "cmd", "main.go"), []byte("package main\n\nimport _ \"github.com/aisphereio/kernel-layout/internal/biz\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "api", "todo", "v1", "todo.pb.go"), []byte("package v1\n\nconst raw = \"github.com/aisphereio/kernel-layout/api/todo/v1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRepo(src, "")
	if err := r.CopyTo(context.Background(), dst, "example.com/app", nil); err != nil {
		t.Fatalf("copying local repo: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(dst, "cmd", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"example.com/app/internal/biz"`) {
		t.Fatalf("expected module replacement in local copy, got:\n%s", out)
	}

	generated, err := os.ReadFile(filepath.Join(dst, "api", "todo", "v1", "todo.pb.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated), "example.com/app") {
		t.Fatalf("generated protobuf descriptor should not be rewritten, got:\n%s", generated)
	}
}
