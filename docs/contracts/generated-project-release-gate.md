# 真实生成项目 Release Gate

Kernel 的单元测试、生成器测试和 `make api` 通过，并不能证明 `kernel new` 生成的业务服务真的可用。发布门禁必须验证 Kernel、kernel-layout 和生成项目三层契约一致。

## 强制链路

```text
Kernel 当前 commit
  -> make tools / make api / make proto-check
  -> kernel new boundary-smoke --repo kernel-layout
  -> generated project replace github.com/aisphereio/kernel => 当前 Kernel commit
  -> make tools-local KERNEL_LOCAL=<current kernel>
  -> make verify
```

这条门禁必须发现并阻断以下问题：

- template proto 本身不满足 Buf / `buf-check-aisphere` 规范；
- proto 已修改但提交的 `.pb.go/_grpc/_http/_authz/_gateway/_kernel` 生成物没有同步；
- template 保留已经删除的生成步骤，例如不存在 proto 源的旧 config generator；
- 业务测试仍引用旧生成类型；
- 当前 Kernel generator 生成的代码无法被默认 layout 编译；
- 当前 Kernel runtime 和默认 layout 的 serverx/authn/authz/accessx 契约不一致；
- `go.mod/go.sum` 或生成项目依赖需要人工补齐才能构建。

## 规则

1. 不允许通过放宽安全/access contract 来让生成项目通过。
2. Buf 的纯风格规则只有在模板明确选择另一种稳定范式时才能显式豁免，并写清理由。
3. 修改 proto 后必须运行真实 `make api`；禁止手工伪造 generated artifacts。
4. `kernel new` 输出的默认项目必须在无人工补代码的情况下直接通过 `make verify`。
5. Kernel release gate 应始终以当前 Kernel commit 构建 generator/runtime，不能偷偷使用已发布旧版本。
6. kernel-layout 变更必须通过自己的 build/test，同时接受 Kernel 的跨仓库生成项目 gate 验证。

## 本轮边界硬化

本轮 hardening 已通过这条 gate 发现并修复了模板中的三类漂移：

- proto package / RPC request 命名 contract 漂移；
- 已废弃 `buf.gen.config.yaml` 仍被模板 verify 执行；
- streaming request proto 重命名后，业务测试和已提交生成物未同步。

这类问题以后必须由 CI 自动发现，不依赖人工记忆。
