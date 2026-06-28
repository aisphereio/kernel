# Aisphere Kernel Docs

Aisphere Kernel 文档导航。文档按"角色 × 场景"组织，按需查阅，无需通读。

---

## 文档地图

### 快速上手（所有人先看）

| 文档 | 内容 | 何时看 |
|---|---|---|
| `../README.md` | 项目总览 + 启动命令 | 第一次接触项目 |
| `../AGENTS.md` | AI 工作规则 | AI 协作开发前 |
| `../errorx/README.md` | errorx 单一入口指南 | 写任何业务代码前 |
| `../logx/README.md` | logx 单一入口指南 | 写任何日志前 |
| `../configx/README.md` | configx 单一入口指南 | 写任何配置前 |

### 深度规范（架构师 / PR review / 设计新模块时看）

| 文档 | 内容 | 行数 |
|---|---|---:|
| `design/errorx.md` | errorx 完整设计规范（26 章） | ~1250 |
| `design/logx.md` | logx 设计规范 | — |
| `design/configx.md` | configx 设计规范 | — |
| `design/kratos-v2.md` | Kratos v2 升级设计 | — |
| `contracts/errorx.md` | errorx 不可破坏契约 + 验收命令 | ~140 |

### AI 编码指南（AI 写业务代码时看）

| 文档 | 内容 |
|---|---|
| `ai/errorx.md` | errorx 完整 AI 指南：速查 + 食谱 + 禁用模式 + 完整 handler 示例 |
| `ai/configx.md` | configx 完整 AI 指南：Source + Scan + Watch + 禁用模式 |
| `ai/metricsx.md` | metricsx 与 kernel.PrometheusMetrics / kernel.Metrics 启动指南 |
| `ai/dtmx.md` | dtmx 分布式事务抽象、DTM Saga、branch header 认证边界 |

> 该文档合并了原 `ai/00-quickstart.md` + `ai/recipes/errorx.md` + `ai/99-forbidden-patterns.md`，
> AI 拿到这一个文件即可写对所有 errorx 场景。

### 验收与运维（CI/CD / 发版前看）

| 文档 | 内容 |
|---|---|
| `process/errorx-acceptance-checklist.md` | errorx 静态/单元/集成验收清单 |
| `process/configx-acceptance-checklist.md` | configx 静态/单元/集成验收清单 |
| `process/errorx-test-report.md` | errorx 测试报告 |
| `process/module-acceptance.md` | 通用模块验收流程 |
| `process/windows-script-policy.md` | Windows 脚本执行策略说明 |

### 迁移与历史（升级时看）

| 文档 | 内容 |
|---|---|
| `migration/v2-to-v3.md` | Kratos v2 → v3 迁移指南 |
| `migration/v2-to-v3_zh.md` | 中文版 |
| `migration/source-adoption-baseline.md` | 源码采纳基线 |

### 上游参考（只读）

| 文档 | 内容 |
|---|---|
| `upstream/KRATOS_README.md` | Kratos 上游 README |
| `upstream/KRATOS_README_zh.md` | 中文版 |
| `upstream/KRATOS_Makefile` | Kratos 上游 Makefile 参考 |

---

## 推荐阅读路径

### 路径 A：新开发者（1 小时上手）

```text
1. ../README.md                          ← 了解项目
2. ../AGENTS.md                          ← 了解 AI 规则
3. ../errorx/README.md                   ← 学会写错误
4. ../logx/README.md                     ← 学会写日志
5. ../configx/README.md                  ← 学会写配置
6. docs/ai/metricsx.md                  ← 学会接入 metrics
7. docs/ai/dtmx.md                      ← 需要分布式事务时阅读
8. 跑通 examples/errorx-basic            ← 第一个示例
9. 跑通 examples/errorx-http             ← HTTP 示例
```

### 路径 B：AI 协作开发（30 分钟）

```text
1. ../AGENTS.md                          ← AI 规则
2. ai/errorx.md                          ← errorx AI 指南（含 10 个食谱）
3. ../errorx/example_test.go             ← Go 标准 Example
4. 复制 examples/errorx-basic 改成业务代码
```

### 路径 C：架构师 / PR review

```text
1. design/errorx.md                      ← 完整设计规范
2. contracts/errorx.md                   ← 不可破坏契约
3. process/errorx-acceptance-checklist.md ← 验收清单
```

### 路径 D：CI/CD 集成

```text
1. process/errorx-acceptance-checklist.md
2. ../Makefile                           ← make verify-errorx / make test-errorx
3. process/windows-script-policy.md      ← Windows 兼容
```

---

## 文档维护规则

1. **单一入口原则**：每个模块只有一个 README 作为入口，其他文档从 README 链接出去
2. **不重复**：同一信息只在一个地方定义，其他地方链接过去
3. **分类清晰**：design（设计）/ contracts（契约）/ guides（指南）/ ai（AI）/ process（流程）/ migration（迁移）严格分类
4. **AI 文档合并**：AI 速查、食谱、禁用模式合并为单一 `ai/<module>.md`，AI 拿一个文件即可
5. **删除即合并**：如果两个文档 80% 重复，合并成一个，删除另一个

---

## 当前状态

```text
✅ errorx 文档体系已完成单一入口重构
✅ configx 文档体系已按 skill 建立
⬜ logx 文档体系待按相同模式重构
✅ metricsx / dtmx 已补 AI 指南和 layout 启动模板
⬜ 后续模块（httpx / grpcx / dbx / ...）按相同模式建立文档
```
