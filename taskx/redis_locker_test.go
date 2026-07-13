package taskx

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLockerContentionAndRelease(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	locker := NewRedisLocker(client, "test:taskx:")
	ctx := context.Background()

	first, acquired, err := locker.TryAcquire(ctx, "grant-expirer", time.Minute)
	if err != nil {
		t.Fatalf("first TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("first TryAcquire() acquired = false, want true")
	}
	if _, acquired, err := locker.TryAcquire(ctx, "grant-expirer", time.Minute); err != nil {
		t.Fatalf("second TryAcquire() error = %v", err)
	} else if acquired {
		t.Fatal("second TryAcquire() acquired = true, want false")
	}
	if err := first.Renew(ctx, 2*time.Minute); err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	if err := first.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, acquired, err := locker.TryAcquire(ctx, "grant-expirer", time.Minute); err != nil {
		t.Fatalf("third TryAcquire() error = %v", err)
	} else if !acquired {
		t.Fatal("third TryAcquire() acquired = false, want true")
	}
}

func TestRedisLeaseDoesNotDeleteAnotherOwner(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	locker := NewRedisLocker(client, "test:taskx:")
	ctx := context.Background()

	lease, acquired, err := locker.TryAcquire(ctx, "job", time.Minute)
	if err != nil || !acquired {
		t.Fatalf("TryAcquire() = acquired %v, err %v", acquired, err)
	}
	const key = "test:taskx:job"
	server.Set(key, "replacement-owner")
	if err := lease.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	value, err := server.Get(key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "replacement-owner" {
		t.Fatalf("value = %q, want replacement-owner", value)
	}
}
