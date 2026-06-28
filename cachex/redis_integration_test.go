//go:build integration
// +build integration

package cachex_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/cachex/redis"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func redisDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("KERNEL_CACHEX_REDIS_ADDR"); dsn != "" {
		return dsn
	}
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "6379")
	return host + ":" + port.Port()
}

func TestIntegrationRedisClusterMode(t *testing.T) {
	dsn := redisDSN(t)
	cache, err := redis.NewDirectClient(cachex.Config{
		Driver: "redis",
		Addrs:  []string{dsn},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	ctx := context.Background()

	if err := cache.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Basic Set/Get
	if err := cache.Set(ctx, "integration:key", "value", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var val string
	if err := cache.Get(ctx, "integration:key", &val); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "value" {
		t.Fatalf("val = %q, want value", val)
	}
}
