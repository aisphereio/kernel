# dbx 设计文档

## 1. 目标

`dbx` 是 Aisphere Kernel 的统一数据库访问模块。目标:

1. **开箱即用** — Kernel 包拿过去就能跑,业务代码不用重复写样板
2. **GORM 底层** — 复用 GORM 的链式 API / Relations / Hooks / Clauses,不自创 ORM
3. **八项内置能力** — 把 aisphere-hub 里 100+ 处重复样板收敛成类型安全 API
4. **PG + MySQL 完全对称** — 同一套 API,driver 差异透明
5. **安全门** — SafeUpsert 白名单 / WithUnscoped opt-in / 错误归一化,防止业务误用

非目标:

- 重写 GORM(GoFrame gdb 那种 14k 行工程,不在范围内)
- schema migration 引擎(用第三方工具)
- ORM 关系映射的复杂查询(用 `db.GORM(ctx)` 逃生口)
- 缓存(用 cachex 单独管)
- 多数据库 fan-out(用 transportx)

## 2. 架构

```text
┌─────────────────────────────────────────────────────┐
│ Business code (handler/service/repository/worker)   │
│ Uses dbx.DB / dbx.Tx interface                      │
│   FindOne / FindMany / Create / SafeUpsert /        │
│   Increment / Paginate / InTx / ...                 │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│ dbx package                                          │
│ - DB / Tx / Config / PageResult interfaces          │
│ - New(), RegisterDriver(), RegisterErrorMapper()    │
│ - InjectDB / InjectTx / WithUnscoped (ctx 透传)     │
│ - SafeUpsert (白名单保护)                           │
│ - Increment / Paginate (便利层)                     │
│ - AssertAffected (ErrNoEffect 转换)                 │
│ - wrapDriverErr (错误归一化)                        │
│ - 8 项内置能力(部分通过 driver callback 实现)      │
└──────┬──────────────────────────────┬───────────────┘
       │                              │
       ▼                              ▼
┌──────────────────┐         ┌──────────────────┐
│ dbx/postgres     │         │ dbx/mysql        │
│ - Registers      │         │ - Registers      │
│   "postgres"     │         │   "mysql"        │
│   driver + mapper│         │   driver + mapper│
│ - gorm.io/       │         │ - gorm.io/       │
│   driver/postgres│         │   driver/mysql   │
│ - Slow query log │         │ - Slow query log │
│   via callbacks  │         │   via callbacks  │
└──────────────────┘         └──────────────────┘
       │                              │
       ▼                              ▼
   Postgres                        MySQL
```

## 3. API 设计

### 3.1 DB 接口

