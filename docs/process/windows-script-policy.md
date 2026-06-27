# Windows PowerShell 执行策略处理说明

## 问题现象

在 Windows 上直接执行：

```powershell
./scripts/verify-errorx.ps1
```

可能报错：

```text
未对文件进行数字签名。无法在当前系统上运行该脚本。
```

这不是 Go 代码问题，也不是 errorx 验收失败，而是 PowerShell Execution Policy 阻止了未签名脚本。

## 推荐方式

在仓库根目录执行：

```cmd
.\scripts\verify-errorx.cmd
```

`verify-errorx.cmd` 会用下面的方式启动 PowerShell：

```cmd
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "scripts\verify-errorx.ps1"
```

这个 `Bypass` 只对本次进程有效，不会修改当前用户或机器级策略。


## Windows 工具链入口

仓库里的 Makefile 已经适配 Windows。下面这些命令可以直接在 PowerShell 里执行：

```powershell
make tools
make test-cmd
make verify-errorx
```

如果本机的 `make` 仍然走到了不兼容的 shell，或者你不想依赖 make，可以直接使用 `.cmd` 包装脚本：

```cmd
.\scripts\tools.cmd
.\scripts\test-cmd.cmd
.\scripts\verify-errorx.cmd
```

这些 `.cmd` 脚本会用 `-ExecutionPolicy Bypass` 启动对应的 PowerShell 脚本，只对当前进程生效，不会修改系统策略。

## 临时方式

也可以直接执行：

```powershell
powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify-errorx.ps1
```

或者先在当前 PowerShell 窗口临时放开：

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\scripts\verify-errorx.ps1
```

`-Scope Process` 只影响当前窗口，关闭窗口后失效。

## 不推荐方式

不建议直接执行：

```powershell
Set-ExecutionPolicy RemoteSigned -Scope LocalMachine
```

因为它会修改机器级策略，影响范围更大。除非你明确知道公司安全策略允许这样做。

## 完全绕过脚本

脚本本质上只是执行这些命令：

```powershell
go test ./errorx -v
go test ./errorx -race
go test ./errorx -cover
go test ./...
go vet ./...
go run ./examples/errorx-basic
go test ./errorx -bench=.
```
