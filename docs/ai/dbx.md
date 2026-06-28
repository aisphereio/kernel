# dbx — AI 编码指南

> AI 写 Aisphere Kernel 业务代码时的数据库访问规范。**只看本文件即可写对所有数据库场景。**

---

## 0. 一句话规则

> 业务代码(handler/service/repository/worker)访问数据库时,**必须**使用 `github.com/aisphereio/kernel/dbx`。
> **禁止**直接 import `database/sql` / `gorm.io/gorm` / `gorm.io/driver/*` / `pgx` / `lib/pq` / `go-sql-driver/mysql` / `sqlx`。

dbx 在 GORM 之上封装了 8 项开箱即用的"预先动作",把你之前在 aisphere-hub 里重复写的 100+ 处样板全部内置。

---

## 1. 速查:什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| 单行 → struct | `db.FindOne(ctx, &dest, query, args...)` | 推荐 |
| 单行按主键 | `db.FindOneByPK(ctx, &dest, pk)` | 主键查询 |
| 多行 → []struct | `db.FindMany(ctx, &dest, query, args...)` | 列表查询 |
| 计数 | `db.Count(ctx, &model, query, args...)` | COUNT |
| 创建 | `db.Create(ctx, &row)` | INSERT(自动填 created_at/updated_at) |
| 全字段保存 | `db.Save(ctx, &row)` | INSERT 或 UPDATE by PK |
| 部分更新 | `db.Update(ctx, &model, query, args, columns)` | UPDATE 指定列 |
| 原子更新 | `db.UpdateColumns(ctx, &model, query, args, columns)` | 跳过 hook 的 UPDATE |
| 删除 | `db.Delete(ctx, &model, query, args...)` | 软删除(if DeletedAt)或硬删除 |
| 按主键删除 | `db.DeleteByPK(ctx, &model, pk)` | 同上 |
| 安全 upsert | `db.SafeUpsert(ctx, &row, allowedColumns)` | ON CONFLICT + 白名单 |
| 原子计数 | `db.Increment(ctx, &model, query, args, col, delta)` | count = count + delta |
| 分页 | `db.Paginate(ctx, &out, &model, query, args, page, size)` | limit+1 + hasMore + total |
| 事务 | `db.InTx(ctx, func(tx dbx.Tx) error {...})` | 回调式(推荐) |
| 手动事务 | `db.BeginTx(ctx)` | escape hatch |
| 健康检查 | `db.PingContext(ctx)` | liveness probe |
| GORM 逃生口 | `db.GORM(ctx)` | Preload / 复杂 Where / 原生 SQL |
| AutoMigrate | `db.AutoMigrate(ctx, &Model{})` | dev only,生产禁用 |

**关键原则**:SQL 只在 repository 层;handler/service 只调用 repo 方法。

---

## 2. 标准食谱(10 个场景,复制即用)

### 2.1 启动期打开 DB

```go
package main

import (
    "log"
    "time"

    "github.com/aisphereio/kernel/configx"
    "github.com/aisphereio/kernel/configx/file"
    "github.com/aisphereio/kernel/dbx"
    _ "github.com/aisphereio/kernel/dbx/postgres"
)

func main() {
    cfg := configx.New(configx.WithSource(file.NewSource("configs/app.yaml")))
    defer cfg.Close()
    if err := cfg.Load(); err != nil { log.Fatal(err) }

    var dbCfg dbx.Config
    if err := cfg.Value("database").Scan(&dbCfg); err != nil { log.Fatal(err) }

    db, err := dbx.New(dbCfg)
    if err != nil { log.Fatal(err) }
    defer db.Close()
    // ... pass db to repositories
}
```