`DB` 是 business code 的唯一依赖:

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
```

设计决策:
- `GORM(ctx)` 是逃生口,给 Preload / 复杂 Where / 原生 SQL 用
- `FindOne` / `FindMany` / `Count` 覆盖 90% 查询场景
- `Create` / `Save` / `Update` / `Delete` 覆盖 90% 写场景
- `SafeUpsert` / `Increment` / `Paginate` 是语义化 helper,消除样板
- `InTx` 回调式事务,自动 commit/rollback/panic-recover
- `AutoMigrate` 提供 dev 入口,生产禁用
- `Tables` 给 schema introspection 用

### 3.2 Tx 接口

`Tx` 镜像 `DB` 但加 `Commit` / `Rollback`:

```go
type Tx interface {
    GORM(ctx) *gorm.DB
    FindOne(...) / FindMany(...) / Count(...) / Create(...) / Save(...) / ...
    SafeUpsert(...) / Increment(...)
    Commit() error
    Rollback() error
}
```

设计决策:
- 没有 `BeginTx`(不支持嵌套事务;`InTx` 嵌套时复用外层 tx)
- 没有 `Paginate`(事务内分页少见,需要时用 `tx.GORM(ctx)` 自己写)
- 在已 done 的 Tx 上调用任何方法返回 `ErrTxCommitted` 或 `ErrTxRolledBack`

### 3.3 Config

```go
type Config struct {
    Driver, DSN string
    MaxOpenConns, MaxIdleConns int
    ConnMaxLifetime, ConnMaxIdleTime, QueryTimeout, SlowQueryThreshold time.Duration
    AuditEnabled, MetricsEnabled, DryRun, Debug bool
}
```

设计决策:
- `Driver` 决定走哪个 driver subpackage
- 时间字段用 `time.Duration`(纳秒),配置文件用 `_ns` 后缀
- 0 值表示"使用 driver 默认"
- `AuditEnabled` / `MetricsEnabled` 是 opt-in,避免意外开启
- `DryRun` / `Debug` 给 dev / staging 用

### 3.4 错误归一化

```go
var (
    ErrNoRows             = newSentinel("dbx: no rows in result set")
    ErrDuplicateKey       = newSentinel("dbx: duplicate key")
    ErrTimeout            = newSentinel("dbx: query timed out")
    ErrSchemaNotReady     = newSentinel("dbx: schema not ready (run migrations)")
    ErrForeignKeyViolation = newSentinel("dbx: foreign key violation")
    ErrClosed             = newSentinel("dbx: database is closed")
    ErrNilConfig          = newSentinel("dbx: config is missing required fields")
    ErrUnknownDriver      = newSentinel("dbx: unknown driver ...")
    ErrTxRolledBack       = newSentinel("dbx: transaction already rolled back")
    ErrTxCommitted        = newSentinel("dbx: transaction already committed")
    ErrUnscopedRequired   = newSentinel("dbx: query touches soft-deleted rows ...")
    ErrUnsafeUpsert       = newSentinel("dbx: SafeUpsert blocked a protected column")
    ErrNoEffect           = newSentinel("dbx: operation affected 0 rows")
)
```

设计决策:
- 用自定义 `errorSentinel` 类型(不用 `errors.New`),实现 `Is(target) bool` 支持链式匹配
- driver subpackage 通过 `RegisterErrorMapper` 注册错误检测函数
- 检测函数用 typed path(`errors.As` against `*pgconn.PgError` / `*mysql.MySQLError`)+ message-string fallback

### 3.5 SafeUpsert

```go
var protectedColumns = map[string]struct{}{
    "owner_id":   {},
    "created_at": {},
    "deleted_at": {},
}

func safeUpsert(gormDB, dest, allowedColumns) error {
    // 检查白名单是否含保护列
    for _, col := range allowedColumns {
        if _, blocked := protectedColumns[col]; blocked {
            return ErrUnsafeUpsert
        }
    }
    // 执行 ON CONFLICT DO UPDATE
    gormDB.Clauses(clause.OnConflict{
        DoUpdates: clause.AssignmentColumns(allowedColumns),
    }).Create(dest)
}
```

设计决策:
- 保护列列表是 dbx 内置的,业务**不能**修改
- 即使白名单误列保护列,也直接拒绝执行(防御深度)
- 用 GORM 的 `clause.OnConflict`,driver 自动翻译成对应 SQL(PG `ON CONFLICT` / MySQL `ON DUPLICATE KEY UPDATE`)

### 3.6 Context 透传

```go
func InjectDB(ctx, *gorm.DB) context.Context
func InjectTx(ctx, Tx) context.Context
func WithUnscoped(ctx) context.Context

