# cachex

`cachex` 是 Aisphere Kernel 的统一缓存模块。它基于 Redis(go-redis/v9),支持单点、集群、哨兵三种模式,提供缓存语义 + Redis 数据结构 + Pub/Sub 全套能力。

## 快速上手

```go
import (
    "github.com/aisphereio/kernel/cachex"
    _ "github.com/aisphereio/kernel/cachex/redis"
)

cache, _ := cachex.New(cachex.Config{
    Driver: "redis",
    Addrs:  []string{"localhost:6379"},
})
defer cache.Close()

// 基础 CRUD
cache.Set(ctx, "user:1", &user, 5*time.Minute)
cache.Get(ctx, "user:1", &user)

// GetOrSet (缓存穿透防护)
cache.GetOrSet(ctx, "user:1", &user, 5*time.Minute, func(ctx) (any, error) {
    return db.FindUser(ctx, 1)
})

// 分布式锁
ok, _ := cache.SetIfNotExist(ctx, "lock:job:42", "owner", 30*time.Second)

// 原子计数器
n, _ := cache.Incr(ctx, "rate_limit:user:1")

// Pub/Sub
sub, _ := cache.Subscribe(ctx, "notifications")
cache.Publish(ctx, "notifications", "hello")
```

## 模式

| 模式 | 配置 |
|---|---|
| 单点 | `Addrs: ["localhost:6379"]` |
| 集群 | `Addrs: ["node1:6379","node2:6379","node3:6379"], Cluster: true` |
| 哨兵 | `Addrs: ["sentinel1:26379","sentinel2:26379"], MasterName: "mymaster"` |

## API

| 类别 | 方法 |
|---|---|
| 基础 CRUD | Get / Set / Del / Exists / Expire / TTL |
| 批量 | MGet / MSet |
| 函数式 | GetOrSet |
| 分布式锁 | SetIfNotExist |
| 计数器 | Incr / IncrBy / Decr |
| Pub/Sub | Publish / Subscribe |
| Hash | HSet / HGet / HGetAll / HDel |
| List | LPush / RPush / LPop / RPop / LRange / LLen |
| Set | SAdd / SRem / SMembers / SIsMember |
| 生命周期 | Ping / Close |

## 测试

```bash
# 单元测试(miniredis,不需要 Docker)
go test ./cachex/... -short

# 集成测试(testcontainers,需要 Docker)
go test ./cachex/...

# 或用外部 Redis
KERNEL_CACHEX_REDIS_ADDR=localhost:6379 go test ./cachex/... -run TestIntegration
```