### 2.2 Repository 单行查询 + 错误转换

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
            errorx.WithPublicMetadata("resource", "skill"),
        )
    }
    if err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_QUERY_FAILED",
            errorx.WithMessage("查询技能失败"),
            errorx.WithRetryable(true),
            errorx.WithMetadata("skill_id", name),
        )
    }
    return &skill, nil
}
```

### 2.3 Repository 多行动态条件查询

aisphere-hub ListSkills 的 if-else 拼 Where 模式,GORM 链式 API 完美支持:

```go
func (r *SkillRepo) List(ctx context.Context, opts SkillListOptions) ([]*Skill, error) {
    tx := r.db.GORM(ctx).Model(&Skill{})
    if q := strings.TrimSpace(opts.Query); q != "" {
        like := "%" + q + "%"
        tx = tx.Where("name ILIKE ? OR display_name ILIKE ?", like, like)
    }
    if status := opts.Status; status != "" {
        tx = tx.Where("status = ?", status)
    }
    if opts.OwnerID != "" {
        tx = tx.Where("owner_id = ?", opts.OwnerID)
    }

    var skills []Skill
    if err := tx.Order("created_at DESC").Limit(opts.Limit).Find(&skills).Error; err != nil {
        return nil, errorx.Wrap(err, "AIHUB_SKILL_LIST_FAILED",
            errorx.WithRetryable(true),
        )
    }
    return convertSkills(skills), nil
}
```

### 2.4 Repository Create + Duplicate Key 检测

```go
func (r *SkillRepo) Create(ctx context.Context, skill *Skill) error {
    err := r.db.Create(ctx, skill)
    if errors.Is(err, dbx.ErrDuplicateKey) {
        return errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在",
            errorx.WithPublicMetadata("name", skill.Name),
        )
    }
    if err != nil {
        return errorx.Wrap(err, "AIHUB_SKILL_CREATE_FAILED",
            errorx.WithMessage("创建技能失败"),
            errorx.WithRetryable(true),
            errorx.WithMetadata("skill_id", skill.ID),
        )
    }
    return nil
}
```

不用再写 `isUniqueViolation(err)` helper — dbx 自动归一化。

### 2.5 SafeUpsert(安全 upsert,白名单保护)

aisphere-hub skill.go 里 `SECURITY: OnConflict DoUpdates MUST NOT touch owner_id/visibility/status` 的注释,在 dbx 里变成类型安全的 API:

```go
func (r *SkillRepo) Upsert(ctx context.Context, skill *Skill) error {
    // 只有 display_name / description / version / source_* / manifest / tags 可以更新
    // owner_id / created_at / deleted_at 永远不被覆盖(即使误列入白名单也会报错)
    err := r.db.SafeUpsert(ctx, skill, []string{
        "display_name",
        "description",
        "version",
        "source_type",
        "source_uri",
        "manifest_json",
        "tags",
    })
    if errors.Is(err, dbx.ErrDuplicateKey) {
        // 不该发生(SafeUpsert 本就是处理冲突的),但保险起见
        return errorx.Conflict("AIHUB_SKILL_ALREADY_EXISTS", "技能已存在")
    }
    if err != nil {
        return errorx.Wrap(err, "AIHUB_SKILL_UPSERT_FAILED",
            errorx.WithRetryable(true),
        )
    }
    return nil
}
```

### 2.6 Increment(原子计数,替代手写 gorm.Expr)

```go
func (r *SkillRepo) BumpDownloadCount(ctx context.Context, name string) error {
    err := r.db.Increment(ctx, &SkillVersion{}, "skill_name = ? AND version = ?",
        []any{name, "1.0.0"}, "download_count", 1)
    if err != nil {
        return errorx.Wrap(err, "AIHUB_SKILL_VERSION_INCREMENT_FAILED",
            errorx.WithRetryable(true),
        )
    }
    return nil
}
```

不用手写 `UpdateColumn("count", gorm.Expr("count + ?", delta))`。

### 2.7 事务(回调式,自动 commit/rollback)

```go
func (r *SkillRepo) Transfer(ctx context.Context, fromID, toID string, amount int) error {
    return r.db.InTx(ctx, func(tx dbx.Tx) error {
        if err := tx.UpdateColumns(ctx, &Account{}, "id = ?", []any{fromID},
            map[string]any{"balance": gorm.Expr("balance - ?", amount)}); err != nil {
            return errorx.Wrap(err, "AIHUB_ACCOUNT_DEBIT_FAILED",
                errorx.WithRetryable(true),
            )
        }
        if err := tx.UpdateColumns(ctx, &Account{}, "id = ?", []any{toID},
            map[string]any{"balance": gorm.Expr("balance + ?", amount)}); err != nil {
            return errorx.Wrap(err, "AIHUB_ACCOUNT_CREDIT_FAILED",
                errorx.WithRetryable(true),
            )
        }
        return nil // commit
    })
}
```

InTx 自动:
- fn 返回 nil → commit
- fn 返回 error → rollback 后返回该 error
- fn panic → rollback 后 re-panic

### 2.8 事务 + 多 repo 共享 tx(Context 透传)

aisphere-hub 的 `db(ctx)` 模式内置成 `DB.GORM(ctx)`:

```go
type Service struct {
    db        dbx.DB
    skillRepo *SkillRepo
    auditRepo *AuditRepo
}

