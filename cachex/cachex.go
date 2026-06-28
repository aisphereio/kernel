package cachex

import (
        "context"
        "time"
)

// Config holds the configuration for a cache connection.
type Config struct {
        // Driver selects the registered driver: "redis".
        Driver string `json:"driver"`

        // Addrs is the list of Redis node addresses.
        // Single-node: ["localhost:6379"]
        // Cluster:     ["node1:6379", "node2:6379", "node3:6379"]
        Addrs []string `json:"addrs"`

        // Username for AUTH (Redis 6+ ACL).
        Username string `json:"username"`

        // Password for AUTH.
        Password string `json:"password"`

        // DB selects the Redis database (0-15 for single-node; ignored in cluster).
        DB int `json:"db"`

        // MasterName enables Sentinel mode (non-empty = sentinel).
        MasterName string `json:"master_name"`

        // Cluster enables cluster mode. If true, Addrs are seed nodes.
        Cluster bool `json:"cluster"`

        // PoolSize is the max number of socket connections.
        // Default is 10 * runtime.GOMAXPROCS.
        PoolSize int `json:"pool_size"`

        // MinIdleConns is the minimum number of idle connections.
        MinIdleConns int `json:"min_idle_conns"`

        // DialTimeout is the timeout for establishing connections.
        DialTimeout time.Duration `json:"dial_timeout_ns"`

        // ReadTimeout is the timeout for socket reads.
        ReadTimeout time.Duration `json:"read_timeout_ns"`

        // WriteTimeout is the timeout for socket writes.
        WriteTimeout time.Duration `json:"write_timeout_ns"`

        // KeyPrefix is prepended to all keys (namespace isolation).
        KeyPrefix string `json:"key_prefix"`

        // TLSEnabled enables TLS.
        TLSEnabled bool `json:"tls_enabled"`

        // TLSSkipVerify skips TLS certificate verification.
        TLSSkipVerify bool `json:"tls_skip_verify"`
}

// Validate returns ErrNilConfig if required fields are missing.
func (c Config) Validate() error {
        if c.Driver == "" || len(c.Addrs) == 0 {
                return ErrNilConfig
        }
        return nil
}

// ============================================================================
// Cache interface
// ============================================================================

