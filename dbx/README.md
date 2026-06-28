# dbx

`dbx` 是 Aisphere Kernel 的统一数据库访问模块。它在 GORM 之上封装了八项开箱即用的"预先动作",让业务代码不用再重复 aisphere-hub 里那 100+ 处样板。Kernel 包**开箱即用**,业务代码拿过去就能跑,而且用得特别好。

> **新手上路**:只看本文件即可上手。需要深度细节时再翻其他文档(见末尾"文档地图")。

---

## 1. 为什么需要 dbx

aisphere-hub 的 `internal/data/skill.go`(1687 行)+ `skillset.go`(500 行)里,我们数到至少 108 处重复样板:

| 重复样板 | 出现次数 | dbx 的解决方式 |
|---|---:|---|
| `r.db(ctx)` helper(优先用 ctx 里的 tx,否则用全局 DB) | 每个 repo 一份 | `DB.GORM(ctx)` 内置 context 透传 |
| `isUniqueViolation(err)` 检查 pgconn.PgError 23505 | 6+ 处 | `dbx.ErrDuplicateKey` 自动归一化 |
| `if res.RowsAffected == 0 { return ErrXxxNotFound }` | 10+ 处 | `dbx.AssertAffected(res)` 一行搞定 |
| `clause.OnConflict{ DoUpdates: ... }` + SECURITY 白名单注释 | 3+ 处 | `db.SafeUpsert(row, allowedColumns)` |
| `Unscoped()` 查已删除记录(误用风险) | 5+ 处 | `dbx.WithUnscoped(ctx)` 显式 opt-in |
| `time.Now()` 填充 updated_at | 10+ 处 | GORM `autoUpdateTime` tag 自动处理 |
| `UpdateColumn("count", gorm.Expr("count + ?", delta))` 原子计数 | 2+ 处 | `db.Increment(model, where, col, delta)` |
| `limit+1` + `hasMore` + `NextOffset` 分页样板 | 3+ 处 | `db.Paginate(ctx, &out, model, where, page, size)` |
| `mapSkillDBError` 翻译 42P01 undefined table | 2+ 处 | `dbx.ErrSchemaNotReady` 提示运行 migration |

dbx 把这些样板收敛成一套 DB / Tx 接口,底层用 GORM 的全部能力(链式 Where、Preload、Relations、Hooks、Clauses),业务代码只见 dbx 的类型安全 API。

```text
configx → dbx.Config
  ↓ dbx.New
dbx.DB (interface, 8 项内置能力)
  ↓ FindOne / FindMany / Create / Save / Update / Delete / SafeUpsert / Increment / Paginate / InTx
dbx.Tx (interface, 同上 + Commit/Rollback)
  ↓
dbx/postgres / dbx/mysql driver subpackages (register via init())
  ↓
GORM → database/sql → pgx / go-sql-driver/mysql
```

dbx 本身不负责 schema migration(提供 AutoMigrate 入口给 dev 环境,生产用 SQL migration)、ORM 关系映射的复杂查询(用 `db.GORM(ctx)` 逃生口)、缓存(用 cachex 单独管)。

---

## 2. 30 秒上手

```go
package main

import (
    "log"
    "time"

    "github.com/aisphereio/kernel/configx"
    "github.com/aisphereio/kernel/configx/file"
    "github.com/aisphereio/kernel/dbx"
    _ "github.com/aisphereio/kernel/dbx/postgres" // 注册 "postgres" driver
)

type DBConfig struct {
    Driver          string `json:"driver"`
    DSN             string `json:"dsn"`
    MaxOpenConns    int    `json:"max_open_conns"`
    MaxIdleConns    int    `json:"max_idle_conns"`
    ConnMaxLifetime int64  `json:"conn_max_lifetime_ns"`
    QueryTimeout    int64  `json:"query_timeout_ns"`
}

func main() {
    cfg := configx.New(configx.WithSource(file.NewSource("configs/app.yaml")))
    defer cfg.Close()
    if err := cfg.Load(); err != nil { log.Fatal(err) }

    var dbCfg DBConfig
    if err := cfg.Value("database").Scan(&dbCfg); err != nil { log.Fatal(err) }

    db, err := dbx.New(dbx.Config{
        Driver:          dbCfg.Driver,
        DSN:             dbCfg.DSN,
        MaxOpenConns:    dbCfg.MaxOpenConns,
        MaxIdleConns:    dbCfg.MaxIdleConns,
        ConnMaxLifetime: time.Duration(dbCfg.ConnMaxLifetime),
        QueryTimeout:    time.Duration(dbCfg.QueryTimeout),
        SlowQueryThreshold: 200 * time.Millisecond,
    })
    if err != nil { log.Fatal(err) }
    defer db.Close()
    // ... pass db to repositories
}
```

