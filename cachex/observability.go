package cachex

import (
	"context"
	"errors"
	"time"

	"github.com/aisphereio/kernel/errorx"
	"github.com/aisphereio/kernel/logx"
)

const (
	CodeNotFound        = errorx.Code("CACHEX_NOT_FOUND")
	CodeInvalidConfig   = errorx.Code("CACHEX_INVALID_CONFIG")
	CodeUnknownDriver   = errorx.Code("CACHEX_UNKNOWN_DRIVER")
	CodeClosed          = errorx.Code("CACHEX_CLOSED")
	CodeTypeMismatch    = errorx.Code("CACHEX_TYPE_MISMATCH")
	CodeNilValue        = errorx.Code("CACHEX_NIL_VALUE")
	CodeOperationFailed = errorx.Code("CACHEX_OPERATION_FAILED")
	CodeTimeout         = errorx.Code("CACHEX_TIMEOUT")
	CodeLockNotAcquired = errorx.Code("CACHEX_LOCK_NOT_ACQUIRED")
	CodeLockNotHeld     = errorx.Code("CACHEX_LOCK_NOT_HELD")
)

const (
	metricCacheOperationsTotal  = "kernel_cachex_operations_total"
	metricCacheOperationSeconds = "kernel_cachex_operation_duration_seconds"
)

func cacheLogger(cfg Config) logx.Logger {
	logger := cfg.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	return logger.Named("cachex").With(logx.String("driver", cfg.Driver))
}

func registerCacheMetrics(cfg Config) {
	if !cfg.MetricsEnabled || cfg.Metrics == nil {
		return
	}
	cfg.Metrics.NewCounter(metricCacheOperationsTotal, "Total cachex operations")
	cfg.Metrics.NewHistogram(metricCacheOperationSeconds, "cachex operation latency in seconds", 0.0005, 0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1)
}

func NormalizeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errorx.As(err); ok {
		return err
	}
	code, status, message, retryable := classifyCacheError(err)
	return errorx.Wrap(err, code,
		errorx.WithMessage(message),
		errorx.WithHTTPStatus(status),
		errorx.WithRetryable(retryable),
		errorx.WithMetadata("component", "cachex"),
	)
}

func classifyCacheError(err error) (errorx.Code, int, string, bool) {
	switch {
	case errors.Is(err, ErrNotFound):
		return CodeNotFound, errorx.HTTPStatusNotFound, "cache key not found", false
	case errors.Is(err, ErrNilConfig):
		return CodeInvalidConfig, errorx.HTTPStatusBadRequest, "cache config is invalid", false
	case errors.Is(err, ErrUnknownDriver):
		return CodeUnknownDriver, errorx.HTTPStatusBadRequest, "cache driver is not registered", false
	case errors.Is(err, ErrClosed):
		return CodeClosed, errorx.HTTPStatusServiceUnavailable, "cache is closed", true
	case errors.Is(err, ErrTypeMismatch):
		return CodeTypeMismatch, errorx.HTTPStatusBadRequest, "cache value type mismatch", false
	case errors.Is(err, ErrNilValue):
		return CodeNilValue, errorx.HTTPStatusBadRequest, "cache value is nil", false
	case errors.Is(err, ErrLockNotAcquired):
		return CodeLockNotAcquired, errorx.HTTPStatusConflict, "cache lock not acquired", false
	case errors.Is(err, ErrLockNotHeld):
		return CodeLockNotHeld, errorx.HTTPStatusConflict, "cache lock not held", false
	case errors.Is(err, context.Canceled):
		return CodeTimeout, errorx.HTTPStatusClientClosedRequest, "cache operation canceled", false
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, errorx.HTTPStatusGatewayTimeout, "cache operation timed out", true
	default:
		return CodeOperationFailed, errorx.HTTPStatusInternalServerError, "cache operation failed", false
	}
}

func observeCacheInit(cfg Config, started time.Time, err error) error {
	elapsed := time.Since(started)
	logger := cacheLogger(cfg)
	if err == nil {
		logger.Info("cachex opened", logx.Duration("elapsed", elapsed), logx.Int("addr_count", len(cfg.Addrs)))
		return nil
	}
	nerr := NormalizeError(err)
	logger.Error("cachex open failed", logx.Duration("elapsed", elapsed), logx.Int("addr_count", len(cfg.Addrs)), logx.Err(nerr))
	return nerr
}

