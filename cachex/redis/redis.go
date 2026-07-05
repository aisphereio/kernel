// Package redis registers the "redis" driver for cachex.
//
// Import this package once in your main function:
//
//	import _ "github.com/aisphereio/kernel/cachex/redis"
//
// The driver uses github.com/redis/go-redis/v9 and supports:
//   - Single-node mode (Config.Cluster = false)
//   - Cluster mode (Config.Cluster = true, Addrs = seed nodes)
//   - Sentinel mode (Config.MasterName != "", Addrs = sentinel addresses)
//   - TLS (Config.TLSEnabled = true)
package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/logx"

	"github.com/redis/go-redis/v9"
)

const driverName = "redis"

func init() {
	cachex.RegisterDriver(driverName, open)
}

// open creates a Cache backed by go-redis/v9.
func open(cfg cachex.Config) (cachex.Cache, error) {
	if cfg.Cluster {
		return newClusterCache(cfg)
	}
	if cfg.MasterName != "" {
		return newSentinelCache(cfg)
	}
	return newSingleCache(cfg)
}

// ============================================================================
// Single-node implementation
// ============================================================================

func newSingleCache(cfg cachex.Config) (cachex.Cache, error) {
	opts := &redis.Options{
		Addr:         firstAddr(cfg.Addrs),
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  orDefault(cfg.DialTimeout, 5*time.Second),
		ReadTimeout:  orDefault(cfg.ReadTimeout, 3*time.Second),
		WriteTimeout: orDefault(cfg.WriteTimeout, 3*time.Second),
		TLSConfig:    buildTLS(cfg),
	}
	client := redis.NewClient(opts)
	return &redisCache{client: client, cfg: cfg, universal: client}, nil
}

// ============================================================================
// Cluster implementation
// ============================================================================

func newClusterCache(cfg cachex.Config) (cachex.Cache, error) {
	opts := &redis.ClusterOptions{
		Addrs:        cfg.Addrs,
		Username:     cfg.Username,
		Password:     cfg.Password,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  orDefault(cfg.DialTimeout, 5*time.Second),
		ReadTimeout:  orDefault(cfg.ReadTimeout, 3*time.Second),
		WriteTimeout: orDefault(cfg.WriteTimeout, 3*time.Second),
		TLSConfig:    buildTLS(cfg),
	}
	client := redis.NewClusterClient(opts)
	return &redisCache{clusterClient: client, cfg: cfg, universal: client}, nil
}

// ============================================================================
// Sentinel implementation
// ============================================================================

func newSentinelCache(cfg cachex.Config) (cachex.Cache, error) {
	opts := &redis.FailoverOptions{
		MasterName:    cfg.MasterName,
		SentinelAddrs: cfg.Addrs,
		Username:      cfg.Username,
		Password:      cfg.Password,
		DB:            cfg.DB,
		PoolSize:      cfg.PoolSize,
		MinIdleConns:  cfg.MinIdleConns,
		DialTimeout:   orDefault(cfg.DialTimeout, 5*time.Second),
		ReadTimeout:   orDefault(cfg.ReadTimeout, 3*time.Second),
		WriteTimeout:  orDefault(cfg.WriteTimeout, 3*time.Second),
		TLSConfig:     buildTLS(cfg),
	}
	client := redis.NewFailoverClient(opts)
	return &redisCache{client: client, cfg: cfg, universal: client}, nil
}

// ============================================================================
// redisCache implements cachex.Cache
// ============================================================================

type redisCache struct {
	client        *redis.Client
	clusterClient *redis.ClusterClient
	cfg           cachex.Config
	universal     redis.UniversalClient
	closed        bool
	mu            sync.Mutex
}

func (c *redisCache) prefixKey(key string) string {
	if c.cfg.KeyPrefix == "" {
		return key
	}
	return c.cfg.KeyPrefix + ":" + key
}