业务代码使用:

```go
type Skill struct {
    ID          int64          `gorm:"primaryKey;autoIncrement;column:id"`
    Name        string         `gorm:"column:name;size:128;uniqueIndex;not null"`
    DisplayName string         `gorm:"column:display_name;size:256;not null;default:''"`
    OwnerID     string         `gorm:"column:owner_id;size:128;not null;default:''"`
    Status      string         `gorm:"column:status;size:32;not null;default:'active'"`
    CreatedAt   time.Time      `gorm:"column:created_at;not null;autoCreateTime"`
    UpdatedAt   time.Time      `gorm:"column:updated_at;not null;autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

type SkillRepo struct{ db dbx.DB }

func (r *SkillRepo) Find(ctx context.Context, name string) (*Skill, error) {
    var skill Skill
    err := r.db.FindOne(ctx, &skill, "name = ?", name)
    if errors.Is(err, dbx.ErrNoRows) {
        return nil, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
            errorx.WithMetadata("skill_id", name),
        )
    }
    if err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
            errorx.WithMessage("查询技能失败"),
            errorx.WithRetryable(true),
        )
    }
    return &skill, nil
}
```

---

## 3. 八项内置能力速查

| 能力 | 配置 | 自动行为 |
|---|---|---|
| **错误归一化** | 默认开启 | `gorm.ErrRecordNotFound` → `dbx.ErrNoRows`;PG 23505 / MySQL 1062 → `dbx.ErrDuplicateKey`;PG 42P01 / MySQL 1146 → `dbx.ErrSchemaNotReady`;ctx 超时 → `dbx.ErrTimeout` |
| **Soft Delete 安全** | 默认开启 | 模型有 `DeletedAt` 字段时,普通查询自动过滤已删除行;`dbx.WithUnscoped(ctx)` 才能查已删除 |
| **OnConflict 白名单** | `db.SafeUpsert(row, allowedColumns)` | `owner_id` / `created_at` / `deleted_at` 永远不被覆盖,即使误列入白名单也会报 `ErrUnsafeUpsert` |
| **Context 透传** | 默认开启 | `DB.GORM(ctx)` 自动优先用 `InjectDB` 注入的请求级 tx;repo 不用每个方法写 `db(ctx)` helper |
| **审计 Hook** | `Config.AuditEnabled = true` | BeforeCreate / AfterUpdate / BeforeDelete 自动调 auditx(需 auditx 集成) |
| **QueryTimeout 全局** | `Config.QueryTimeout > 0` | ctx 无 deadline 时自动加 timeout;有 deadline 时不重复加 |
| **Slow Query Log** | `Config.SlowQueryThreshold > 0` | 超阈值查询自动 `logx.Warn`,带 SQL 文本(截断 500 字符)、耗时、影响行数 |
| **Metrics 埋点** | `Config.MetricsEnabled = true` | 每个查询自动 `metricsx.Histogram(db_query_duration, labels=driver/op/status)`(需 metricsx 集成,占位) |

---

## 4. 构造器速查

| 场景 | API | 说明 |
|---|---|---|
| 创建 DB | `dbx.New(cfg)` | 打开连接池,注册 driver,装配 8 项能力 |
| 关闭 DB | `db.Close()` | 幂等 |
| 健康检查 | `db.PingContext(ctx)` | liveness probe |
| 池状态 | `db.Stats()` | `sql.DBStats` |
| driver 名 | `db.DriverName()` | "postgres" / "mysql" |
| 逃生口 | `db.GORM(ctx)` | 返回 `*gorm.DB`,用于 Preload / 复杂 Where / 原生 SQL |
| AutoMigrate | `db.AutoMigrate(ctx, &Model{})` | dev 环境快速建表,**生产禁用** |
| 列表表 | `db.Tables(ctx)` | 返回当前 schema 的所有表名 |

---

## 5. CRUD API 速查

### 单行查询

```go
// 按条件
var skill Skill
err := db.FindOne(ctx, &skill, "name = ? AND status = ?", name, "active")

