package objectstorex_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aisphereio/kernel/objectstorex"
)

// fakeStore is an in-memory implementation of objectstorex.Client for testing.
type fakeStore struct {
	bucket  string
	objects map[string][]byte
	meta    map[string]objectstorex.ObjectInfo
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		bucket:  "test-bucket",
		objects: map[string][]byte{},
		meta:    map[string]objectstorex.ObjectInfo{},
	}
}

func (f *fakeStore) Bucket() string                             { return f.bucket }
func (f *fakeStore) BucketExists(context.Context) (bool, error) { return true, nil }
func (f *fakeStore) EnsureBucket(context.Context) error         { return nil }
func (f *fakeStore) DriverName() string                         { return "fake" }
func (f *fakeStore) Close() error                               { return nil }

func (f *fakeStore) PutObject(_ context.Context, key string, body objectstorex.ReadSeeker, size int64, opts objectstorex.PutOptions) (objectstorex.ObjectInfo, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return objectstorex.ObjectInfo{}, err
	}
	f.objects[key] = data
	info := objectstorex.ObjectInfo{
		Bucket:       f.bucket,
		Key:          key,
		Size:         int64(len(data)),
		ContentType:  opts.ContentType,
		Metadata:     opts.Metadata,
		LastModified: time.Now(),
	}
	f.meta[key] = info
	return info, nil
}

func (f *fakeStore) GetObject(_ context.Context, key string, _ objectstorex.GetOptions) (objectstorex.ReadCloser, objectstorex.ObjectInfo, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, objectstorex.ObjectInfo{}, objectstorex.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), f.meta[key], nil
}

func (f *fakeStore) DeleteObject(_ context.Context, key string) error {
	delete(f.objects, key)
	delete(f.meta, key)
	return nil
}

func (f *fakeStore) StatObject(_ context.Context, key string) (objectstorex.ObjectInfo, error) {
	info, ok := f.meta[key]
	if !ok {
		return objectstorex.ObjectInfo{}, objectstorex.ErrNotFound
	}
	return info, nil
}

