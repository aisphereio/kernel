// Package main demonstrates basic objectstorex usage with Minio.
//
// Run:
//
//	export KERNEL_OBJECTSTOREX_MINIO_ENDPOINT=localhost:9000
//	go run ./examples/objectstorex-basic
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aisphereio/kernel/objectstorex"
	_ "github.com/aisphereio/kernel/objectstorex/minio"
)

func main() {
	endpoint := os.Getenv("KERNEL_OBJECTSTOREX_MINIO_ENDPOINT")
	if endpoint == "" {
		fmt.Println("set KERNEL_OBJECTSTOREX_MINIO_ENDPOINT to enable this example")
		return
	}

	store, err := objectstorex.New(objectstorex.Config{
		Driver:       "minio",
		Endpoint:     endpoint,
		UseSSL:       false,
		AccessKey:    getEnv("KERNEL_OBJECTSTOREX_MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:    getEnv("KERNEL_OBJECTSTOREX_MINIO_SECRET_KEY", "minioadmin"),
		Bucket:       getEnv("KERNEL_OBJECTSTOREX_MINIO_BUCKET", "test-bucket"),
		EnsureBucket: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "new store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()

	// Put
	data := []byte("hello object storage!")
	key := "demo/hello.txt"
	info, err := store.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), objectstorex.PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"author": "demo"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "PutObject: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("PutObject %s (size=%d, etag=%s)\n", key, info.Size, info.ETag)

	// Get
	rc, info2, err := store.GetObject(ctx, key, objectstorex.GetOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetObject: %v\n", err)
		os.Exit(1)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	fmt.Printf("GetObject %s = %q (size=%d, type=%s)\n", key, string(got), info2.Size, info2.ContentType)

	// Stat
	stat, _ := store.StatObject(ctx, key)
	fmt.Printf("StatObject %s = size=%d, modified=%s\n", key, stat.Size, stat.LastModified.Format(time.RFC3339))

	// List
	items, _ := store.ListObjects(ctx, objectstorex.ListOptions{Prefix: "demo/", Recursive: true})
	fmt.Printf("ListObjects demo/ = %d items\n", len(items))

	// Copy
	_, _ = store.CopyObject(ctx, key, "demo/hello-copy.txt", objectstorex.PutOptions{ContentType: "text/plain"})
	fmt.Println("CopyObject demo/hello.txt → demo/hello-copy.txt")

	// Presign URL
	url, _ := store.PresignGet(ctx, key, 15*time.Minute)
	fmt.Printf("PresignGet %s (expires in 15min)\n", url[:min(len(url), 60)])

	// Delete
	_ = store.DeleteObject(ctx, key)
	_ = store.DeleteObject(ctx, "demo/hello-copy.txt")
	fmt.Println("Deleted both objects")

	fmt.Println("done")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
