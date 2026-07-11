# Data & Storage

Aisphere Kernel provides unified, provider-neutral interfaces for database access, caching, object storage, and distributed transactions. Business code must use these interfaces and never import provider SDKs directly.

## Database (`dbx`)

`dbx` is a **batteries-included database access layer** built on GORM (Postgres + MySQL).

### Key Interface

```go
type DB interface {
    GORM(ctx context.Context) *gorm.DB
    // CRUD operations
    FindOne(ctx, dest, conds...) error
    FindMany(ctx, dest, conds...) error
    Create(ctx, data) error
    Save(ctx, data) error
    Update(ctx, model, values, conds...) error
    Delete(ctx, model, conds...) error
    // Transactions
    InTx(ctx, fn func(ctx) error) error
    // Safe upsert
    SafeUpsert(ctx, data, whitelist) error
    // Atomic counters
    Increment(ctx, model, column, value) error
    // Pagination
    Paginate(ctx, dest, page, pageSize, conds...) (total, error)
}
```

### Built-in Safety Features

| Feature | Description |
|---|---|
| Error normalization | `gorm.ErrRecordNotFound` → `dbx.ErrNotFound`; PG `23505` / MySQL `1062` → `dbx.ErrDuplicateKey` |
| Soft delete safety | `WithUnscoped` opt-in to bypass soft delete |
| OnConflict whitelist | `owner_id`, `created_at`, `deleted_at` protected from overwrite |
| Context propagation | `InjectDB` / `InjectTx` / `DB.GORM(ctx)` |
| Audit hook | `BeforeCreate` / `AfterUpdate` / `BeforeDelete` (opt-in) |
| Global QueryTimeout | Configurable timeout for all queries |
| Slow query log | Via GORM callbacks |
| Metrics | Placeholder, opt-in |

### Forbidden Imports

- `database/sql` in business code
- `gorm.io/gorm` direct import (use `dbx.DB` interface)
- `gorm.io/driver/postgres` (use `dbx/postgres`)
- `gorm.io/driver/mysql` (use `dbx/mysql`)
- `github.com/jmoiron/sqlx`, `github.com/jackc/pgx`, `github.com/lib/pq`, `github.com/go-sql-driver/mysql`

**Source**: `/dbx/dbx.go`, `/dbx/errors.go`, `/dbx/observability.go`, `/dbx/observed_db.go`, `/dbx/rawsql.go`, `/dbx/clause_helper.go`

## Repository Pattern (`dbrepo`)

`dbrepo` provides opinionated repository helpers on top of `dbx`:

| Component | Purpose | Source |
|---|---|---|
| `ResourceConfig` | Resource repository configuration | `/dbrepo/resource.go` |
| `ResourceRepository` | Safe CRUD for resource entities | `/dbrepo/resource.go` |

## Migrations (`migrationx`)

`migrationx` provides SQL migration integration:

| Mode | Behavior |
|---|---|
| `disabled` | No migration |
| `dev_apply` | Auto-apply in development |
| `apply` | Apply migrations |
| `validate` | Validate migration state |
| `gorm_dev_auto` | GORM auto-migrate for development |

**Source**: `/migrationx/migrationx.go`

## Caching (`cachex`)

`cachex` provides a unified caching interface:

| Interface | Purpose |
|---|---|
| `Set(ctx, key, value, ttl)` | Set a cache entry |
| `Get(ctx, key, dest)` | Get a cache entry |
| `GetOrSet(ctx, key, dest, ttl, fn)` | Get or compute and set |
| `Delete(ctx, key)` | Delete a cache entry |
| `Exists(ctx, key)` | Check if key exists |
| `TTL(ctx, key)` | Get remaining TTL |

Default provider: **Redis** via `/cachex/redis/`.

**Source**: `/cachex/cachex.go`, `/cachex/observability.go`, `/cachex/errors.go`

## Object Storage (`objectstorex`)

`objectstorex` provides a unified object storage interface:

| Interface | Purpose |
|---|---|
| `PutObject(ctx, bucket, object, reader, size)` | Upload object |
| `GetObject(ctx, bucket, object)` | Download object |
| `DeleteObject(ctx, bucket, object)` | Delete object |
| `PresignURL(ctx, bucket, object, expiry)` | Generate presigned URL |
| `ListObjects(ctx, bucket, prefix)` | List objects |

Default provider: **MinIO** (`/objectstorex/minio/`).

**Source**: `/objectstorex/objectstore.go`, `/objectstorex/observability.go`

## Distributed Transactions (`dtmx`)

`dtmx` provides distributed transaction abstraction using the **Saga pattern**:

| Component | Purpose | Source |
|---|---|---|
| `Manager` | Transaction lifecycle management | `/dtmx/manager.go` |
| `Config` | DTM configuration | `/dtmx/config.go` |
| `Saga` | Saga transaction builder | `/dtmx/types.go` |
| `NewGID` | Generate global transaction ID | `/dtmx/manager.go` |
| `NewSaga` | Create new Saga | `/dtmx/manager.go` |
| `AddHTTP` | Add action + compensate endpoint | `/dtmx/types.go` |
| `SubmitSaga` | Submit Saga for execution | `/dtmx/manager.go` |

Default provider: **DTM** (`/dtmx/dtm/`).

**Important**: DTM is an independent transaction coordinator service — Kernel does not embed the DTM server. Branch endpoints must be idempotent, not exposed to public network, and protected by `branch_secret`.

**Source**: `/dtmx/manager.go`, `/dtmx/config.go`, `/dtmx/types.go`, `/dtmx/context.go`, `/dtmx/auth.go`, `/dtmx/errors.go`, `/dtmx/registry.go`, `/dtmx/observability.go`

## Related Documentation

- [Core Runtime Packages](core-packages.md)
- [docs/ai/dbx.md](../../docs/ai/dbx.md)
- [docs/ai/dtmx.md](../../docs/ai/dtmx.md)
- [docs/process/dbx-acceptance-checklist.md](../../docs/process/dbx-acceptance-checklist.md)