func (c *redisCache) prefixKeys(keys []string) []string {
	if c.cfg.KeyPrefix == "" {
		return keys
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = c.cfg.KeyPrefix + ":" + k
	}
	return out
}

// --- Basic CRUD ---

func (c *redisCache) Get(ctx context.Context, key string, dest any) error {
	val, err := c.universal.Get(ctx, c.prefixKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return cachex.ErrNotFound
		}
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cachex: marshal value: %w", err)
	}
	return c.universal.Set(ctx, c.prefixKey(key), data, ttl).Err()
}

func (c *redisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.universal.Del(ctx, c.prefixKeys(keys)...).Err()
}

func (c *redisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	n, err := c.universal.Exists(ctx, c.prefixKeys(keys)...).Result()
	return n, err
}

func (c *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.universal.Expire(ctx, c.prefixKey(key), ttl).Err()
}

func (c *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.universal.TTL(ctx, c.prefixKey(key)).Result()
}

// --- Batch ---

func (c *redisCache) MGet(ctx context.Context, keys []string, dest any) error {
	if len(keys) == 0 {
		return nil
	}
	results, err := c.universal.MGet(ctx, c.prefixKeys(keys)...).Result()
	if err != nil {
		return err
	}
	// Serialize results as JSON array and unmarshal into dest.
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *redisCache) MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error {
	if len(pairs) == 0 {
		return nil
	}
	// go-redis MSet doesn't support per-key TTL, so we use pipeline.
	pipe := c.universal.TxPipeline()
	for k, v := range pairs {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("cachex: marshal value for key %s: %w", k, err)
		}
		pipe.Set(ctx, c.prefixKey(k), data, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// --- GetOrSet ---

func (c *redisCache) GetOrSet(ctx context.Context, key string, dest any, ttl time.Duration, fn func(ctx context.Context) (any, error)) error {
	// Try Get first.
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil // cache hit
	}
	if !errors.Is(err, cachex.ErrNotFound) {
		return err // real error
	}

	// Cache miss — call fn.
	val, err := fn(ctx)
	if err != nil {
		return err
	}
	if val == nil {
		return cachex.ErrNilValue
	}

	// Cache the result (best-effort; don't fail if caching fails).
	if err := c.Set(ctx, key, val, ttl); err != nil {
		logx.Warn("cachex: GetOrSet cache write failed", "key", key, "error", err)
	}

	// Marshal/unmarshal into dest so the caller gets the same type as cache hit.
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// --- SetIfNotExist ---

func (c *redisCache) SetIfNotExist(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("cachex: marshal value: %w", err)
	}
	ok, err := c.universal.SetNX(ctx, c.prefixKey(key), data, ttl).Result()
	return ok, err
}

// --- Atomic counter ---

func (c *redisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.universal.Incr(ctx, c.prefixKey(key)).Result()
}

func (c *redisCache) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return c.universal.IncrBy(ctx, c.prefixKey(key), delta).Result()
}

func (c *redisCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.universal.Decr(ctx, c.prefixKey(key)).Result()
}

// --- Pub/Sub ---

func (c *redisCache) Publish(ctx context.Context, channel string, message any) (int64, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return 0, fmt.Errorf("cachex: marshal message: %w", err)
	}
	return c.universal.Publish(ctx, channel, data).Result()
}

func (c *redisCache) Subscribe(ctx context.Context, channels ...string) (*cachex.Subscription, error) {
	pubsub := c.universal.Subscribe(ctx, channels...)
	msgCh := pubsub.Channel()

	// Bridge go-redis channel to cachex.Message channel.
	out := make(chan *cachex.Message, 16)
	sub := &cachex.Subscription{
		Channel: out,
		UnsubscribeFn: func(chs ...string) error {
			return pubsub.Unsubscribe(ctx, chs...)
		},
		CloseFn: func() error {
			return pubsub.Close()
		},
	}

	go func() {
		defer close(out)
		for msg := range msgCh {
			out <- &cachex.Message{
				Channel: msg.Channel,
				Pattern: msg.Pattern,
				Payload: msg.Payload,
			}
		}
	}()

	return sub, nil
}

