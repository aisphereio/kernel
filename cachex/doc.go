// Package cachex provides the unified cache API for Aisphere Kernel.
//
// cachex is the ONLY cache abstraction that business code should depend on.
// It exposes a stable Cache interface backed by Redis (single-node or
// cluster), with JSON serialization, context propagation, and error
// normalization built in.
//
// # Quickstart
//
//	import (
//	    "github.com/aisphereio/kernel/cachex"
//	    _ "github.com/aisphereio/kernel/cachex/redis" // register "redis" driver
//	)
//
//	cache, err := cachex.New(cachex.Config{
//	    Driver: "redis",
//	    Addrs:  []string{"localhost:6379"},
//	})
//	if err != nil { return err }
//	defer cache.Close()
//
//	// Set with TTL
//	err = cache.Set(ctx, "user:123", user, 5*time.Minute)
//
//	// Get (auto-deserializes JSON into dest)
//	var user User
//	err = cache.Get(ctx, "user:123", &user)
//
//	// GetOrSet: cache-through pattern (prevents cache penetration)
//	err = cache.GetOrSet(ctx, "user:123", &user, 5*time.Minute, func(ctx context.Context) (*User, error) {
//	    return db.FindUser(ctx, 123)
//	})
//
//	// SetIfNotExist: distributed lock primitive
//	ok, err := cache.SetIfNotExist(ctx, "lock:job:42", "owner", 30*time.Second)
//
//	// Incr: atomic counter
//	count, err := cache.Incr(ctx, "rate_limit:user:123")
//
// # Drivers
//
//	import _ "github.com/aisphereio/kernel/cachex/redis" // registers "redis"
//
// The redis driver uses github.com/redis/go-redis/v9 and supports both
// single-node and cluster modes (set Config.Cluster = true).
//
// # Forbidden patterns
//
// Do not import `github.com/redis/go-redis/v9` in business code. Use the
// cachex.Cache interface. Do not use `encoding/json` directly for cache
// serialization — cachex handles it internally.
package cachex
