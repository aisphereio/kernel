package objectstorex

import (
        "context"
        "errors"
        "time"
)

// Public sentinel errors.
var (
        // ErrNotFound is returned by GetObject / StatObject when the key does not exist.
        ErrNotFound = errors.New("objectstorex: object not found")

        // ErrNilConfig is returned by New when Config is missing required fields.
        ErrNilConfig = errors.New("objectstorex: config is missing required fields")

        // ErrUnknownDriver is returned when the driver name has not been registered.
        ErrUnknownDriver = errors.New("objectstorex: unknown driver (did you import objectstorex/minio?)")

        // ErrBucketNotExists is returned when the configured bucket does not exist.
        ErrBucketNotExists = errors.New("objectstorex: bucket does not exist")

        // ErrClosed is returned when a closed store is used.
        ErrClosed = errors.New("objectstorex: store is closed")
)

// Config holds the configuration for an object store connection.
type Config struct {
        // Driver selects the registered driver: "minio".
        Driver string `json:"driver"`

        // Endpoint is the S3-compatible endpoint URL.
        // Minio: "localhost:9000"
        // AWS S3: "s3.us-east-1.amazonaws.com"
        Endpoint string `json:"endpoint"`

        // UseSSL enables HTTPS.
        UseSSL bool `json:"use_ssl"`

        // AccessKey is the access key ID.
        AccessKey string `json:"access_key"`

        // SecretKey is the secret access key.
        SecretKey string `json:"secret_key"`

        // Bucket is the default bucket name.
        Bucket string `json:"bucket"`

        // Region is the S3 region (leave empty for Minio).
        Region string `json:"region"`

        // EnsureBucket, if true, creates the bucket on startup if it doesn't exist.
        EnsureBucket bool `json:"ensure_bucket"`

        // PresignExpiry is the default expiry for presigned URLs.
        // Zero means 15 minutes.
        PresignExpiry time.Duration `json:"presign_expiry_ns"`
}

// Validate returns ErrNilConfig if required fields are missing.
func (c Config) Validate() error {
        if c.Driver == "" || c.Endpoint == "" || c.AccessKey == "" || c.SecretKey == "" {
                return ErrNilConfig
        }
        return nil
}

// ============================================================================
// Client interface
// ============================================================================

// Client is the runtime object store interface used by kernel modules and apps.
//
// All methods accept context.Context. It mirrors the aisphere-hub
// objectstore.Client interface exactly so existing code can adopt
// objectstorex with minimal changes.
type Client interface {
        // --- Bucket management ---

        // Bucket returns the default bucket name.
        Bucket() string

        // BucketExists checks if the configured bucket exists.
        BucketExists(ctx context.Context) (bool, error)

        // EnsureBucket creates the bucket if it doesn't exist. Idempotent.
        EnsureBucket(ctx context.Context) error

        // --- CRUD ---

        // PutObject uploads an object. body is the reader; size is the content
        // length (use -1 if unknown). Returns ObjectInfo on success.
        PutObject(ctx context.Context, key string, body ReadSeeker, size int64, opts PutOptions) (ObjectInfo, error)

        // GetObject downloads an object. Returns a ReadCloser (caller must close)
        // and ObjectInfo. Returns ErrNotFound if the key doesn't exist.
        GetObject(ctx context.Context, key string, opts GetOptions) (ReadCloser, ObjectInfo, error)

        // DeleteObject removes an object. No-op if the key doesn't exist.
        DeleteObject(ctx context.Context, key string) error

        // StatObject returns metadata about an object without downloading it.
        StatObject(ctx context.Context, key string) (ObjectInfo, error)

        // --- List ---

        // ListObjects lists objects in the bucket matching the prefix.
        ListObjects(ctx context.Context, opts ListOptions) ([]ObjectInfo, error)

        // --- Copy ---

        // CopyObject copies an object from srcKey to dstKey.
        CopyObject(ctx context.Context, srcKey, dstKey string, opts PutOptions) (ObjectInfo, error)

        // --- Presign ---

        // PresignPut generates a presigned URL for uploading an object directly
        // from the browser. The URL expires after expiry.
        PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error)

        // PresignGet generates a presigned URL for downloading an object directly
        // from the browser. The URL expires after expiry.
        PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)

        // --- Multipart upload ---

        // InitMultipartUpload starts a multipart upload session.
        // Returns an upload ID that must be passed to UploadPart and CompleteMultipartUpload.
        InitMultipartUpload(ctx context.Context, key string, opts PutOptions) (uploadID string, err error)

        // UploadPart uploads a single part of a multipart upload.
        // partNumber is 1-indexed. Returns the ETag of the part.
        UploadPart(ctx context.Context, key, uploadID string, partNumber int, body ReadSeeker, size int64) (etag string, err error)

        // CompleteMultipartUpload finalizes a multipart upload by assembling
        // all uploaded parts into the final object.
        CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) (ObjectInfo, error)

        // AbortMultipartUpload cancels a multipart upload and discards uploaded parts.
        AbortMultipartUpload(ctx context.Context, key, uploadID string) error

        // --- Lifecycle ---

        // Close closes the underlying client. Idempotent.
        Close() error

        // DriverName returns the registered driver name.
        DriverName() string
}

