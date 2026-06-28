package objectstorex_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aisphereio/kernel/objectstorex"
	"github.com/aisphereio/kernel/objectstorex/minio"
)

func minioConfig(t *testing.T) objectstorex.Config {
	t.Helper()
	endpoint := os.Getenv("KERNEL_OBJECTSTOREX_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("set KERNEL_OBJECTSTOREX_MINIO_ENDPOINT to enable minio integration tests")
	}
	return objectstorex.Config{
		Driver:       "minio",
		Endpoint:     endpoint,
		UseSSL:       false,
		AccessKey:    getEnv("KERNEL_OBJECTSTOREX_MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:    getEnv("KERNEL_OBJECTSTOREX_MINIO_SECRET_KEY", "minioadmin"),
		Bucket:       getEnv("KERNEL_OBJECTSTOREX_MINIO_BUCKET", "test-bucket"),
		Region:       "",
		EnsureBucket: true,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestIntegrationMinioPutGet(t *testing.T) {
	cfg := minioConfig(t)
	store, err := minio.NewDirectClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	data := []byte("integration test data")
	key := "integration-test/put-get.txt"

	_, err = store.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), objectstorex.PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"test": "true"},
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	defer store.DeleteObject(ctx, key)

	rc, info, err := store.GetObject(ctx, key, objectstorex.GetOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != "integration test data" {
		t.Fatalf("got %q, want 'integration test data'", got)
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("ContentType = %q, want text/plain", info.ContentType)
	}
}

func TestIntegrationMinioListAndPresign(t *testing.T) {
	cfg := minioConfig(t)
	store, err := minio.NewDirectClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	// Put 3 objects
	for _, k := range []string{"list-test/a.txt", "list-test/b.txt", "list-test/c.txt"} {
		_, _ = store.PutObject(ctx, k, bytes.NewReader([]byte("x")), 1, objectstorex.PutOptions{})
		defer store.DeleteObject(ctx, k)
	}

	// List with prefix
	items, err := store.ListObjects(ctx, objectstorex.ListOptions{Prefix: "list-test/", Recursive: true})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("len = %d, want 3", len(items))
	}

	// Presign
	url, err := store.PresignGet(ctx, "list-test/a.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if url == "" {
		t.Fatal("PresignGet returned empty URL")
	}
}