func observeCacheOperation(cfg Config, ctx context.Context, operation string, started time.Time, err error) error {
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
		cfg.Metrics.IncrementCounter(ctx, metricCacheOperationsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricCacheOperationSeconds, elapsed.Seconds(), labels...)
	}
	if nerr != nil && !errors.Is(nerr, ErrNotFound) {
		cacheLogger(cfg).Error("cachex operation failed", logx.String("operation", operation), logx.Duration("elapsed", elapsed), logx.Err(nerr))
	}
	return nerr
}

func observeCacheOperationRaw(cfg Config, ctx context.Context, operation string, started time.Time, err error) error {
	elapsed := time.Since(started)
	status := "ok"
	code := errorx.CodeOK.String()
	if err != nil {
		status = "error"
		code = errorx.CodeOf(err).String()
	}
	if cfg.MetricsEnabled && cfg.Metrics != nil {
		labels := []string{"driver", cfg.Driver, "operation", operation, "status", status, "code", code}
		cfg.Metrics.IncrementCounter(ctx, metricCacheOperationsTotal, labels...)
		cfg.Metrics.RecordHistogram(ctx, metricCacheOperationSeconds, elapsed.Seconds(), labels...)
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		cacheLogger(cfg).Error("cachex operation failed", logx.String("operation", operation), logx.Duration("elapsed", elapsed), logx.Err(err))
	}
	return err
}

type observedCache struct {
	next Cache
	cfg  Config
}

func observeCache(next Cache, cfg Config) Cache {
	if next == nil {
		return nil
	}
	return &observedCache{next: next, cfg: cfg}
}

