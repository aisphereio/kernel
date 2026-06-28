// Package main demonstrates basic cachex usage with Redis.
//
// Run:
//
//	export KERNEL_CACHEX_REDIS_ADDR=localhost:6379
//	go run ./examples/cachex-basic
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aisphereio/kernel/cachex"
	_ "github.com/aisphereio/kernel/cachex/redis"
)

type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	addr := os.Getenv("KERNEL_CACHEX_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	cache, err := cachex.New(cachex.Config{
		Driver: "redis",
		Addrs:  []string{addr},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "new cache: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	ctx := context.Background()

	// Ping
	if err := cache.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("connected to Redis")

	// Set + Get
	user := &User{Name: "alice", Age: 30}
	_ = cache.Set(ctx, "user:1", user, 5*time.Minute)
	fmt.Printf("Set user:1 = %+v\n", user)

	var got User
	_ = cache.Get(ctx, "user:1", &got)
	fmt.Printf("Get user:1 = %+v\n", got)

	// GetOrSet (cache-through)
	var computed string
	_ = cache.GetOrSet(ctx, "compute:expensive", &computed, time.Minute, func(ctx context.Context) (any, error) {
		fmt.Println("  (cache miss — computing...)")
		return "computed-result", nil
	})
	fmt.Printf("GetOrSet result = %s\n", computed)

	// SetIfNotExist (distributed lock)
	ok, _ := cache.SetIfNotExist(ctx, "lock:job:42", "owner", 30*time.Second)
	fmt.Printf("SetIfNotExist lock:job:42 = %v\n", ok)

	// Incr (atomic counter)
	n, _ := cache.Incr(ctx, "rate_limit:user:1")
	fmt.Printf("Incr rate_limit = %d\n", n)

	// Hash
	_ = cache.HSet(ctx, "session:abc", "user_id", "user_123")
	_ = cache.HSet(ctx, "session:abc", "role", "admin")
	var role string
	_ = cache.HGet(ctx, "session:abc", "role", &role)
	fmt.Printf("HGet session:abc role = %s\n", role)

	// Pub/Sub
	sub, _ := cache.Subscribe(ctx, "notifications")
	defer sub.Close()
	time.Sleep(50 * time.Millisecond) // let subscription register
	cache.Publish(ctx, "notifications", "hello from publisher")
	select {
	case msg := <-sub.Channel:
		fmt.Printf("Received: channel=%s payload=%s\n", msg.Channel, msg.Payload)
	case <-time.After(time.Second):
		fmt.Println("timeout waiting for message")
	}

	fmt.Println("done")
}
