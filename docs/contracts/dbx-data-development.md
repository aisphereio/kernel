# Kernel 数据开发范式

## 结论

Kernel 不把 SQL 写进 proto。数据库结构以 SQL migration 为唯一真实来源；`dbx` 负责连接池和 GORM 执行层，`migrationx` 负责自动执行/校验 migration，`dbrepo` 负责提供 AI 友好的资源仓库。

## 分层

```text
dbx
  GORM 底座、连接池、事务、错误归一化、PG/MySQL driver

migrationx
  发现 migrations/ SQL，执行/校验 goose-compatible migration

dbrepo
  ResourceRepository[T]，封装 tenant、owner、soft delete、分页、过滤、排序白名单

serverx.BuildService
  根据配置自动打开 DB、执行/校验 migration、关闭 DB
```

## 配置

```yaml
database:
  enabled: true
  driver: postgres
  dsn: ${DATABASE_DSN}
  auto_create_database: false
  max_open_conns: 20
  max_idle_conns: 10
  query_timeout: "2s"
  slow_query_threshold: "200ms"
  migration:
    enabled: true
    engine: goose
    dir: ./migrations
    table: kernel_schema_migrations
    mode: dev_apply # disabled / validate / dev_apply / apply / gorm_dev_auto
    fail_on_pending: true
    # 多副本启动时不允许 apply/dev_apply，除非你明确有迁移锁或外部迁移 job。
    allow_concurrent: false
```

## Migration 文件

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS skills (...);

-- +goose Down
DROP TABLE IF EXISTS skills;
```

也支持 split 文件：

```text
000001_create_skills.up.sql
000001_create_skills.down.sql
```

## Repository 用法

```go
skills := dbrepo.MustResourceRepository[SkillRow](db, dbrepo.ResourceConfig{
    Resource:       "skill",
    Table:          "skills",
    TenantScoped:   true,
    OwnerScoped:    true,
    SoftDelete:     true,
    Timestamps:     true,
    AllowedFilters: []string{"status", "owner_id"},
    AllowedSorts:   []string{"created_at", "updated_at", "name"},
})

row, err := skills.Get(ctx, id)
page, err := skills.List(ctx, dbrepo.Query{Page: 1, Size: 20, Filters: map[string]any{"status": "active"}})
err = skills.Create(ctx, row)
err = skills.Patch(ctx, id, map[string]any{"display_name": "Demo"})
err = skills.Delete(ctx, id)
```

## 规则

- 生产 schema 变更必须走 SQL migration。
- GORM AutoMigrate 只允许 dev/test 快速验证。
- 业务不得直接 `sql.Open` / `gorm.Open`。
- 多租户表必须通过 `dbrepo.ResourceRepository` 或 generated repo 自动注入 tenant 过滤；缺少 tenant_id 时必须 fail-closed，不能放行全表查询。
- 复杂 SQL 可以保留为 migration 或 dbx RawSQL escape hatch，但必须经过 review。


## 当前 P0 安全约束

- `TenantScoped=true` 时，`Get/List/Patch/Delete` 如果上下文没有 tenant，直接返回 `ErrTenantRequired`。
- `OwnerScoped=true` 时，缺少 subject 会返回 `ErrOwnerRequired`。
- `Patch` 默认禁止修改 `id`、`tenant_id`、`owner_id`、`created_at`、`deleted_at`。
- `AllowedPatchFields` 非空时，Patch 只能修改白名单字段；`BlockedPatchFields` 始终拒绝。
- `-- +goose StatementBegin/StatementEnd` 可包裹包含分号的函数/触发器 SQL；生产后续仍建议接真实 goose library。