// --- Hash ---

func (c *redisCache) HSet(ctx context.Context, key, field string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cachex: marshal value: %w", err)
	}
	return c.universal.HSet(ctx, c.prefixKey(key), field, data).Err()
}

func (c *redisCache) HGet(ctx context.Context, key, field string, dest any) error {
	val, err := c.universal.HGet(ctx, c.prefixKey(key), field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return cachex.ErrNotFound
		}
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *redisCache) HGetAll(ctx context.Context, key string, dest any) error {
	result, err := c.universal.HGetAll(ctx, c.prefixKey(key)).Result()
	if err != nil {
		return err
	}
	// Convert map[string]string to map[string]json.RawMessage for proper
	// deserialization into map[string]T.
	m := make(map[string]json.RawMessage, len(result))
	for k, v := range result {
		m[k] = json.RawMessage(v)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *redisCache) HDel(ctx context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return c.universal.HDel(ctx, c.prefixKey(key), fields...).Err()
}

// --- List ---

func (c *redisCache) LPush(ctx context.Context, key string, values ...any) error {
	data := make([]any, len(values))
	for i, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		data[i] = b
	}
	return c.universal.LPush(ctx, c.prefixKey(key), data...).Err()
}

func (c *redisCache) RPush(ctx context.Context, key string, values ...any) error {
	data := make([]any, len(values))
	for i, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		data[i] = b
	}
	return c.universal.RPush(ctx, c.prefixKey(key), data...).Err()
}

func (c *redisCache) LPop(ctx context.Context, key string, dest any) error {
	val, err := c.universal.LPop(ctx, c.prefixKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return cachex.ErrNotFound
		}
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *redisCache) RPop(ctx context.Context, key string, dest any) error {
	val, err := c.universal.RPop(ctx, c.prefixKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return cachex.ErrNotFound
		}
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *redisCache) LRange(ctx context.Context, key string, start, stop int64, dest any) error {
	results, err := c.universal.LRange(ctx, c.prefixKey(key), start, stop).Result()
	if err != nil {
		return err
	}
	// results is []string of JSON-encoded values.
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *redisCache) LLen(ctx context.Context, key string) (int64, error) {
	return c.universal.LLen(ctx, c.prefixKey(key)).Result()
}

// --- Set ---

func (c *redisCache) SAdd(ctx context.Context, key string, members ...any) error {
	data := make([]any, len(members))
	for i, m := range members {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		data[i] = b
	}
	return c.universal.SAdd(ctx, c.prefixKey(key), data...).Err()
}

func (c *redisCache) SRem(ctx context.Context, key string, members ...any) error {
	data := make([]any, len(members))
	for i, m := range members {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		data[i] = b
	}
	return c.universal.SRem(ctx, c.prefixKey(key), data...).Err()
}

func (c *redisCache) SMembers(ctx context.Context, key string, dest any) error {
	results, err := c.universal.SMembers(ctx, c.prefixKey(key)).Result()
	if err != nil {
		return err
	}
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *redisCache) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	b, err := json.Marshal(member)
	if err != nil {
		return false, err
	}
	return c.universal.SIsMember(ctx, c.prefixKey(key), b).Result()
}

// --- Lifecycle ---

func (c *redisCache) Ping(ctx context.Context) error {
	return c.universal.Ping(ctx).Err()
}

func (c *redisCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.universal.Close()
}

func (c *redisCache) DriverName() string { return driverName }

// ============================================================================
// Helpers
// ============================================================================

func firstAddr(addrs []string) string {
	if len(addrs) == 0 {
		return "localhost:6379"
	}
	return addrs[0]
}

func orDefault(d, def time.Duration) time.Duration {
	if d == 0 {
		return def
	}
	return d
}

func buildTLS(cfg cachex.Config) *tls.Config {
	if !cfg.TLSEnabled {
		return nil
	}
	return &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify,
	}
}