func (s *Service) CreateWithAudit(ctx context.Context, skill *Skill) error {
    // InTx 自动把 tx 注入到 ctx;下游 repo 调 r.db.FindOne(ctx, ...)
    // 会自动用这个 tx,不需要显式传 tx 参数
    return s.db.InTx(ctx, func(tx dbx.Tx) error {
        if err := s.skillRepo.CreateWithCtx(ctx, skill); err != nil {
            return err
        }
        if err := s.auditRepo.RecordWithCtx(ctx, &AuditEvent{
            Action: "skill.create",
        }); err != nil {
            return err
        }
        return nil
    })
}

// repo 实现:不需要自己写 db(ctx) helper,也不需要 WithTx 版本
func (r *SkillRepo) CreateWithCtx(ctx context.Context, skill *Skill) error {
    // r.db.Create(ctx, ...) 内部调 r.db.GORM(ctx),
    // GORM(ctx) 自动优先用 ctx 里的 tx(由 InTx 注入),否则用全局 DB
    return r.db.Create(ctx, skill)
}
```

### 2.9 Paginate(分页,替代手写 limit+1 + hasMore)

```go
func (r *SkillRepo) ListPage(ctx context.Context, ownerID string, page, size int) ([]*Skill, int64, bool, error) {
    var skills []Skill
    res, err := r.db.Paginate(ctx, &skills, &Skill{},
        "owner_id = ?", []any{ownerID}, page, size)
    if err != nil {
        return nil, 0, false, errorx.Wrap(err, "AIHUB_SKILL_LIST_FAILED",
            errorx.WithRetryable(true),
        )
    }
    return convertSkills(skills), res.Total, res.HasMore, nil
}
```

不用手写 `limit+1` + `hasMore` + `NextOffset` 计算。

### 2.10 Soft Delete + Unscoped(安全查已删除)

```go
// 普通查询:自动过滤已删除行(DeletedAt 字段存在时)
var skill Skill
err := db.FindOne(ctx, &skill, "name = ?", name)
// 如果 skill 已被软删除,返回 ErrNoRows

// 显式查已删除行(必须 opt-in,防止误用)
var deleted Skill
err = db.FindOne(dbx.WithUnscoped(ctx), &deleted, "name = ?", name)
// 会返回已删除的 skill

