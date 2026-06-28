package objectstorex

import (
	"context"
	"errors"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

const (
	CodeNotFound        = errorx.Code("OBJECTSTOREX_NOT_FOUND")
	CodeInvalidConfig   = errorx.Code("OBJECTSTOREX_INVALID_CONFIG")
	CodeUnknownDriver   = errorx.Code("OBJECTSTOREX_UNKNOWN_DRIVER")
	CodeBucketNotExists = errorx.Code("OBJECTSTOREX_BUCKET_NOT_EXISTS")
	CodeClosed          = errorx.Code("OBJECTSTOREX_CLOSED")
	CodeOperationFailed = errorx.Code("OBJECTSTOREX_OPERATION_FAILED")
	CodeTimeout         = errorx.Code("OBJECTSTOREX_TIMEOUT")
)

const (
	metricObjectStoreOperationsTotal  = "kernel_objectstorex_operations_total"
	metricObjectStoreOperationSeconds = "kernel_objectstorex_operation_duration_seconds"
)

func objectStoreLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("objectstorex").With(logx.String("driver", cfg.Driver), logx.String("bucket", cfg.Bucket))
}

func registerObjectStoreMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.NewCounter(metricObjectStoreOperationsTotal, "Total objectstorex operations")
	cfg.Metrics.NewHistogram(metricObjectStoreOperationSeconds, "objectstorex operation latency in seconds", 0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5)
}

func NormalizeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errorx.As(err); ok {
		return err
	}
	code, status, message, retryable := classifyObjectStoreError(err)
	return errorx.Wrap(err, code,
		errorx.WithMessage(message),
		errorx.WithHTTPStatus(status),
		errorx.WithRetryable(retryable),
		errorx.WithMetadata("component", "objectstorex"),
	)
}

func classifyObjectStoreError(err error) (errorx.Code, int, string, bool) {
	switch {
	case errors.Is(err, ErrNotFound):
		return CodeNotFound, errorx.HTTPStatusNotFound, "object not found", false
	case errors.Is(err, ErrNilConfig):
		return CodeInvalidConfig, errorx.HTTPStatusBadRequest, "object store config is invalid", false
	case errors.Is(err, ErrUnknownDriver):
		return CodeUnknownDriver, errorx.HTTPStatusBadRequest, "object store driver is not registered", false
	case errors.Is(err, ErrBucketNotExists):
		return CodeBucketNotExists, errorx.HTTPStatusServiceUnavailable, "object store bucket does not exist", true
	case errors.Is(err, ErrClosed):
		return CodeClosed, errorx.HTTPStatusServiceUnavailable, "object store is closed", true
	case errors.Is(err, context.Canceled):
		return CodeTimeout, errorx.HTTPStatusClientClosedRequest, "object store operation canceled", false
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, errorx.HTTPStatusGatewayTimeout, "object store operation timed out", true
	default:
		return CodeOperationFailed, errorx.HTTPStatusInternalServerError, "object store operation failed", false
	}
}

func observeObjectStoreInit(cfg Config, started time.Time, err error) error {
	elapsed := time.Since(started)
	logger := objectStoreLogger(cfg)
	if err == nil {
		logger.Info("objectstorex opened", logx.Duration("elapsed", elapsed), logx.Bool("ensure_bucket", cfg.EnsureBucket))
		return nil
	}
	nerr := NormalizeError(err)
	logger.Error("objectstorex open failed", logx.Duration("elapsed", elapsed), logx.Err(nerr))
	return nerr
}

func observeObjectStoreOperation(cfg Config, ctx context.Context, operation string, started time.Time, key string, err error) error {
	elapsed := time.Since(started)
	nerr := NormalizeError(err)
	status := "ok"
	code := errorx.CodeOK.String()
	if nerr != nil {
		status = "error"
		code = errorx.CodeOf(nerr).String()
	}
	if cfg.MetricsEnabled && cfg.Metrics != nil {
		labels := []string{"driver", cfg.Driver, "operation", operation, "status", status, "code", code}
		cfg.Metrics.IncrementCounter(ctx, metricObjectStoreOperationsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricObjectStoreOperationSeconds, elapsed.Seconds(), labels...)
	}
	if nerr != nil && !errors.Is(nerr, ErrNotFound) {
		objectStoreLogger(cfg).Error("objectstorex operation failed", logx.String("operation", operation), logx.String("object_key", key), logx.Duration("elapsed", elapsed), logx.Err(nerr))
	}
	return nerr
}

type observedClient struct {
	next Client
	cfg  Config
}

