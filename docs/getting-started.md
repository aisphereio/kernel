# Kernel 快速开始

这份文档面向第一次使用 `github.com/aisphereio/kernel` 的业务开发者。

Kernel 分成两类东西：

1. **CLI / 代码生成工具**：开发者只需要先全局安装 `kernel` CLI；生成项目后，用项目内 `make tools` 安装剩余工具链到 `.bin/`。
2. **运行时库**：由生成后的业务项目在 `go.mod` 中依赖，例如 `errorx`、`logx`、`configx`、`serverx`、`dbx`、`cachex`、`objectstorex`。

业务开发者不需要直接 import 生成器包，也不需要手动逐个全局安装 `protoc-gen-*`。

---

## 1. 快速体验：安装最新发布版

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel version
```

然后从独立 layout 仓库创建服务：

```bash
kernel new todo-service --repo https://github.com/aisphereio/kernel-layout.git
cd todo-service
make tools
make api
make proto-check
make verify
make run
```

`--repo` 是公开安装后的推荐写法。因为 `go install` 只安装 CLI 二进制，不会携带 Kernel 仓库里的本地 `layout/` 目录。

---

## 2. 工程实践：固定 Kernel 版本

生产项目不要依赖隐式漂移的工具链。建议固定一个 Kernel release tag，但用户仍然只需要先全局安装一个 `kernel` CLI。

```bash
KERNEL_VERSION=v0.1.16

go install github.com/aisphereio/kernel/cmd/kernel@${KERNEL_VERSION}
```

创建项目时把版本写进生成项目：

```bash
kernel new todo-service \
  --repo https://github.com/aisphereio/kernel-layout.git \
  --kernel-version ${KERNEL_VERSION}
```

进入项目后，让项目 Makefile 安装剩余工具链：

```bash
cd todo-service
make tools
make api
make proto-check
make verify
make run
```

`make tools` 会根据生成项目里的 `KERNEL_VERSION` 安装 Kernel 生成器，并把工具放在当前项目 `.bin/` 目录中。这样不会污染全局 PATH，也避免每个开发者记一长串 `go install` 命令。

---

## 3. 本地开发 layout

如果你正在同时开发 `kernel` 和 `kernel-layout`，可以用本地 layout：

```bash
export KERNEL_LAYOUT=/path/to/kernel-layout
kernel new todo-service --kernel-version ${KERNEL_VERSION}
```

Windows PowerShell：

```powershell
$env:KERNEL_LAYOUT="E:\coding\aisphereio\kernel-layout"
kernel new todo-service --kernel-version $env:KERNEL_VERSION
```

也可以直接传本地路径：

```bash
kernel new todo-service --repo /path/to/kernel-layout --kernel-version ${KERNEL_VERSION}
```

---

## 4. 常用 scaffold 参数

```bash
kernel new todo-service \
  --repo https://github.com/aisphereio/kernel-layout.git \
  --kernel-version ${KERNEL_VERSION} \
  --features dbx,cachex,objectstorex,authn,authz,auditx,metricsx,logx,configx \
  --db-driver postgres \
  --cache-driver redis \
  --objectstore-driver minio \
  --authn-provider casdoor \
  --authz-provider spicedb
```

当前 CLI 支持的核心参数：

| 参数 | 作用 |
|---|---|
| `--repo`, `-r` | layout 仓库 URL 或本地 layout 路径 |
| `--branch`, `-b` | layout 仓库分支 |
| `--kernel-version` | 写入生成项目 Makefile 的 Kernel 版本 |
| `--features` | 启用的 scaffold 能力列表 |
| `--db-driver` | 默认 DB driver，例如 `postgres` |
| `--cache-driver` | 默认 cache driver，例如 `redis` |
| `--objectstore-driver` | 默认对象存储 driver，例如 `minio` |
| `--authn-provider` | 默认认证 provider，例如 `casdoor` |
| `--authz-provider` | 默认授权 provider，例如 `spicedb` |
| `--nomod` | 在已有 Go module 下新增子项目 |

---

## 5. 生成项目后的标准流程

```bash
cd todo-service
make tools        # 安装项目需要的 protoc/buf/kernel 生成器到 .bin/
make api          # 生成 proto / HTTP / gRPC / gateway 代码
make proto-check  # buf lint/build 和 Kernel proto 检查
make verify       # 项目测试、vet、生成校验
make run          # 本地启动服务
```

默认 layout 会带一个 proto-first Todo CRUD 示例，方便验证 HTTP、gRPC、配置、日志、metrics、DB/cache/objectstore 初始化路径。

---

## 6. 常见问题

### local layout not found

错误：

```text
ERROR: failed to resolve layout(local layout not found; pass --repo or set KERNEL_LAYOUT)
```

原因：你安装的是 CLI 二进制，不是在 Kernel 源码仓库中运行。解决方式二选一：

```bash
kernel new todo-service --repo https://github.com/aisphereio/kernel-layout.git
```

或者：

```bash
export KERNEL_LAYOUT=/path/to/kernel-layout
kernel new todo-service
```

### 应该 go install kernel 还是 import kernel？

都要，但用途不同：

- `go install github.com/aisphereio/kernel/cmd/kernel@<version>`：安装脚手架 CLI。
- `make tools`：在生成项目中安装 protoc/buf/Kernel 生成器等项目本地工具。
- `import github.com/aisphereio/kernel/...`：业务项目使用运行时库。
- 不要在业务代码里 import `cmd/protoc-gen-*` 这类生成器包。

### 用 @latest 还是固定 tag？

快速体验可以用 `@latest`。工程项目建议固定 tag，例如 `v0.1.16`，并通过 `kernel new --kernel-version` 写入生成项目；后续工具安装交给项目内 `make tools`。