func (c *observedCache) Get(ctx context.Context, key string, dest any) error {
	start := time.Now()
	err := c.next.Get(ctx, key, dest)
	return observeCacheOperation(c.cfg, ctx, "get", start, err)
}
func (c *observedCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	err := c.next.Set(ctx, key, value, ttl)
	return observeCacheOperation(c.cfg, ctx, "set", start, err)
}
func (c *observedCache) Del(ctx context.Context, keys ...string) error {
	start := time.Now()
	err := c.next.Del(ctx, keys...)
	return observeCacheOperation(c.cfg, ctx, "del", start, err)
}
func (c *observedCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()
	n, err := c.next.Exists(ctx, keys...)
	return n, observeCacheOperation(c.cfg, ctx, "exists", start, err)
}
func (c *observedCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	start := time.Now()
	err := c.next.Expire(ctx, key, ttl)
	return observeCacheOperation(c.cfg, ctx, "expire", start, err)
}
func (c *observedCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	start := time.Now()
	ttl, err := c.next.TTL(ctx, key)
	return ttl, observeCacheOperation(c.cfg, ctx, "ttl", start, err)
}
func (c *observedCache) MGet(ctx context.Context, keys []string, dest any) error {
	start := time.Now()
	err := c.next.MGet(ctx, keys, dest)
	return observeCacheOperation(c.cfg, ctx, "mget", start, err)
}
func (c *observedCache) MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error {
	start := time.Now()
	err := c.next.MSet(ctx, pairs, ttl)
	return observeCacheOperation(c.cfg, ctx, "mset", start, err)
}
func (c *observedCache) GetOrSet(ctx context.Context, key string, dest any, ttl time.Duration, fn func(ctx context.Context) (any, error)) error {
	start := time.Now()
	err := c.next.GetOrSet(ctx, key, dest, ttl, fn)
	return observeCacheOperationRaw(c.cfg, ctx, "get_or_set", start, err)
}
func (c *observedCache) SetIfNotExist(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	start := time.Now()
	ok, err := c.next.SetIfNotExist(ctx, key, value, ttl)
	return ok, observeCacheOperation(c.cfg, ctx, "set_if_not_exist", start, err)
}
func (c *observedCache) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	start := time.Now()
	token, err := c.next.Lock(ctx, key, ttl)
	return token, observeCacheOperation(c.cfg, ctx, "lock", start, err)
}
func (c *observedCache) Unlock(ctx context.Context, key, token string) error {
	start := time.Now()
	err := c.next.Unlock(ctx, key, token)
	return observeCacheOperation(c.cfg, ctx, "unlock", start, err)
}
func (c *observedCache) Incr(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	v, err := c.next.Incr(ctx, key)
	return v, observeCacheOperation(c.cfg, ctx, "incr", start, err)
}
func (c *observedCache) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	start := time.Now()
	v, err := c.next.IncrBy(ctx, key, delta)
	return v, observeCacheOperation(c.cfg, ctx, "incr_by", start, err)
}
func (c *observedCache) Decr(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	v, err := c.next.Decr(ctx, key)
	return v, observeCacheOperation(c.cfg, ctx, "decr", start, err)
}
func (c *observedCache) Publish(ctx context.Context, channel string, message any) (int64, error) {
	start := time.Now()
	n, err := c.next.Publish(ctx, channel, message)
	return n, observeCacheOperation(c.cfg, ctx, "publish", start, err)
}
func (c *observedCache) Subscribe(ctx context.Context, channels ...string) (*Subscription, error) {
	start := time.Now()
	sub, err := c.next.Subscribe(ctx, channels...)
	return sub, observeCacheOperation(c.cfg, ctx, "subscribe", start, err)
}
func (c *observedCache) HSet(ctx context.Context, key, field string, value any) error {
	start := time.Now()
	err := c.next.HSet(ctx, key, field, value)
	return observeCacheOperation(c.cfg, ctx, "hset", start, err)
}
func (c *observedCache) HGet(ctx context.Context, key, field string, dest any) error {
	start := time.Now()
	err := c.next.HGet(ctx, key, field, dest)
	return observeCacheOperation(c.cfg, ctx, "hget", start, err)
}
func (c *observedCache) HGetAll(ctx context.Context, key string, dest any) error {
	start := time.Now()
	err := c.next.HGetAll(ctx, key, dest)
	return observeCacheOperation(c.cfg, ctx, "hget_all", start, err)
}
func (c *observedCache) HDel(ctx context.Context, key string, fields ...string) error {
	start := time.Now()
	err := c.next.HDel(ctx, key, fields...)
	return observeCacheOperation(c.cfg, ctx, "hdel", start, err)
}
func (c *observedCache) LPush(ctx context.Context, key string, values ...any) error {
	start := time.Now()
	err := c.next.LPush(ctx, key, values...)
	return observeCacheOperation(c.cfg, ctx, "lpush", start, err)
}
func (c *observedCache) RPush(ctx context.Context, key string, values ...any) error {
	start := time.Now()
	err := c.next.RPush(ctx, key, values...)
	return observeCacheOperation(c.cfg, ctx, "rpush", start, err)
}
func (c *observedCache) LPop(ctx context.Context, key string, dest any) error {
	start := time.Now()
	err := c.next.LPop(ctx, key, dest)
	return observeCacheOperation(c.cfg, ctx, "lpop", start, err)
}
func (c *observedCache) RPop(ctx context.Context, key string, dest any) error {
	start := time.Now()
	err := c.next.RPop(ctx, key, dest)
	return observeCacheOperation(c.cfg, ctx, "rpop", start, err)
}
func (c *observedCache) LRange(ctx context.Context, key string, startIdx, stop int64, dest any) error {
	start := time.Now()
	err := c.next.LRange(ctx, key, startIdx, stop, dest)
	return observeCacheOperation(c.cfg, ctx, "lrange", start, err)
}
func (c *observedCache) LLen(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	n, err := c.next.LLen(ctx, key)
	return n, observeCacheOperation(c.cfg, ctx, "llen", start, err)
}
func (c *observedCache) SAdd(ctx context.Context, key string, members ...any) error {
	start := time.Now()
	err := c.next.SAdd(ctx, key, members...)
	return observeCacheOperation(c.cfg, ctx, "sadd", start, err)
}
func (c *observedCache) SRem(ctx context.Context, key string, members ...any) error {
	start := time.Now()
	err := c.next.SRem(ctx, key, members...)
	return observeCacheOperation(c.cfg, ctx, "srem", start, err)
}
func (c *observedCache) SMembers(ctx context.Context, key string, dest any) error {
	start := time.Now()
	err := c.next.SMembers(ctx, key, dest)
	return observeCacheOperation(c.cfg, ctx, "smembers", start, err)
}
func (c *observedCache) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	start := time.Now()
	ok, err := c.next.SIsMember(ctx, key, member)
	return ok, observeCacheOperation(c.cfg, ctx, "sismember", start, err)
}
func (c *observedCache) Ping(ctx context.Context) error {
	start := time.Now()
	err := c.next.Ping(ctx)
	return observeCacheOperation(c.cfg, ctx, "ping", start, err)
}
func (c *observedCache) Close() error {
	start := time.Now()
	err := c.next.Close()
	return observeCacheOperation(c.cfg, context.Background(), "close", start, err)
}
func (c *observedCache) DriverName() string { return c.next.DriverName() }
