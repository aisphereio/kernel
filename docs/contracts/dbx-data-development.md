# Kernel 数据开发范式

## 结论

Kernel 不把 SQL 写进 proto。数据库结构以 SQL migration 为唯一真实来源；`dbx` 负责连接池和 GORM 执行层，`migrationx` 负责自动执行/校验 migration，`dbrepo` 负责提供 AI 友好的资源仓库。

生产默认 migration 引擎是 **真实 goose provider**。Kernel 不自行解析复杂 SQL，不按分号拆分生产 migration，因此 PostgreSQL function、trigger、procedural block 等语义交给成熟 migration 引擎处理。

## 分层

```text
dbx
  GORM 底座、连接池、事务、错误归一化、PG/MySQL driver

migrationx
  读取 migrations/，默认委托 pressly/goose 执行或校验 SQL migration

dbrepo
  ResourceRepository[T]，封装 tenant、owner、soft delete、分页、过滤、排序和 patch 白名单

serverx.BuildServiceFromFactory
  根据配置自动打开 DB、执行/校验 migration、构造 Data，并通过 ServiceDeps 注入业务实例
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
```

`engine: goose` 是默认生产路径。`engine: kernel_sql` 只为历史兼容和确定性单测保留，不用于复杂生产 SQL。

### Migration mode

- `disabled`：不处理 schema。
- `gorm_dev_auto`：只用于 dev/test 的 GORM AutoMigrate 路径。
- `dev_apply`：开发环境启动时自动执行 goose migration。
- `apply`：启动时执行 migration；当前只允许单副本服务。
- `validate`：不改 schema，只检查是否存在 pending migration；生产服务推荐。

在 Kernel 提供明确的跨 PG/MySQL migration locker 前，`apply/dev_apply + replicas > 1` 必须 fail-closed。生产多副本推荐把 migration 放到独立 Job/发布阶段，服务本身使用 `validate`。

## Migration 文件

推荐使用 goose SQL 文件：

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS skills (...);

-- +goose Down
DROP TABLE IF EXISTS skills;
```

复杂函数/触发器按 goose 的标准 SQL migration 语法表达；Kernel 不重写其 SQL parser。

## Repository 用法

```go
skills := dbrepo.MustResourceRepository[SkillRow](db, dbrepo.ResourceConfig{
    Resource:           "skill",
    Table:              "skills",
    TenantScoped:       true,
    OwnerScoped:        true,
    SoftDelete:         true,
    Timestamps:         true,
    AllowedFilters:     []string{"status", "owner_id"},
    AllowedSorts:       []string{"created_at", "updated_at", "name"},
    AllowedPatchFields: []string{"display_name", "status"},
})

row, err := skills.Get(ctx, id)
page, err := skills.List(ctx, dbrepo.Query{Page: 1, Size: 20, Filters: map[string]any{"status": "active"}})
err = skills.Create(ctx, row)
err = skills.Patch(ctx, id, map[string]any{"display_name": "Demo"})
err = skills.Delete(ctx, id)
```

## Service 自动注入

`RegisterData` 构造 repository/data bundle，`ServiceFactory` 从 `ServiceDeps.Data` 获得它：

```go
module := skillv1.SkillServiceKernelModule()
module.RegisterData = func(db dbx.DB) (any, error) {
    return NewSkillData(db)
}

app, err := serverx.BuildServiceFromFactory(ctx, cfg, module,
    func(ctx context.Context, deps serverx.ServiceDeps) (any, error) {
        data := deps.Data.(*SkillData)
        return NewSkillService(data), nil
    },
)
```

AI 不应在 handler 里重新打开 DB、构造 GORM connection 或执行 migration。

## 硬规则

- 生产 schema 变更必须走 SQL migration；SQL 是数据库结构唯一真实来源。
- GORM AutoMigrate 只允许 dev/test 快速验证。
- 业务不得直接 `sql.Open` / `gorm.Open`。
- `dbrepo` 通过 `dbx.NormalizeGORMError` 归一化底层 GORM/driver 错误，业务层不依赖 GORM error。
- `TenantScoped=true` 时，`Get/List/Patch/Delete/Create` 缺 tenant 必须返回 `ErrTenantRequired`，禁止退化为全表访问。
- `OwnerScoped=true` 时，缺 subject 返回 `ErrOwnerRequired`。
- `Patch` 默认禁止修改 `id`、`tenant_id`、`owner_id`、`created_at`、`deleted_at`；业务应显式配置 `AllowedPatchFields`。
- 复杂查询优先沉淀专用 repository 方法；RawSQL 是 escape hatch，不是普通 CRUD 默认路径。
- 多副本服务不得在启动阶段无锁执行 migration apply；当前框架强制 fail-closed。
