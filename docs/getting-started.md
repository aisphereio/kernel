# Kernel 快速开始

这份文档面向第一次使用 `github.com/aisphereio/kernel` 的业务开发者。

## 默认 full profile

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel version
kernel new todo-service
cd todo-service
make tools
make api
make deploy
make proto-check
make verify
make run
```

`kernel new` 默认使用独立模板仓库 `https://github.com/aisphereio/kernel-layout.git`。Kernel 仓库只保留框架 runtime、契约、生成器和 CLI。

## MVP

```bash
kernel new todo-service --mvp
cd todo-service
make tools
make api
make test
make run
```

`--mvp` 只用于最小骨架验证。正式业务服务应回到 full profile。

## 裁剪能力

```bash
kernel new todo-service --disable iam,gateway,dtmx
```

`--disable` 由 `kernel-layout` 仓库里的 `.kernel/features/<feature>` 处理。

## 固定 Kernel 版本

```bash
KERNEL_VERSION=v0.2.4
go install github.com/aisphereio/kernel/cmd/kernel@${KERNEL_VERSION}
kernel new todo-service --kernel-version ${KERNEL_VERSION}
```

## 本地开发 layout

本地调试模板时显式指定 `KERNEL_LAYOUT` 或 `--repo`：

```bash
export KERNEL_LAYOUT=/your/kernel-layout
kernel new todo-service --kernel-version ${KERNEL_VERSION}
```

也可以直接传本地路径或指定远程分支：

```bash
kernel new todo-service --repo /your/kernel-layout
kernel new todo-service --repo https://github.com/aisphereio/kernel-layout.git --branch main
```

## 常用参数

```text
--profile
--mvp
--disable
--repo
--branch
--kernel-version
--features
--db-driver
--cache-driver
--objectstore-driver
--authn-provider
--authz-provider
--nomod
```

## 生成项目后的标准流程

以下命令在生成后的业务服务仓库中执行，由 `kernel-layout` 的 Makefile 提供：

```bash
cd todo-service
make tools
make api
make deploy
make proto-check
make verify
make run
```

## Kernel 仓库自身的验证

如果你在开发 `github.com/aisphereio/kernel` 本身，根 Makefile 只负责 runtime、contract、generator 和 CLI：

```bash
make tools
make api
make proto-check
make verify
```

服务部署清单生成模板和生成项目 Makefile 属于 `kernel-layout`。

## 规则

业务开发者不需要直接 import 生成器包，也不需要手动逐个全局安装 `protoc-gen-*`。生成项目通过 `make tools` 安装项目本地工具链。