func (f *fakeStore) ListObjects(_ context.Context, opts objectstorex.ListOptions) ([]objectstorex.ObjectInfo, error) {
	var out []objectstorex.ObjectInfo
	for key, info := range f.meta {
		if opts.Prefix == "" || hasPrefix(key, opts.Prefix) {
			out = append(out, info)
			if opts.MaxKeys > 0 && len(out) >= opts.MaxKeys {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeStore) CopyObject(ctx context.Context, srcKey, dstKey string, opts objectstorex.PutOptions) (objectstorex.ObjectInfo, error) {
	data, ok := f.objects[srcKey]
	if !ok {
		return objectstorex.ObjectInfo{}, objectstorex.ErrNotFound
	}
	f.objects[dstKey] = data
	info := objectstorex.ObjectInfo{
		Bucket:      f.bucket,
		Key:         dstKey,
		Size:        int64(len(data)),
		ContentType: opts.ContentType,
		Metadata:    opts.Metadata,
	}
	f.meta[dstKey] = info
	return info, nil
}

func (f *fakeStore) PresignPut(context.Context, string, time.Duration) (string, error) {
	return "https://fake-presign-put-url", nil
}

func (f *fakeStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://fake-presign-get-url", nil
}

func (f *fakeStore) InitMultipartUpload(context.Context, string, objectstorex.PutOptions) (string, error) {
	return "fake-upload-id", nil
}

func (f *fakeStore) UploadPart(_ context.Context, _ string, _ string, _ int, body objectstorex.ReadSeeker, _ int64) (string, error) {
	_, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return "fake-etag", nil
}

func (f *fakeStore) CompleteMultipartUpload(context.Context, string, string, []objectstorex.CompletedPart) (objectstorex.ObjectInfo, error) {
	return objectstorex.ObjectInfo{}, nil
}

func (f *fakeStore) AbortMultipartUpload(context.Context, string, string) error { return nil }

func hasPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ===== Tests =====

func TestPutAndGet(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	data := []byte("hello world")
	info, err := store.PutObject(ctx, "test/file.txt", bytes.NewReader(data), int64(len(data)), objectstorex.PutOptions{
		ContentType: "text/plain",
		Metadata:    map[string]string{"author": "test"},
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if info.Size != int64(len(data)) {
		t.Fatalf("size = %d, want %d", info.Size, len(data))
	}

	rc, info2, err := store.GetObject(ctx, "test/file.txt", objectstorex.GetOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != "hello world" {
		t.Fatalf("got %q, want 'hello world'", got)
	}
	if info2.ContentType != "text/plain" {
		t.Fatalf("ContentType = %q, want text/plain", info2.ContentType)
	}
}

func TestGetNotFound(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	_, _, err := store.GetObject(ctx, "missing", objectstorex.GetOptions{})
	if !errors.Is(err, objectstorex.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	_, _ = store.PutObject(ctx, "to-delete", bytes.NewReader([]byte("x")), 1, objectstorex.PutOptions{})
	if err := store.DeleteObject(ctx, "to-delete"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	_, _, err := store.GetObject(ctx, "to-delete", objectstorex.GetOptions{})
	if !errors.Is(err, objectstorex.ErrNotFound) {
		t.Fatalf("err after delete = %v, want ErrNotFound", err)
	}
}

func TestStat(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	_, _ = store.PutObject(ctx, "stat-test", bytes.NewReader([]byte("stat me")), 7, objectstorex.PutOptions{ContentType: "text/plain"})

	info, err := store.StatObject(ctx, "stat-test")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if info.Size != 7 {
		t.Fatalf("Size = %d, want 7", info.Size)
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("ContentType = %q, want text/plain", info.ContentType)
	}
}

func TestList(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	_, _ = store.PutObject(ctx, "dir/a.txt", bytes.NewReader([]byte("a")), 1, objectstorex.PutOptions{})
	_, _ = store.PutObject(ctx, "dir/b.txt", bytes.NewReader([]byte("b")), 1, objectstorex.PutOptions{})
	_, _ = store.PutObject(ctx, "other.txt", bytes.NewReader([]byte("o")), 1, objectstorex.PutOptions{})

	items, err := store.ListObjects(ctx, objectstorex.ListOptions{Prefix: "dir/", Recursive: true})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
}

func TestCopy(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	_, _ = store.PutObject(ctx, "src.txt", bytes.NewReader([]byte("copy me")), 7, objectstorex.PutOptions{ContentType: "text/plain"})

	_, err := store.CopyObject(ctx, "src.txt", "dst.txt", objectstorex.PutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("CopyObject: %v", err)
	}

	rc, _, err := store.GetObject(ctx, "dst.txt", objectstorex.GetOptions{})
	if err != nil {
		t.Fatalf("GetObject dst: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "copy me" {
		t.Fatalf("got %q, want 'copy me'", got)
	}
}

func TestPresign(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	url, err := store.PresignPut(ctx, "key", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	if url == "" {
		t.Fatal("PresignPut returned empty URL")
	}

	url, err = store.PresignGet(ctx, "key", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if url == "" {
		t.Fatal("PresignGet returned empty URL")
	}
}

func TestBucket(t *testing.T) {
	store := newFakeStore()

	if store.Bucket() != "test-bucket" {
		t.Fatalf("Bucket = %q, want test-bucket", store.Bucket())
	}

	exists, err := store.BucketExists(context.Background())
	if err != nil || !exists {
		t.Fatalf("BucketExists = %v, %v, want true, nil", exists, err)
	}

	if err := store.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
}

func TestMultipartUpload(t *testing.T) {
	store := newFakeStore()
	ctx := context.Background()

	uploadID, err := store.InitMultipartUpload(ctx, "big-file", objectstorex.PutOptions{ContentType: "application/octet-stream"})
	if err != nil {
		t.Fatalf("InitMultipartUpload: %v", err)
	}
	if uploadID == "" {
		t.Fatal("uploadID is empty")
	}

	etag, err := store.UploadPart(ctx, "big-file", uploadID, 1, bytes.NewReader([]byte("part1")), 5)
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if etag == "" {
		t.Fatal("etag is empty")
	}

	parts := []objectstorex.CompletedPart{{PartNumber: 1, ETag: etag}}
	_, err = store.CompleteMultipartUpload(ctx, "big-file", uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	// Abort should also work without error.
	_ = store.AbortMultipartUpload(ctx, "big-file", "another-upload-id")
}
