# GitHub Actions 构建委托规范

本地沙箱经常存在离线依赖、编译时间、工具链缓存限制。因此完整构建和 fullflow demo 不应该只依赖本地运行。

## 1. 判定原则

以下情况不直接判定为业务失败：

- 本地 GOPROXY=off 导致依赖无法解析。
- fullflow 编译时间超过沙箱限制。
- protoc/buf 工具链缺失。
- 大型 layout 生成物首次编译超时。

必须记录到 `TODO_STATUS.md`，然后委托 GitHub Actions。

## 2. 本地优先跑 targeted tests

```bash
go test ./serverx ./validation/iamgateway ./bootx ./requestx ./admissionx ./middleware/autowire ./ratelimitx ./clientpolicyx ./middleware/retry ./middleware/timeout
```

## 3. Actions 负责长任务

当前 workflow：

```text
.github/workflows/governance-demo.yml
```

它负责：

- 下载依赖。
- 运行治理链路 targeted tests。
- 尝试 layout/fullflow compile smoke。
- 上传构建日志 artifact。

## 4. 离线依赖回灌

如果本地必须复现 Actions 结果，可以让 Actions 产出：

```text
go module cache / goproxy bundle
buf cache
生成器二进制
测试日志
```

然后回灌到沙箱或本地开发机。

## 5. 禁止事项

- 禁止因为本地超时而删除测试。
- 禁止绕过 serverx/autowire/requestx 改写 demo。
- 禁止用 mock 替代框架链路本身。