func (d *db) GORM(ctx) *gorm.DB {
    // Priority 1: InjectDB
    if injected, ok := ctx.Value(ctxKeyDB).(*gorm.DB); ok { return injected }
    // Priority 2: InjectTx
    if tx, ok := ctx.Value(ctxKeyTx).(Tx); ok { return tx.GORM(ctx) }
    // Priority 3: pool DB
    return d.gormDB.WithContext(ctx)
}
```

设计决策:
- 用 private ctxKey,业务只能通过 `InjectDB` / `InjectTx` / `WithUnscoped` 设置
- `InTx(fn)` 自动注入 tx 到 ctx,fn 内部调 `r.db.FindOne(ctx, ...)` 自动用这个 tx
- 业务 repo 不需要写 `db(ctx)` helper,也不需要 `WithTx(tx)` 版本

### 3.7 InTx 回调式事务

```go
func (d *db) InTx(ctx, fn) (err error) {
    // 嵌套 InTx 复用外层 tx
    if existing, ok := ctx.Value(ctxKeyTx).(Tx); ok {
        return fn(existing)
    }
    gormTx := d.GORM(ctx).Begin()
    tx := &txImpl{gormTx: gormTx, parent: d}
    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback()
            panic(p)
        }
        if err != nil { _ = tx.Rollback(); return }
        err = tx.Commit()
    }()
    txCtx := InjectTx(ctx, tx)
    return fn(&txCtxAdapter{tx: tx, ctx: txCtx})
}
```

设计决策:
- panic 时 rollback 后 re-panic(保留 panic 行为)
- err 非 nil 时 rollback,但不包装 err
- Commit 失败时 err 被赋值为 commit error
- `txCtxAdapter` 让 fn 内部调 `r.db.FindOne(ctx, ...)` 自动用 tx

### 3.8 Slow Query Log + Metrics

通过 GORM Callbacks 实现:

```go
gormDB.Callback().Query().Before("gorm:query").Register("dbx:before_query", func(tx *gorm.DB) {
    tx.InstanceSet("dbx:start_time", time.Now())
})
gormDB.Callback().Query().After("gorm:query").Register("dbx:after_query", func(tx *gorm.DB) {
    start, _ := tx.InstanceGet("dbx:start_time")
    elapsed := time.Since(start.(time.Time))
    if elapsed > cfg.SlowQueryThreshold {
        logx.Warn("slow db query", ...)
    }
})
```

设计决策:
- 用 GORM 的 Callback 机制,不自创 hook
- Before/After 配对,用 `InstanceSet` / `InstanceGet` 传递 start time
- SQL 文本截断到 500 字符,避免日志爆炸
- Metrics 是占位,等 metricsx 集成后接入

## 4. Driver 注册

driver subpackage 通过 `init()` 注册:

```go
// dbx/postgres/postgres.go
func init() {
    dbx.RegisterDriver("postgres", open)
    dbx.RegisterErrorMapper(mapError)
}
```

设计决策:
- `init()` 自动注册,调用方只需 `import _ "dbx/postgres"`
- 重复注册会 panic(init 期 bug 立刻暴露)
- ErrorMapper 用 typed path + message-string fallback,避免 import driver 类型
- Slow query / Debug log 通过 GORM Callback 注册,在 driver subpackage 内完成

## 5. 错误转换边界

dbx 不 import errorx,避免循环依赖:

```text
dbx (no errorx import)
  ↓ repository layer converts
