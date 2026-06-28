# dbx 契约

本契约对 Kernel 及业务模块是强制性要求。

## 包使用规则

`github.com/aisphereio/kernel/dbx` 是唯一的 Kernel 数据库访问包。

在 Kernel/业务 API 数据访问路径中禁止以下导入:

```go
import "database/sql"
import "gorm.io/gorm"                        // 用 dbx.DB / dbx.Tx 接口替代
import "gorm.io/driver/postgres"            // 用 dbx/postgres 替代
import "gorm.io/driver/mysql"               // 用 dbx/mysql 替代
import "github.com/jmoiron/sqlx"
import "github.com/jackc/pgx/v5"            // 用 dbx/postgres 替代
import "github.com/lib/pq"                  // 用 dbx/postgres 替代
import "github.com/go-sql-driver/mysql"     // 用 dbx/mysql 替代
```

测试代码(`*_test.go`)允许在集成测试里直接用 `gorm.Open` 建独立 GORM 实例,但生产代码不允许。

## 公共 API

以下符号保持 source 兼容,不可以在不升 major 版本的情况下破坏:

```go
type DB interface {
    GORM(ctx context.Context) *gorm.DB
    FindOne(ctx, dest, query, args...) error
    FindOneByPK(ctx, dest, pk) error
    FindMany(ctx, dest, query, args...) error
    Count(ctx, model, query, args...) (int64, error)
    Create(ctx, dest) error
    Save(ctx, dest) error
    Update(ctx, model, query, args, columns) error
    UpdateColumns(ctx, model, query, args, columns) error
    Delete(ctx, model, query, args...) error
    DeleteByPK(ctx, model, pk) error
    SafeUpsert(ctx, dest, allowedColumns) error
    Increment(ctx, model, query, args, column, delta) error
    Paginate(ctx, dest, model, query, args, page, size) (*PageResult, error)
    InTx(ctx, fn func(Tx) error) error
    BeginTx(ctx) (Tx, error)
    PingContext(ctx) error
    AutoMigrate(ctx, models...) error
    Tables(ctx) ([]string, error)
    Stats() DBStats
    DriverName() string
    Close() error
}

type Tx interface {
    // 所有 DB 的查询/写方法 + Commit / Rollback
    GORM(ctx) *gorm.DB
    FindOne(...) / FindMany(...) / Count(...) / Create(...) / Save(...) / ...
    SafeUpsert(...) / Increment(...)
    Commit() error
    Rollback() error
}

type Config struct {
    Driver, DSN string
    MaxOpenConns, MaxIdleConns int
    ConnMaxLifetime, ConnMaxIdleTime, QueryTimeout, SlowQueryThreshold time.Duration
    AuditEnabled, MetricsEnabled, DryRun, Debug bool
}

type PageResult struct {
    Items []any
    Total int64
    Page, Size int
    HasMore bool
}

func New(cfg Config) (DB, error)
func RegisterDriver(name string, fn DriverOpener)
func RegisterErrorMapper(fn func(error) error)
func IsDriverRegistered(name string) bool
func RegisteredDrivers() []string
func InjectDB(ctx, *gorm.DB) context.Context
func InjectTx(ctx, Tx) context.Context
func WithUnscoped(ctx) context.Context
func AssertAffected(*gorm.DB) error
```

## Driver 注册契约

- Driver subpackage 通过 `init()` 调用 `RegisterDriver` + `RegisterErrorMapper`
- Driver 名称 "postgres" 和 "mysql" 保留,不可被业务 driver 覆盖
- 重复注册同名 driver 会 panic
- Driver 必须实现 duplicate-key 检测(PG 23505 / MySQL 1062)

## 错误归一化契约

| dbx 错误 | 触发条件 |
|---|---|
| `ErrNoRows` | `FindOne` / `FindOneByPK` 无结果;GORM `gorm.ErrRecordNotFound` 自动转换 |
| `ErrDuplicateKey` | PG 23505 / MySQL 1062 |
| `ErrTimeout` | `context.DeadlineExceeded` / `context.Canceled` |
| `ErrSchemaNotReady` | PG 42P01 / MySQL 1146(表不存在) |
| `ErrForeignKeyViolation` | PG 23503 / MySQL 1452 |
| `ErrClosed` | 在已 Close 的 DB 上操作 |
| `ErrTxCommitted` | 在已 Commit 的 Tx 上操作 |
| `ErrTxRolledBack` | 在已 Rollback 的 Tx 上操作 |
| `ErrUnsafeUpsert` | SafeUpsert 白名单包含保护列 |
| `ErrNoEffect` | AssertAffected 检测 RowsAffected == 0 |
| `ErrNilConfig` | New 时 DSN 或 Driver 为空 |
| `ErrUnknownDriver` | Config.Driver 未注册 |

所有 sentinel 都支持 `errors.Is` 链式匹配,即使被 `fmt.Errorf("%w: ...", err)` 包装。

## SafeUpsert 契约

`SafeUpsert(ctx, dest, allowedColumns)` 的行为:

1. 检查 `allowedColumns` 是否包含保护列(`owner_id` / `created_at` / `deleted_at`)
2. 包含则返回 `ErrUnsafeUpsert`,不执行 SQL
3. 不包含则执行 `INSERT ... ON CONFLICT DO UPDATE`(PG)或 `INSERT ... ON DUPLICATE KEY UPDATE`(MySQL)
4. 冲突时只更新 `allowedColumns` 列出的列

