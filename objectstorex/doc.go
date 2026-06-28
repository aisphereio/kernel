// Package objectstorex provides the unified object storage API for Aisphere Kernel.
//
// objectstorex is the ONLY object storage abstraction that business code
// should depend on. It exposes a stable Client interface backed by Minio
// (which is S3-compatible), with context propagation, presign URL, and
// multipart upload support.
//
// # Quickstart
//
//	import (
//	    "github.com/aisphereio/kernel/objectstorex"
//	    _ "github.com/aisphereio/kernel/objectstorex/minio" // register "minio" driver
//	)
//
//	store, err := objectstorex.New(objectstorex.Config{
//	    Driver:   "minio",
//	    Endpoint: "localhost:9000",
//	    UseSSL:   false,
//	    AccessKey: "minioadmin",
//	    SecretKey: "minioadmin",
//	    Bucket:   "aihub",
//	    Region:   "us-east-1",
//	})
//	if err != nil { return err }
//	defer store.Close()
//
//	// Upload
//	_, err = store.PutObject(ctx, "skills/demo/package.zip", bytes.NewReader(data), int64(len(data)),
//	    objectstorex.PutOptions{ContentType: "application/zip"})
//
//	// Download
//	rc, info, err := store.GetObject(ctx, "skills/demo/package.zip", objectstorex.GetOptions{})
//	defer rc.Close()
//
//	// Presign URL for frontend direct upload
//	url, err := store.PresignPut(ctx, "skills/demo/package.zip", 15*time.Minute)
//
// # Drivers
//
//	import _ "github.com/aisphereio/kernel/objectstorex/minio" // registers "minio"
//
// The minio driver uses github.com/minio/minio-go/v7 and is compatible with
// any S3-compatible service (AWS S3, Minio, DigitalOcean Spaces, etc.).
//
// # Forbidden patterns
//
// Do not import `github.com/minio/minio-go/v7` in business code. Use the
// objectstorex.Client interface.
package objectstorex