// 按主键
err := db.FindOneByPK(ctx, &skill, skillID)

// 多列复合主键
err := db.FindOneByPK(ctx, &version, map[string]any{"skill_name": name, "version": v})
```

`FindOne` 找不到行返回 `dbx.ErrNoRows`。

### 多行查询

```go
var skills []Skill
err := db.FindMany(ctx, &skills, "owner_id = ? AND status = ?", ownerID, "active")
```

### 写操作

```go
// Create(GORM 自动填 created_at / updated_at)
skill := &Skill{Name: "demo", OwnerID: "user_123"}
err := db.Create(ctx, skill)
// skill.ID 自动填充

// Save(全字段更新或插入)
err := db.Save(ctx, skill)

// Update(部分字段,自动填 updated_at)
err := db.Update(ctx, &Skill{}, "name = ?", []any{name}, map[string]any{
    "display_name": "New Name",
    "status":       "archived",
})

// Delete(软删除 if DeletedAt 字段存在,否则硬删除)
err := db.Delete(ctx, &Skill{}, "name = ?", name)
err := db.DeleteByPK(ctx, &Skill{}, skillID)
```

### SafeUpsert(安全 upsert)

```go
// INSERT ... ON CONFLICT DO UPDATE (PG) / INSERT ... ON DUPLICATE KEY UPDATE (MySQL)
// 只有 display_name / status 会被更新;owner_id 永远不被覆盖
err := db.SafeUpsert(ctx, skill, []string{"display_name", "status"})

// 误把 owner_id 列入白名单会直接报错
err := db.SafeUpsert(ctx, skill, []string{"display_name", "owner_id"})
// → dbx.ErrUnsafeUpsert: column "owner_id" is protected
```

### Increment(原子计数)

```go
// UPDATE ... SET download_count = download_count + 1 WHERE ...
err := db.Increment(ctx, &Skill{}, "name = ?", []any{name}, "download_count", 1)
```

### Paginate(分页)

```go
var page []Skill
res, err := db.Paginate(ctx, &page, &Skill{}, "owner_id = ?", []any{ownerID}, 1, 20)
// res.Total = 156
// res.Page = 1, res.Size = 20
// res.HasMore = true
// len(page) = 20
```

### 事务

```go
// 回调式(推荐)
err := db.InTx(ctx, func(tx dbx.Tx) error {
    if err := tx.Create(ctx, skillRow); err != nil { return err }
    if err := tx.Create(ctx, versionRow); err != nil { return err }
    return nil // commit
})
// InTx 自动:fn 返回 nil → commit;返回 error → rollback;panic → rollback + re-panic

// 手动式
tx, err := db.BeginTx(ctx)
defer tx.Rollback() // 幂等,Commit 后变成 no-op
if err := tx.Create(ctx, row); err != nil { return err }
return tx.Commit()
```

---

## 6. driver 注册

```go
import (
    "github.com/aisphereio/kernel/dbx"
    _ "github.com/aisphereio/kernel/dbx/postgres"  // 注册 "postgres"
    // _ "github.com/aisphereio/kernel/dbx/mysql"   // 注册 "mysql"
)
```

| Driver | 包 | 底层 | 错误检测 |
|---|---|---|---|
| postgres | `dbx/postgres` | `gorm.io/driver/postgres` (pgx/stdlib) | `pgconn.PgError.Code`: 23505 / 23503 / 42P01 |
| mysql | `dbx/mysql` | `gorm.io/driver/mysql` (go-sql-driver) | `mysql.MySQLError.Number`: 1062 / 1452 / 1146 |

driver 重复注册会 panic(init 期暴露 bug)。

---

## 7. 错误归一化速查

| dbx 错误 | 触发条件 | 推荐 errorx 构造器 | retryable |
|---|---|---|---:|
| `ErrNoRows` | `FindOne` 无结果 | `errorx.NotFound` | false |
| `ErrDuplicateKey` | 唯一约束冲突 | `errorx.Conflict` | false |
| `ErrTimeout` | context 超时 | `errorx.Timeout` | true |
| `ErrSchemaNotReady` | 表不存在(未跑 migration) | `errorx.Internal` + 提示 | false |
| `ErrForeignKeyViolation` | 外键约束失败 | `errorx.BadRequest` | false |
| `ErrClosed` | DB 已关闭 | `errorx.Unavailable` | true |
| `ErrTxCommitted` | Tx 已 Commit 后操作 | bug,不应出现 | false |
| `ErrTxRolledBack` | Tx 已 Rollback 后操作 | bug,不应出现 | false |
| `ErrUnsafeUpsert` | SafeUpsert 白名单含保护列 | bug,代码错误 | false |
| `ErrNoEffect` | `AssertAffected` 检测 0 行 | `errorx.NotFound` | false |

所有 sentinel 都支持 `errors.Is` 链式匹配,即使被 `fmt.Errorf("%w: ...", err)` 包装。

---

## 8. Context 透传模式

aisphere-hub 的 `db(ctx)` 模式内置成 `DB.GORM(ctx)`:

```go
// 启动期:把全局 DB 装进 repo
type SkillRepo struct{ db dbx.DB }