errorx (business error package)
```

repository 层负责把 dbx 错误转换为 errorx:

```go
err := r.db.FindOne(ctx, &s, q, id)
if errors.Is(err, dbx.ErrNoRows) {
    return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", ...)
}
if errors.Is(err, dbx.ErrDuplicateKey) {
    return nil, errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", ...)
}
if err != nil {
    return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED", ...)
}
```

## 6. 与 GORM 的关系

dbx 是 GORM 之上的"安全 + 便利"层:

- 底层用 `*gorm.DB` / `gorm.Tx` / `gorm.DBStats`
- 不重新实现连接池(用 GORM 的 `sql.DB` 池)
- 不重新实现事务(用 GORM 的 `Transaction` / `Begin` / `Commit`)
- 不重新实现链式 Where(业务用 `db.GORM(ctx).Where(...)` 逃生口)
- 添加了 SafeUpsert / Increment / Paginate 语义化 helper
- 添加了错误归一化 / Soft Delete 安全门 / Context 透传
- 添加了 Slow Query Log / Metrics(callback 形式)

`db.GORM(ctx)` 是合法逃生口,不是禁止模式。但业务代码应优先用 dbx 的类型安全 API。

## 7. 测试策略

dbx 的测试分三层:

1. **contract_test.go** — 契约测试,验证 public API 不变。不需要真实 DB。
2. **pg_integration_test.go / mysql_integration_test.go** — 真实 DB 集成测试,用 testcontainers-go 起容器。需要 Docker。
3. **example_test.go / example_business_test.go** — Go 标准 Example,展示 API 用法。

测试覆盖率目标:
- 不需要真实 DB 的代码:>= 40%
- 需要 DB 的代码:用 testcontainers 覆盖

testcontainers 配置:
- PG: `postgres:16-alpine`,用 pgx driver
- MySQL: `mysql:8.0`,charset=utf8mb4,parseTime=true

## 8. 未来扩展

可能添加的 API(不破坏现有契约):

- `DB.Preload(ctx, &dest, "Relations", query, args...)` — Preload helper
- `DB.BatchCreate(ctx, &rows, batchSize)` — 批量插入
- `DB.WithLogger(logx.Logger) DB` — 注入 slow-query logger
- `DB.WithMetrics(metricsx.Metrics) DB` — 注入 metrics collector
- `DB.WithAudit(auditx.Service) DB` — 注入 audit hook
- `DB.HealthCheck(ctx) error` — 替代 PingContext,带超时和表存在性检查

不会添加的 API:

- ORM 关系映射(belongs_to / has_many / preload 自动化)— 用 GORM 逃生口
- schema migration 引擎 — 用第三方工具
- 多数据库 fan-out — 用 transportx
- 自动 retry — 用 workerx
- Sharding — 暂不需要
- 读写分离 — 暂不需要

## 9. 与其他模块的关系

```text
configx → dbx.Config (Scan)
logx    → dbx slow query log (via callback)
errorx  ← repository layer converts dbx errors
metricsx → future dbx.WithMetrics (query duration histogram)
auditx  → future dbx.WithAudit (BeforeCreate / AfterUpdate / BeforeDelete)
workerx → consumes dbx errors for retry decision
```

## 10. 性能考虑

- dbx 接口方法调用比直接调 GORM 多一层间接,但开销 < 10ns,可忽略
- SafeUpsert 的白名单检查是 O(n) map lookup,n 通常 < 10,开销 < 100ns
- Slow Query Log 的 callback 开销:每次查询 +2 次 InstanceSet/Get + 1 次 time.Since,约 200ns
- Metrics callback(未来)预计 +500ns/查询
- 连接池大小对性能影响最大:太大会拖垮 PG,太小会让请求排队。经验值见 README § 10

## 11. 与 aisphere-hub 的迁移路径

aisphere-hub 的 `internal/data/skill.go` 迁移到 dbx 的步骤:

1. 把 `skillModel` 的 `gorm:` tag 保持不变(dbx 完全兼容 GORM tag)
2. 删除 `r.db(ctx)` helper — 用 `r.db.GORM(ctx)` 替代
3. 删除 `isUniqueViolation(err)` / `mapSkillDBError(err)` — dbx 自动归一化
4. 把 `db.Where(...).First(&row)` 改成 `db.FindOne(ctx, &row, ...)`
5. 把 `db.Create(&row)` 改成 `db.Create(ctx, &row)`
6. 把 `db.Transaction(func(tx *gorm.DB) error {...})` 改成 `db.InTx(ctx, func(tx dbx.Tx) error {...})`
7. 把 `clause.OnConflict{ DoUpdates: ... }` 改成 `db.SafeUpsert(ctx, row, allowedColumns)`
8. 把 `tx.UpdateColumn("count", gorm.Expr("count + ?", delta))` 改成 `db.Increment(ctx, ...)`
9. 把手写 `limit+1` + `hasMore` 改成 `db.Paginate(ctx, ...)`
10. 把 `Unscoped()` 调用改成 `dbx.WithUnscoped(ctx)` + `db.FindOne(ctx, ...)`

迁移后 skill.go 预计从 1687 行降到 ~1200 行(-30%),样板代码被 dbx 内置能力替代。
