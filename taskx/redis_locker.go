package taskx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisLeasePrefix = "kernel:taskx:lease:"

const renewLeaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`

const releaseLeaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`

// RedisLocker implements distributed leases with SET NX and ownership-checked
// Lua scripts. The scripts use one key and are safe for Redis Cluster slots.
type RedisLocker struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisLocker creates a Redis-backed Locker. Empty prefix uses the Kernel
// default namespace.
func NewRedisLocker(client redis.UniversalClient, prefix string) *RedisLocker {
	if prefix == "" {
		prefix = defaultRedisLeasePrefix
	}
	return &RedisLocker{client: client, prefix: prefix}
}

func (l *RedisLocker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lease, bool, error) {
	if l == nil || l.client == nil {
		return nil, false, errors.New("taskx: redis client is required")
	}
	if strings.TrimSpace(key) == "" {
		return nil, false, errors.New("taskx: lease key is required")
	}
	if ttl <= 0 {
		return nil, false, errors.New("taskx: lease ttl must be greater than zero")
	}
	redisKey := l.prefix + key
	token := newRunID()
	acquired, err := l.client.SetNX(ctx, redisKey, token, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("taskx: acquire redis lease %q: %w", redisKey, err)
	}
	if !acquired {
		return nil, false, nil
	}
	return &redisLease{client: l.client, key: redisKey, token: token}, true, nil
}

type redisLease struct {
	client redis.UniversalClient
	key    string
	token  string
}

func (l *redisLease) Renew(ctx context.Context, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("taskx: lease ttl must be greater than zero")
	}
	result, err := l.client.Eval(ctx, renewLeaseScript, []string{l.key}, l.token, ttl.Milliseconds()).Int64()
	if err != nil {
		return fmt.Errorf("taskx: renew redis lease %q: %w", l.key, err)
	}
	if result != 1 {
		return ErrLeaseLost
	}
	return nil
}

func (l *redisLease) Release(ctx context.Context) error {
	_, err := l.client.Eval(ctx, releaseLeaseScript, []string{l.key}, l.token).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("taskx: release redis lease %q: %w", l.key, err)
	}
	return nil
}
