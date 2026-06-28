package cachex_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/cachex/redis"
	"github.com/alicebob/miniredis/v2"
)

func newTestCache(t *testing.T) cachex.Cache {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	cache, err := redis.NewDirectClient(cachex.Config{
		Driver: "redis",
		Addrs:  []string{mr.Addr()},
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	return cache
}

func TestSetAndGet(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	if err := cache.Set(ctx, "user:1", &User{Name: "alice", Age: 30}, 5*time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got User
	if err := cache.Get(ctx, "user:1", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "alice" || got.Age != 30 {
		t.Fatalf("got %+v, want {alice 30}", got)
	}
}

func TestGetNotFound(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	var v string
	err := cache.Get(ctx, "missing", &v)
	if !errors.Is(err, cachex.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDel(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	_ = cache.Set(ctx, "k1", "v1", 0)
	_ = cache.Set(ctx, "k2", "v2", 0)

	if err := cache.Del(ctx, "k1", "k2"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	n, _ := cache.Exists(ctx, "k1", "k2")
	if n != 0 {
		t.Fatalf("Exists = %d, want 0", n)
	}
}

func TestExists(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	_ = cache.Set(ctx, "exists", "v", 0)

	n, err := cache.Exists(ctx, "exists", "missing")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if n != 1 {
		t.Fatalf("Exists = %d, want 1", n)
	}
}

func TestExpireAndTTL(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	_ = cache.Set(ctx, "k", "v", 0)

	if err := cache.Expire(ctx, "k", 10*time.Second); err != nil {
		t.Fatalf("Expire: %v", err)
	}

	ttl, err := cache.TTL(ctx, "k")
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > 10*time.Second {
		t.Fatalf("TTL = %v, want ~10s", ttl)
	}
}

func TestMSetAndMGet(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	pairs := map[string]any{
		"a": "val-a",
		"b": "val-b",
		"c": "val-c",
	}
	if err := cache.MSet(ctx, pairs, 5*time.Minute); err != nil {
		t.Fatalf("MSet: %v", err)
	}

	var results []string
	if err := cache.MGet(ctx, []string{"a", "b", "c"}, &results); err != nil {
		t.Fatalf("MGet: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}
	// results[0] is the JSON-decoded value of "a", which is "val-a" as a JSON string.
	// Since MGet returns []any from redis, the values are already strings.
}

func TestGetOrSet(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	// First call: cache miss, fn is called.
	called := 0
	var result string
	err := cache.GetOrSet(ctx, "compute:1", &result, 5*time.Minute, func(ctx context.Context) (any, error) {
		called++
		return "computed-value", nil
	})
	if err != nil {
		t.Fatalf("GetOrSet: %v", err)
	}
	if result != "computed-value" {
		t.Fatalf("result = %q, want computed-value", result)
	}
	if called != 1 {
		t.Fatalf("fn called %d times, want 1", called)
	}

	// Second call: cache hit, fn is NOT called.
	result = ""
	err = cache.GetOrSet(ctx, "compute:1", &result, 5*time.Minute, func(ctx context.Context) (any, error) {
		called++
		return "should-not-be-called", nil
	})
	if err != nil {
		t.Fatalf("GetOrSet second: %v", err)
	}
	if result != "computed-value" {
		t.Fatalf("result = %q, want computed-value (from cache)", result)
	}
	if called != 1 {
		t.Fatalf("fn called %d times, want 1 (cache hit)", called)
	}
}

func TestSetIfNotExist(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	// First call: key doesn't exist → returns true.
	ok, err := cache.SetIfNotExist(ctx, "lock:1", "owner-1", 30*time.Second)
	if err != nil {
		t.Fatalf("SetIfNotExist: %v", err)
	}
	if !ok {
		t.Fatal("first SetIfNotExist = false, want true")
	}

	// Second call: key exists → returns false.
	ok, err = cache.SetIfNotExist(ctx, "lock:1", "owner-2", 30*time.Second)
	if err != nil {
		t.Fatalf("SetIfNotExist second: %v", err)
	}
	if ok {
		t.Fatal("second SetIfNotExist = true, want false")
	}
}

func TestIncrDecr(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	n, err := cache.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if n != 1 {
		t.Fatalf("Incr = %d, want 1", n)
	}

	n, err = cache.IncrBy(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("IncrBy: %v", err)
	}
	if n != 6 {
		t.Fatalf("IncrBy = %d, want 6", n)
	}

	n, err = cache.Decr(ctx, "counter")
	if err != nil {
		t.Fatalf("Decr: %v", err)
	}
	if n != 5 {
		t.Fatalf("Decr = %d, want 5", n)
	}
}

func TestHashOps(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	if err := cache.HSet(ctx, "hash:1", "field1", "val1"); err != nil {
		t.Fatalf("HSet: %v", err)
	}
	if err := cache.HSet(ctx, "hash:1", "field2", "val2"); err != nil {
		t.Fatalf("HSet: %v", err)
	}

	var v1 string
	if err := cache.HGet(ctx, "hash:1", "field1", &v1); err != nil {
		t.Fatalf("HGet: %v", err)
	}
	if v1 != "val1" {
		t.Fatalf("v1 = %q, want val1", v1)
	}

	// HGetAll into map[string]string
	all := map[string]string{}
	if err := cache.HGetAll(ctx, "hash:1", &all); err != nil {
		t.Fatalf("HGetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}
	if all["field1"] != "val1" || all["field2"] != "val2" {
		t.Fatalf("all = %v", all)
	}

	// HDel
	if err := cache.HDel(ctx, "hash:1", "field1"); err != nil {
		t.Fatalf("HDel: %v", err)
	}
	err := cache.HGet(ctx, "hash:1", "field1", &v1)
	if !errors.Is(err, cachex.ErrNotFound) {
		t.Fatalf("HGet after Del = %v, want ErrNotFound", err)
	}
}

func TestListOps(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	_ = cache.RPush(ctx, "list:1", "a", "b", "c")

	var vals []string
	if err := cache.LRange(ctx, "list:1", 0, -1, &vals); err != nil {
		t.Fatalf("LRange: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("len = %d, want 3", len(vals))
	}

	n, _ := cache.LLen(ctx, "list:1")
	if n != 3 {
		t.Fatalf("LLen = %d, want 3", n)
	}

	var first string
	_ = cache.LPop(ctx, "list:1", &first)
	if first != "a" {
		t.Fatalf("LPop = %q, want a", first)
	}
}

func TestSetOps(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	_ = cache.SAdd(ctx, "set:1", "a", "b", "c")

	isMember, _ := cache.SIsMember(ctx, "set:1", "b")
	if !isMember {
		t.Fatal("SIsMember(b) = false, want true")
	}

	isMember, _ = cache.SIsMember(ctx, "set:1", "z")
	if isMember {
		t.Fatal("SIsMember(z) = true, want false")
	}

	var members []string
	_ = cache.SMembers(ctx, "set:1", &members)
	if len(members) != 3 {
		t.Fatalf("SMembers len = %d, want 3", len(members))
	}

	_ = cache.SRem(ctx, "set:1", "a")
	members = nil
	_ = cache.SMembers(ctx, "set:1", &members)
	if len(members) != 2 {
		t.Fatalf("SMembers after SRem len = %d, want 2", len(members))
	}
}

func TestPubSub(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	sub, err := cache.Subscribe(ctx, "ch1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sub.Close()

	// Small delay to ensure subscription is active.
	time.Sleep(50 * time.Millisecond)

	n, err := cache.Publish(ctx, "ch1", "hello")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if n < 1 {
		t.Fatalf("Publish subscribers = %d, want >= 1", n)
	}

	select {
	case msg := <-sub.Channel:
		if msg.Channel != "ch1" {
			t.Fatalf("msg.Channel = %q, want ch1", msg.Channel)
		}
		// Payload is JSON-encoded "hello" = "hello" (with quotes).
		if msg.Payload != `"hello"` {
			t.Fatalf("msg.Payload = %q, want \"hello\"", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPing(t *testing.T) {
	cache := newTestCache(t)
	ctx := context.Background()

	if err := cache.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