// Delete 默认是软删除
err = db.Delete(ctx, &Skill{}, "name = ?", name)
// → UPDATE skills SET deleted_at = NOW() WHERE name = ? AND deleted_at IS NULL
```

---

## 3. SQL 命名规则

### 3.1 表名

格式:`{module}_{entity}` 或 `{module}_{entity}_{subentity}`,全小写蛇形。

```text
✅ aihub_skills
✅ aihub_skill_versions
✅ iam_users
❌ Skills / skill-versions / skills
```

### 3.2 列名

全小写蛇形:`id / name / created_at / updated_at / owner_id / deleted_at`。

### 3.3 GORM tag

```go
type Skill struct {
    ID          int64          `gorm:"primaryKey;autoIncrement;column:id"`
    Name        string         `gorm:"column:name;size:128;uniqueIndex;not null"`
    DisplayName string         `gorm:"column:display_name;size:256;not null;default:''"`
    OwnerID     string         `gorm:"column:owner_id;size:128;not null;default:''"`
    CreatedAt   time.Time      `gorm:"column:created_at;not null;autoCreateTime"`
    UpdatedAt   time.Time      `gorm:"column:updated_at;not null;autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}
```

关键 tag:
- `column:xxx` — 列名(必须,避免依赖 GORM 默认蛇形转换)
- `primaryKey` — 主键
- `autoIncrement` — 自增
- `uniqueIndex` — 唯一索引
- `not null` / `default:'xxx'` — 约束
- `autoCreateTime` / `autoUpdateTime` — 自动时间戳(业务**不要**手填)
- `index` — 普通索引
- `type:jsonb` / `type:text` — 显式类型

---

## 4. 禁止模式

### 4.1 业务代码禁止

```go
// ❌ 直接 import database/sql
import "database/sql"

// ❌ 直接 import GORM
import "gorm.io/gorm"
db, _ := gorm.Open(...)

// ❌ 直接 import driver
import _ "github.com/jackc/pgx/v5/stdlib"
import _ "github.com/go-sql-driver/mysql"

// ❌ 用 sqlx
import "github.com/jmoiron/sqlx"

// ❌ 在 handler / service 写 SQL
func (h *Handler) Create(w, r) {
    h.db.GORM(r.Context()).Exec("INSERT INTO skills ...")
}

// ❌ 吞掉 ErrNoRows
err := db.FindOne(ctx, &s, q, id)
if err != nil { return nil, err }  // ErrNoRows 也被当 error 上抛

// ❌ Unscoped 没用 WithUnscoped
db.GORM(ctx).Unscoped().Find(&skills)  // 绕过安全门

// ❌ SafeUpsert 把保护列入白名单
db.SafeUpsert(ctx, row, []string{"display_name", "owner_id"})  // 会报 ErrUnsafeUpsert

// ❌ 手填 updated_at
db.Update(ctx, &Skill{}, ..., map[string]any{"updated_at": time.Now(), ...})
// GORM autoUpdateTime 自动处理,不要手填

// ❌ 忘记 defer Close
db, _ := dbx.New(cfg)
// 没有 defer db.Close()
```

### 4.2 替代写法

```go
// ✅ 用 dbx
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

// ✅ SafeUpsert 白名单不含保护列
db.SafeUpsert(ctx, row, []string{"display_name", "status"})

// ✅ autoUpdateTime 自动处理,不手填
db.Update(ctx, &Skill{}, "name = ?", []any{name}, map[string]any{
    "display_name": "New Name",
    "status":       "archived",
    // 不填 updated_at
})
```

### 4.3 允许的例外

测试代码可以用 `gorm.Open(postgres.Open(dsn), &gorm.Config{})` 在集成测试里建独立 GORM 实例,但生产代码不允许。

---

## 5. Driver 注册契约

main 包必须显式导入一次:

```go
import (
    "github.com/aisphereio/kernel/dbx"
    _ "github.com/aisphereio/kernel/dbx/postgres"  // 注册 "postgres"
    // _ "github.com/aisphereio/kernel/dbx/mysql"   // 注册 "mysql"
)
```

注册后:
- `dbx.New(Config{Driver: "postgres", ...})` 能成功打开连接
- `dbx.ErrDuplicateKey` / `ErrSchemaNotReady` / `ErrForeignKeyViolation` 能被 `errors.Is` 检测到

Driver 重复注册会 panic。

---

## 6. 错误转换速查

| dbx 错误 | 推荐 errorx 构造器 | retryable |
|---|---|---:|
| `ErrNoRows` | `errorx.NotFound` | false |
| `ErrDuplicateKey` | `errorx.Conflict` | false |
| `ErrTimeout` | `errorx.Timeout` | true |
| `ErrSchemaNotReady` | `errorx.Internal` + 提示"运行 migration" | false |
| `ErrForeignKeyViolation` | `errorx.BadRequest` | false |
| `ErrClosed` | `errorx.Unavailable` | true |
| `ErrNoEffect`(AssertAffected) | `errorx.NotFound` | false |
| 其他 driver 错误 | `errorx.Wrap(err, ...)` | true(保守默认) |

`ErrTxCommitted` / `ErrTxRolledBack` / `ErrUnsafeUpsert` 一般是代码 bug,不应在生产路径出现,应 fail-fast。

---

## 7. 完整 handler 示例

```go
package handler

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"

    "github.com/aisphereio/kernel/dbx"
    "github.com/aisphereio/kernel/errorx"
    "github.com/aisphereio/kernel/logx"
)

type SkillHandler struct {
    logger logx.Logger
    repo   SkillRepo
}

func (h *SkillHandler) Get(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := r.URL.Query().Get("id")
    if id == "" {
        writeError(w, errorx.BadRequest("AIHUB_SKILL_ID_REQUIRED", "skill id required"))
        return
    }

    skill, err := h.repo.Find(ctx, id)
    if errors.Is(err, dbx.ErrNoRows) {
        writeError(w, errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
            errorx.WithPublicMetadata("resource", "skill"),
        ))
        return
    }
    if err != nil {
        writeError(w, err) // repo 已经返回 errorx
        return
    }

    writeJSON(w, http.StatusOK, skill)
}
```

Repository 实现:

```go
type SkillRepo interface {
    Find(ctx context.Context, id string) (*Skill, error)
}

type skillRepo struct{ db dbx.DB }

func NewSkillRepo(db dbx.DB) SkillRepo {
    return &skillRepo{db: db}
}

func (r *skillRepo) Find(ctx context.Context, name string) (*Skill, error) {
    var s Skill
    err := r.db.FindOne(ctx, &s, "name = ?", name)
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
    return &s, nil
}
```

---

## 8. 验收清单(写完 dbx 代码后自检)

- [ ] 所有 `database/sql` / `gorm.io/gorm` / driver import 都换成 `dbx`
- [ ] SQL 只在 repository 层
- [ ] `defer db.Close()` 已添加
- [ ] `ErrNoRows` 单独处理,不当作 error 上抛
- [ ] `ErrDuplicateKey` 转换为 `errorx.Conflict`
- [ ] `ErrTimeout` 转换为 `errorx.Timeout` 并 `WithRetryable(true)`
- [ ] SafeUpsert 白名单不含 `owner_id` / `created_at` / `deleted_at`
- [ ] Unscoped 必须用 `dbx.WithUnscoped(ctx)`
- [ ] 事务用 `InTx` 回调式(推荐)
- [ ] 多 repo 共享 tx 时不用显式传 tx,靠 ctx 透传
- [ ] Struct 字段都有 `gorm` tag,`column` 必填
- [ ] `autoCreateTime` / `autoUpdateTime` 启用,业务不手填时间戳
- [ ] NULL 字段用 `*T` 或 `sql.NullXxx`
- [ ] 测试代码 `go test ./... -short` 通过(不需要 DB)
- [ ] 集成测试 `go test ./...` 通过(需要 Docker 或 DSN 环境变量)
- [ ] `go vet ./...` 通过
- [ ] `./scripts/check-dbx-usage.sh` 通过

---

## 9. 相关文档

- `dbx/README.md` — 单一入口用户指南
- `dbx/doc.go` — `go doc dbx` 输出源
- `dbx/contract_test.go` — 不可破坏契约测试
- `dbx/pg_integration_test.go` — 真实 PG 集成测试(testcontainers)
- `dbx/mysql_integration_test.go` — 真实 MySQL 集成测试(testcontainers)
- `docs/design/dbx.md` — 完整设计规范
- `docs/contracts/dbx.md` — 不可破坏契约
- `docs/process/dbx-acceptance-checklist.md` — CI 验收清单
- `examples/dbx-basic/` — aisphere-hub skill 场景示例
