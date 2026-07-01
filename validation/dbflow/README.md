# DB Flow 验证

这个目录验证 Kernel 的数据开发范式：

1. SQL migration 放在 `migrations/`，SQL 是数据库结构的唯一真实来源。
2. `migrationx` 负责发现、校验、执行 goose-compatible SQL。
3. `dbrepo.ResourceRepository[T]` 提供 tenant/owner/soft-delete/timestamp 约定下的傻瓜式 CRUD。
4. 业务 handler 不直接 `sql.Open`、不直接 `gorm.Open`，优先使用 generated/hand-written repository。

当前 P0 不把 SQL 放进 proto。proto 只负责 API + access policy。