// Cache is the runtime cache interface used by kernel modules and apps.
//
// All methods accept context.Context. Methods that take a dest pointer
// (Get / GetOrSet) auto-serialize/deserialize via JSON.
type Cache interface {
        // --- Basic CRUD ---

        // Get retrieves the value for key and deserializes into dest.
        // Returns ErrNotFound if the key does not exist.
        Get(ctx context.Context, key string, dest any) error

        // Set stores value at key with the given TTL.
        // TTL == 0 means no expiration.
        Set(ctx context.Context, key string, value any, ttl time.Duration) error

        // Del removes one or more keys.
        Del(ctx context.Context, keys ...string) error

        // Exists checks if any of the keys exist. Returns count.
        Exists(ctx context.Context, keys ...string) (int64, error)

        // Expire sets a TTL on an existing key. No-op if key doesn't exist.
        Expire(ctx context.Context, key string, ttl time.Duration) error

        // TTL returns the remaining TTL of a key.
        // Returns -1 if key has no expiration.
        // Returns -2 if key does not exist.
        TTL(ctx context.Context, key string) (time.Duration, error)

        // --- Batch operations ---

        // MGet retrieves multiple keys. dest must be *[]T or *[]any.
        // Missing keys appear as nil in the result.
        MGet(ctx context.Context, keys []string, dest any) error

        // MSet sets multiple key-value pairs with the same TTL.
        MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error

        // --- GetOrSet (cache-through) ---

        // GetOrSet retrieves the value for key. If the key is missing, it calls
        // fn to compute the value, caches it with the given TTL, and returns it.
        // This is the standard cache penetration prevention pattern.
        GetOrSet(ctx context.Context, key string, dest any, ttl time.Duration, fn func(ctx context.Context) (any, error)) error

        // --- SetIfNotExist (distributed lock primitive) ---

        // SetIfNotExist sets key=value with TTL only if key does not exist.
        // Returns true if the key was set (i.e., acquired).
        SetIfNotExist(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)

        // --- Atomic counter ---

        // Incr atomically increments key by 1. Creates the key with value 1 if
        // it doesn't exist. Returns the new value.
        Incr(ctx context.Context, key string) (int64, error)

        // IncrBy atomically increments key by delta. Returns the new value.
        IncrBy(ctx context.Context, key string, delta int64) (int64, error)

        // Decr atomically decrements key by 1. Returns the new value.
        Decr(ctx context.Context, key string) (int64, error)

        // --- Pub/Sub ---

        // Publish sends a message to a channel. Returns the number of subscribers.
        Publish(ctx context.Context, channel string, message any) (int64, error)

        // Subscribe subscribes to channels and returns a Subscription.
        // Call Subscription.Close() to stop receiving.
        Subscribe(ctx context.Context, channels ...string) (*Subscription, error)

        // --- Hash operations ---

        // HSet sets a field in a hash.
        HSet(ctx context.Context, key, field string, value any) error

        // HGet gets a field from a hash and deserializes into dest.
        // Returns ErrNotFound if the field does not exist.
        HGet(ctx context.Context, key, field string, dest any) error

        // HGetAll returns all fields of a hash as a map.
        // dest must be *map[string]any or *map[string]T.
        HGetAll(ctx context.Context, key string, dest any) error

        // HDel removes fields from a hash.
        HDel(ctx context.Context, key string, fields ...string) error

        // --- List operations ---

        // LPush prepends values to a list.
        LPush(ctx context.Context, key string, values ...any) error

        // RPush appends values to a list.
        RPush(ctx context.Context, key string, values ...any) error

        // LPop removes and returns the first element.
        LPop(ctx context.Context, key string, dest any) error

        // RPop removes and returns the last element.
        RPop(ctx context.Context, key string, dest any) error

        // LRange returns a range of elements from a list.
        // dest must be *[]T or *[]any.
        LRange(ctx context.Context, key string, start, stop int64, dest any) error

        // LLen returns the length of a list.
        LLen(ctx context.Context, key string) (int64, error)

        // --- Set operations ---

        // SAdd adds members to a set.
        SAdd(ctx context.Context, key string, members ...any) error

        // SRem removes members from a set.
        SRem(ctx context.Context, key string, members ...any) error

        // SMembers returns all members of a set.
        // dest must be *[]T or *[]any.
        SMembers(ctx context.Context, key string, dest any) error

        // SIsMember checks if a value is in a set.
        SIsMember(ctx context.Context, key string, member any) (bool, error)

        // --- Lifecycle ---

        // Ping verifies the cache is reachable.
        Ping(ctx context.Context) error

        // Close closes the cache connection. Idempotent.
        Close() error

        // DriverName returns the registered driver name.
        DriverName() string
}

// ============================================================================
// Subscription (Pub/Sub)
// ============================================================================

// Subscription represents an active Pub/Sub subscription.
type Subscription struct {
        // Channel receives messages from all subscribed channels.
        Channel <-chan *Message

        // internal fields (set by driver implementation)
        UnsubscribeFn func(channels ...string) error
        CloseFn       func() error
}

// Message is a single Pub/Sub message.
type Message struct {
        Channel string
        Pattern string // non-empty if subscribed via pattern
        Payload string
}

// Unsubscribe stops receiving from the given channels.
func (s *Subscription) Unsubscribe(channels ...string) error {
        if s.UnsubscribeFn == nil {
                return nil
        }
        return s.UnsubscribeFn(channels...)
}

// Close closes the subscription entirely.
func (s *Subscription) Close() error {
        if s.CloseFn == nil {
                return nil
        }
        return s.CloseFn()
}

// ============================================================================
// Driver registry
// ============================================================================

// DriverOpener opens a Cache for the given Config.
type DriverOpener func(cfg Config) (Cache, error)

var drivers = map[string]DriverOpener{}

// RegisterDriver registers a driver opener under the given name.
// Called by cachex/redis in its init() function.
func RegisterDriver(name string, fn DriverOpener) {
        if _, exists := drivers[name]; exists {
                panic("cachex: driver " + name + " already registered")
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

// New opens a cache connection using the supplied Config.
func New(cfg Config) (Cache, error) {
        if err := cfg.Validate(); err != nil {
                return nil, err
        }
        open, ok := drivers[cfg.Driver]
        if !ok {
                return nil, ErrUnknownDriver
        }
        return open(cfg)
}
