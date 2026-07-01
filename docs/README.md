# Aisphere Kernel 文档中心

本目录只保留当前 Kernel 开发范式需要阅读的主线文档。过期、阶段性或历史对比材料移入 `docs/archive/legacy/`，不要作为新业务开发依据。

## 1. 先读这四份

1. [快速开始](getting-started.md)
2. [Kernel 边界与分层](architecture/kernel-boundary.md)
3. [规范驱动开发范式](ai/kernel-development-paradigm.md)
4. [Agent 开发规则](../AGENTS.md)

标准起步命令：

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel new todo-service
cd todo-service
make tools
```

MVP 骨架：

```bash
kernel new todo-service --mvp
```

本地/私有 layout 才需要 `--repo`。

## 2. Runtime API 边界

| 主题 | 文档 |
|---|---|
| Package 状态 | [contracts/package-status.md](contracts/package-status.md) |
| Runtime API 边界 | [contracts/runtime-api-boundary.md](contracts/runtime-api-boundary.md) |
| Proto 能力矩阵 | [contracts/proto-capability-matrix.md](contracts/proto-capability-matrix.md) |

## 3. Contract：写业务前必须对齐

| 主题 | 文档 |
|---|---|
| Access Policy | [contracts/access-policy.md](contracts/access-policy.md) |
| RequestInfo | [contracts/request-info.md](contracts/request-info.md) |
| 生成 RequestInfo | [contracts/generated-request-info.md](contracts/generated-request-info.md) |
| Admission | [contracts/admission.md](contracts/admission.md) |
| 服务间调用 | [contracts/downstream-policy.md](contracts/downstream-policy.md) |
| 限流 | [contracts/rate-limit.md](contracts/rate-limit.md) |
| Serverx | [contracts/serverx.md](contracts/serverx.md) |
| Gateway 路由 | [contracts/gateway-routing.md](contracts/gateway-routing.md) |
| 启动治理校验 | [contracts/boot-governance-validation.md](contracts/boot-governance-validation.md) |
| 数据开发范式 | [contracts/dbx-data-development.md](contracts/dbx-data-development.md) |

## 4. Design

| 主题 | 文档 |
|---|---|
| Kubernetes apiserver 可学习范式 | [design/k8s-apiserver-lessons.md](design/k8s-apiserver-lessons.md) |
| Kernel 边界与分层 | [architecture/kernel-boundary.md](architecture/kernel-boundary.md) |
| AuthN/AuthZ/Audit 设计 | [design/security-authn-authz-auditx.md](design/security-authn-authz-auditx.md) |

## 5. Validation 状态

`validation/` 已从 runtime tree 移除。场景检查、generated-shape 实验、IAM/Gateway/SkillService 联调验证，不再放在 Kernel 主模块默认包图里。

后续需要验证业务时，使用独立仓库、生成项目自己的 tests、显式 build tag 或 GitHub Actions 专用 job。

## 6. AI / Agent 工作方式

| 主题 | 文档 |
|---|---|
| 开发范式 | [ai/kernel-development-paradigm.md](ai/kernel-development-paradigm.md) |
| GitHub Actions 委托构建 | [ai/github-actions-build-delegation.md](ai/github-actions-build-delegation.md) |

## 7. 文档维护规则

- 新业务开发只参考当前 README 链接出的文档。
- 历史分析、对比、阶段性草稿放入 `docs/archive/legacy/`。
- 文档默认使用中文。
- 代码能力变化时，必须同步更新对应 `docs/contracts/*.md` 和 `AGENTS.md`。
- 如果文档与代码冲突，以代码和测试为准，并立即修正文档。