// 请求期:在 handler 边界注入 tx 到 ctx
func (h *Handler) Create(w, r) {
    ctx := r.Context()
    // 如果需要跨多个 repo 的事务,在 service 层开 tx 并注入:
    err := h.db.InTx(ctx, func(tx dbx.Tx) error {
        // tx 自动通过 InjectTx 注入到 ctx;下游 repo 调 r.db.FindOne(ctx, ...)
        // 会自动用这个 tx,不需要显式传 tx 参数
        return h.skillRepo.CreateWithCtx(ctx, skill)
    })
}

// repo 实现:不需要自己写 db(ctx) helper
func (r *SkillRepo) Find(ctx context.Context, name string) (*Skill, error) {
    var skill Skill
    // r.db.FindOne(ctx, ...) 内部调 r.db.GORM(ctx),
    // GORM(ctx) 自动优先用 ctx 里的 tx,否则用全局 DB
    err := r.db.FindOne(ctx, &skill, "name = ?", name)
    // ...
}
```

---

## 9. Soft Delete 安全模式

模型有 `gorm.DeletedAt` 字段时,dbx 自动启用 soft delete:

```go
type Skill struct {
    // ...
    DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

// 普通查询:自动过滤已删除行
db.FindOne(ctx, &skill, "name = ?", name)  // 不会返回已删除的 skill

// 显式查已删除行(必须 opt-in)
unscopedCtx := dbx.WithUnscoped(ctx)
db.FindOne(unscopedCtx, &skill, "name = ?", name)  // 会返回已删除的 skill

// Delete 默认是软删除
db.Delete(ctx, &Skill{}, "name = ?", name)  // UPDATE ... SET deleted_at = NOW()

// 硬删除:用 GORM 逃生口
db.GORM(ctx).Unscoped().Delete(&Skill{}, "name = ?", name)
```

---

## 10. 配置推荐

### 10.1 连接池大小

| 工作负载 | MaxOpenConns | MaxIdleConns |
|---|---:|---:|
| 小服务 / 低 QPS | 8 | 4 |
| 中等流量 | 16-32 | 8-16 |
| 高流量 / 短查询 | 32-64 | 16-32 |
| 高流量 / 长查询 | 64-128 | 32-64 |

经验法则:
- `MaxIdleConns >= MaxOpenConns / 2`
- `ConnMaxLifetime` 15-30 min(让 PG 故障切换有机会重新路由)
- `ConnMaxIdleTime` 2-5 min(避免空闲连接占 PG `max_connections`)
- `QueryTimeout` 5-10 秒
- `SlowQueryThreshold` 200ms(生产)/ 50ms(debug)

### 10.2 Postgres DSN

```text
postgres://user:pass@host:5432/dbname?sslmode=disable&connect_timeout=5
postgresql://user:pass@host:5432/dbname?sslmode=require
host=localhost port=5432 user=user password=pass dbname=app sslmode=disable
```

### 10.3 MySQL DSN

```text
user:pass@tcp(host:3306)/dbname?parseTime=true&loc=Local&charset=utf8mb4
```

`parseTime=true` **必须**,否则 `time.Time` 字段扫描失败。

### 10.4 完整 YAML 配置示例

```yaml
database:
  driver: postgres
  dsn: postgres://user:pass@localhost:5432/app?sslmode=disable
  max_open_conns: 32
  max_idle_conns: 8
  conn_max_lifetime_ns: 1800000000000   # 30 min
  conn_max_idle_time_ns: 300000000000   # 5 min
  query_timeout_ns: 5000000000          # 5 s
  slow_query_threshold_ns: 200000000    # 200 ms
  audit_enabled: false                  # opt-in
  metrics_enabled: false                # opt-in
  dry_run: false
  debug: false
```

---

## 11. AutoMigrate(dev only)

```go
// dev 环境:快速建表
if err := db.AutoMigrate(ctx, &Skill{}, &SkillVersion{}, &SkillFile{}); err != nil {
    log.Fatal(err)
}

// 生产环境:用 SQL migration(golang-migrate / goose)
// dbx 不管 schema;aisphere-hub 风格 migrations/postgres/*.sql
```

`AutoMigrate` 是幂等的,重复调用不会破坏数据。但它**不**:
- 删除列或表
- 修改列类型(只新增)
- 创建外键约束(`DisableForeignKeyConstraintWhenMigrating: true`)

生产环境永远用 SQL migration。`AutoMigrate` 只用于 dev 快速迭代和测试。

---

## 12. 禁止模式

```go
// ❌ 业务代码直接 import database/sql
import "database/sql"

// ❌ 业务代码直接 import GORM
import "gorm.io/gorm"
db, _ := gorm.Open(...)

// ❌ 业务代码直接 import driver
import _ "github.com/jackc/pgx/v5/stdlib"
import _ "github.com/go-sql-driver/mysql"

// ❌ 业务代码用 sqlx
import "github.com/jmoiron/sqlx"

// ❌ 在 handler / service 写 SQL
func (h *Handler) ServeHTTP(w, r) {
    h.db.GORM(r.Context()).Exec("INSERT INTO skills ...")  // SQL 漏到 handler
}

// ❌ 吞掉 ErrNoRows
err := db.FindOne(ctx, &skill, q, id)
if err != nil { return nil, err }  // ErrNoRows 也被当 error 上抛

// ❌ 忘记 defer Close
db, _ := dbx.New(cfg)
// 没有 defer db.Close()

// ❌ Unscoped 没用 WithUnscoped
db.GORM(ctx).Unscoped().Find(&skills)  // 绕过安全门
```

替代:

```go
// ✅ 用 dbx.New
db, _ := dbx.New(dbx.Config{Driver: "postgres", DSN: dsn})
defer db.Close()

// ✅ SQL 只在 repository
func (r *SkillRepo) Find(ctx context.Context, id string) (*Skill, error) {
    var s Skill
    err := r.db.FindOne(ctx, &s, "id = ?", id)
    // ...
}

// ✅ ErrNoRows 单独处理
if errors.Is(err, dbx.ErrNoRows) { return nil, errorx.NotFound(...) }

// ✅ Unscoped 显式 opt-in
db.FindOne(dbx.WithUnscoped(ctx), &skill, "name = ?", name)
```

---

## 13. 文档地图

```text
快速上手
├── 本文件 (dbx/README.md)                  ← 单一入口
├── dbx/doc.go                              ← go doc 输出源
├── dbx/example_test.go                     ← Go 标准示例
└── dbx/example_business_test.go            ← 业务场景示例

深度规范
├── docs/design/dbx.md                      ← 设计规范
└── docs/contracts/dbx.md                   ← 不可破坏契约

AI 编码指南
├── docs/ai/dbx.md                          ← AI 编码指南
└── AGENTS.md                               ← 项目级 AI 规则

验收与运维
└── docs/process/dbx-acceptance-checklist.md

可运行示例
└── examples/dbx-basic/                     ← aisphere-hub skill 场景示例
```

---

## 14. 发版前检查

```bash
# 单元测试(不需要 DB)
go test ./dbx -count=1 -short

# 集成测试(需要 Docker for testcontainers,或设 KERNEL_DBX_PG_DSN / KERNEL_DBX_MYSQL_DSN)
go test ./dbx -count=1

# 性能基准
go test ./dbx -bench=. -benchmem

# go vet
go vet ./dbx/...

# 禁止模式扫描
./scripts/check-dbx-usage.sh
```

---

## 15. 设计哲学一句话

> Kernel 开箱即用。
> dbx 把 aisphere-hub 里 100+ 处重复样板收敛成一套类型安全 API。
> 业务代码拿过去就能跑,而且用得特别好。
