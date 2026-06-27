# Aisphere Kernel Source Adoption Baseline

版本：v0.0.0-baseline

本文档记录本次迁移基线的处理结果。

## 1. 迁移目标

本次迁移不是在业务项目中依赖 Kratos，也不是在 Kratos 外面包一层，而是将 Kratos 源码吸收到 `github.com/aisphereio/kernel` 中，作为 Aisphere Kernel 的初始代码基线。

完成后的项目身份：

```text
github.com/aisphereio/kernel
```

业务与后续框架开发只面向 Aisphere Kernel，不再以 Kratos 作为产品心智。

## 2. 已完成事项

- 根模块从 `github.com/go-kratos/kratos/v3` 本地化为 `github.com/aisphereio/kernel`。
- 根包名从 `kratos` 本地化为 `kernel`。
- CLI 目录从 `cmd/kratos` 迁移为 `cmd/kernel`。
- CLI module path 本地化为 `github.com/aisphereio/kernel/cmd/kernel`。
- `protoc-gen-go-http` 和 `protoc-gen-go-errors` 命令 module path 已本地化。
- `contrib/*` 子模块 module path 已本地化为 `github.com/aisphereio/kernel/contrib/.../v3`。
- 代码中的 `github.com/go-kratos/kratos...` import path 已替换为 `github.com/aisphereio/kernel...`。
- 原上游 README 已保存到 `docs/upstream/`。
- 新增 `THIRD_PARTY_NOTICES.md` 用于保留上游来源和 MIT 许可说明。
- 合并了小包中的 Aisphere Kernel 建设文档、AI 规范、验收规范、errorx 设计文档、errorx 代码和测试。
- 新增本地 `.bin` 工具链 Makefile，不再要求全局安装生成工具。

## 3. 仍保留的上游能力

以下目录作为本地源码能力完整保留，后续逐步改造：

```text
app.go / options.go
config/
errors/
log/
middleware/
metadata/
transport/
encoding/
registry/
selector/
contrib/
cmd/kernel/
cmd/protoc-gen-go-http/
cmd/protoc-gen-go-errors/
third_party/
```

## 4. 已合并的 Aisphere 新能力

```text
errorx/
docs/design/errorx.md
docs/design/logx.md
docs/process/module-acceptance.md
docs/contracts/errorx.md
docs/ai/*
examples/errorx-basic/
AGENTS.md
kernel.manifest.yaml
```

## 5. 下一步建议

不要立刻大规模改 DB、S3、IAM。建议按以下顺序逐步替换：

```text
1. errors → 按 errorx 语义改造或最终切换到 errorx
2. log → 按 logx 规范改造
3. config → 简化并接入 Kernel 配置规范
4. app/core → 加 Module 生命周期
5. transport/http → 统一 response 和 request context
6. data/ → 新增 DB / Redis / S3 抽象
7. security/ → 新增 authn / identity / authz / audit
8. starter/ + adapters/ → 接具体实现
```

## 6. 验证说明

本包根模块保留上游 Go 版本要求：

```text
go 1.25.0
```

因此请使用 Go 1.25+ 执行：

```bash
make verify
```

当前 sandbox 环境只有 Go 1.23.2，并且无法联网下载 Go 1.25 toolchain，所以未能在 sandbox 内完整运行 `go test ./...`。

## 7. 法务和来源

本项目包含来源于 `go-kratos/kratos` 的 MIT 许可源码。上游说明保存在：

```text
THIRD_PARTY_NOTICES.md
docs/upstream/KRATOS_README.md
docs/upstream/KRATOS_README_zh.md
```