func observeObjectStore(next Client, cfg Config) Client {
	if next == nil {
		return nil
	}
	return &observedClient{next: next, cfg: cfg}
}

func (c *observedClient) Bucket() string { return c.next.Bucket() }

func (c *observedClient) BucketExists(ctx context.Context) (bool, error) {
	start := time.Now()
	ok, err := c.next.BucketExists(ctx)
	return ok, observeObjectStoreOperation(c.cfg, ctx, "bucket_exists", start, "", err)
}
func (c *observedClient) EnsureBucket(ctx context.Context) error {
	start := time.Now()
	err := c.next.EnsureBucket(ctx)
	return observeObjectStoreOperation(c.cfg, ctx, "ensure_bucket", start, "", err)
}
func (c *observedClient) PutObject(ctx context.Context, key string, body ReadSeeker, size int64, opts PutOptions) (ObjectInfo, error) {
	start := time.Now()
	info, err := c.next.PutObject(ctx, key, body, size, opts)
	return info, observeObjectStoreOperation(c.cfg, ctx, "put_object", start, key, err)
}
func (c *observedClient) GetObject(ctx context.Context, key string, opts GetOptions) (ReadCloser, ObjectInfo, error) {
	start := time.Now()
	rc, info, err := c.next.GetObject(ctx, key, opts)
	return rc, info, observeObjectStoreOperation(c.cfg, ctx, "get_object", start, key, err)
}
func (c *observedClient) DeleteObject(ctx context.Context, key string) error {
	start := time.Now()
	err := c.next.DeleteObject(ctx, key)
	return observeObjectStoreOperation(c.cfg, ctx, "delete_object", start, key, err)
}
func (c *observedClient) StatObject(ctx context.Context, key string) (ObjectInfo, error) {
	start := time.Now()
	info, err := c.next.StatObject(ctx, key)
	return info, observeObjectStoreOperation(c.cfg, ctx, "stat_object", start, key, err)
}
func (c *observedClient) ListObjects(ctx context.Context, opts ListOptions) ([]ObjectInfo, error) {
	start := time.Now()
	out, err := c.next.ListObjects(ctx, opts)
	return out, observeObjectStoreOperation(c.cfg, ctx, "list_objects", start, opts.Prefix, err)
}
func (c *observedClient) CopyObject(ctx context.Context, srcKey, dstKey string, opts PutOptions) (ObjectInfo, error) {
	start := time.Now()
	info, err := c.next.CopyObject(ctx, srcKey, dstKey, opts)
	return info, observeObjectStoreOperation(c.cfg, ctx, "copy_object", start, dstKey, err)
}
func (c *observedClient) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	start := time.Now()
	url, err := c.next.PresignPut(ctx, key, expiry)
	return url, observeObjectStoreOperation(c.cfg, ctx, "presign_put", start, key, err)
}
func (c *observedClient) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	start := time.Now()
	url, err := c.next.PresignGet(ctx, key, expiry)
	return url, observeObjectStoreOperation(c.cfg, ctx, "presign_get", start, key, err)
}
func (c *observedClient) InitMultipartUpload(ctx context.Context, key string, opts PutOptions) (string, error) {
	start := time.Now()
	uploadID, err := c.next.InitMultipartUpload(ctx, key, opts)
	return uploadID, observeObjectStoreOperation(c.cfg, ctx, "init_multipart_upload", start, key, err)
}
func (c *observedClient) UploadPart(ctx context.Context, key, uploadID string, partNumber int, body ReadSeeker, size int64) (string, error) {
	start := time.Now()
	etag, err := c.next.UploadPart(ctx, key, uploadID, partNumber, body, size)
	return etag, observeObjectStoreOperation(c.cfg, ctx, "upload_part", start, key, err)
}
func (c *observedClient) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) (ObjectInfo, error) {
	start := time.Now()
	info, err := c.next.CompleteMultipartUpload(ctx, key, uploadID, parts)
	return info, observeObjectStoreOperation(c.cfg, ctx, "complete_multipart_upload", start, key, err)
}
func (c *observedClient) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	start := time.Now()
	err := c.next.AbortMultipartUpload(ctx, key, uploadID)
	return observeObjectStoreOperation(c.cfg, ctx, "abort_multipart_upload", start, key, err)
}
func (c *observedClient) Close() error {
	start := time.Now()
	err := c.next.Close()
	return observeObjectStoreOperation(c.cfg, context.Background(), "close", start, "", err)
}
func (c *observedClient) DriverName() string { return c.next.DriverName() }