保护列列表是 dbx 内置的,业务**不能**通过任何 API 修改或绕过。即使 `allowedColumns` 列出保护列,dbx 也会拒绝执行。

## Soft Delete 契约

模型有 `gorm.DeletedAt` 字段时:
- 普通查询自动加 `WHERE deleted_at IS NULL`
- `Delete` 是软删除(`UPDATE ... SET deleted_at = NOW()`)
- 查已删除行必须用 `dbx.WithUnscoped(ctx)` opt-in
- 业务代码**不能**直接调 `db.GORM(ctx).Unscoped()`(虽然技术上可以,但会绕过安全门,属于禁止模式)

## Context 透传契约

`DB.GORM(ctx)` 的优先级:
1. `InjectDB(ctx, *gorm.DB)` 注入的请求级 DB(通常是 tx)
2. `InjectTx(ctx, Tx)` 注入的 Tx(unwrap 到 *gorm.DB)
3. 全局 *gorm.DB,加 ctx via WithContext

`InTx(fn)` 自动把 tx 注入到 ctx,所以 fn 内部调 `r.db.FindOne(ctx, ...)` 会自动用这个 tx,不需要显式传 tx 参数。

## QueryTimeout 契约

- `Config.QueryTimeout > 0` 时,若 ctx 无 deadline,自动加 timeout
- ctx 已有 deadline 时不重复加
- timeout 通过 `context.WithTimeout` 实现,会泄漏一个 timer(直到 timer 触发)
- 业务代码应优先在 handler 边界传带 deadline 的 ctx,避免依赖 QueryTimeout

## 事务契约

- `BeginTx` 在 ctx 取消时返回 error
- `InTx` 在 fn 返回 nil 时 commit,返回 error 时 rollback,panic 时 rollback 并 re-panic
- `Commit` 在已 Commit 后返回 `ErrTxCommitted`
- `Commit` 在已 Rollback 后返回 `ErrTxRolledBack`
- `Rollback` 是幂等的:在已 Rollback 后返回 nil;在已 Commit 后返回 `ErrTxCommitted`
- 在已 done 的 Tx 上调用查询/写方法返回 `ErrTxCommitted` 或 `ErrTxRolledBack`
- `InTx` 嵌套调用(在 fn 内部再调 InTx)会复用外层 tx,不会开新事务

## Close 契约

- `Close` 是幂等的,多次调用返回 nil
- `Close` 后所有查询返回 driver 错误
- `Close` 不等待进行中的 query 完成

## AutoMigrate 契约

- `AutoMigrate` 是幂等的
- 只新增列/表/索引,**不**删除、**不**修改列类型
- `DisableForeignKeyConstraintWhenMigrating: true`,不创建外键约束
- **生产环境禁用**,只用 SQL migration

## Underlying (GORM) 契约

`DB.GORM(ctx)` 返回 `*gorm.DB`,供 Preload / 复杂 Where / 原生 SQL 使用。这是合法的逃生口,不是禁止模式。但:
- 业务代码应优先用 dbx 的类型安全 API(FindOne / Create / SafeUpsert 等)
- 只有 dbx API 表达不了的查询才用 GORM
- 用 GORM 时仍受 Soft Delete / QueryTimeout / Slow Query Log / Metrics 保护

## Migration 契约

- dbx 提供 `AutoMigrate` 给 dev 环境
- 生产环境用 SQL migration(golang-migrate / goose / sql-migrate)
- 迁移 SQL 不属于业务代码,不受"禁止 import driver"规则约束
- 迁移工具可以调 `db.GORM(ctx)` 获取底层连接

## Postgres 特定契约

- DSN 接受 `postgres://` / `postgresql://` URL 或 key=value 格式
- 占位符:`$1, $2, $3`(GORM 自动处理)
- Duplicate key 检测:`pgconn.PgError.Code == "23505"` + 消息兜底
- Schema not ready 检测:`pgconn.PgError.Code == "42P01"` + 消息兜底
- Soft delete:`gorm.DeletedAt` + `deleted_at` 列(TIMESTAMPTZ)

## MySQL 特定契约

- DSN 格式:`user:pass@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4`
- `parseTime=true` **必须**(否则 `time.Time` 扫描失败)
- 占位符:`?, ?, ?`(GORM 自动处理)
- Duplicate key 检测:`mysql.MySQLError.Number == 1062` + 消息兜底
- Schema not ready 检测:`mysql.MySQLError.Number == 1146` + 消息兜底
- Soft delete:`gorm.DeletedAt` + `deleted_at` 列(DATETIME 或 TIMESTAMP)

## 与 errorx 的边界

dbx **不导入** errorx,避免循环依赖。错误转换在 repository 层完成:

```go
if errors.Is(err, dbx.ErrNoRows) {
    return nil, errorx.NotFound(...)
}
```

dbx 错误不应该被业务代码包装成业务 errorx 错误以外的任何东西。配置错误(`ErrNilConfig` / `ErrUnknownDriver`)一般在启动期就 fail-fast,不应在请求路径出现。