// ============================================================================
// Types
// ============================================================================

// ReadSeeker is io.ReadSeeker (aliased to avoid importing io in the interface).
type ReadSeeker interface {
        Read(p []byte) (n int, err error)
        Seek(offset int64, whence int) (int64, error)
}

// ReadCloser is io.ReadCloser.
type ReadCloser interface {
        Read(p []byte) (n int, err error)
        Close() error
}

// ObjectInfo holds metadata about a stored object.
type ObjectInfo struct {
        Bucket       string
        Key          string
        Size         int64
        ContentType  string
        ETag         string
        LastModified time.Time
        Metadata     map[string]string
}

// PutOptions controls object upload behavior.
type PutOptions struct {
        ContentType  string
        Metadata     map[string]string
        // CacheControl sets the Cache-Control header.
        CacheControl string
}

// GetOptions controls object download behavior.
type GetOptions struct {
        // Range requests a byte range (e.g., "bytes=0-1023"). Empty = full object.
        Range string
}

// ListOptions controls object listing behavior.
type ListOptions struct {
        // Prefix filters objects by key prefix.
        Prefix string
        // MaxKeys limits the number of results. 0 = use server default (1000).
        MaxKeys int
        // Recursive lists all objects (not just "folders").
        Recursive bool
}

// CompletedPart represents an uploaded part in a multipart upload.
type CompletedPart struct {
        PartNumber int
        ETag       string
}

// ============================================================================
// Driver registry
// ============================================================================

// DriverOpener opens a Client for the given Config.
type DriverOpener func(cfg Config) (Client, error)

var drivers = map[string]DriverOpener{}

// RegisterDriver registers a driver opener under the given name.
func RegisterDriver(name string, fn DriverOpener) {
        if _, exists := drivers[name]; exists {
                panic("objectstorex: driver " + name + " already registered")
        }
        drivers[name] = fn
}

// IsDriverRegistered returns true if the named driver has been registered.
func IsDriverRegistered(name string) bool {
        _, ok := drivers[name]
        return ok
}

// RegisteredDrivers returns the names of all registered drivers.
func RegisteredDrivers() []string {
        out := make([]string, 0, len(drivers))
        for name := range drivers {
                out = append(out, name)
        }
        return out
}

// ============================================================================
// Constructor
// ============================================================================

// New opens an object store connection using the supplied Config.
func New(cfg Config) (Client, error) {
        if err := cfg.Validate(); err != nil {
                return nil, err
        }
        open, ok := drivers[cfg.Driver]
        if !ok {
                return nil, ErrUnknownDriver
        }
        return open(cfg)
}
