# logx basic example

最小可运行示例，展示 logx 的核心用法。

## 运行

```bash
go run ./examples/logx-basic
```

## 预期输出

```text
time=2026-... level=INFO msg="service started" service=aihub env=dev version=v1.0.0 addr=:8000
time=2026-... level=DEBUG msg="querying skill" service=aihub env=dev version=v1.0.0 module=skill_repo id=skill_001
...
current level: info
after SetLevel(debug): debug
noop enabled? false
```

## 这个示例展示了什么

1. **Bootstrap**：`logx.New(cfg)` 返回 Logger + LevelController
2. **DefaultConfig**：dev 模式用 console 格式，AddSource=true
3. **结构化字段**：`logx.String` / `logx.Int` / `logx.Err` 等
4. **Named logger**：`logger.Named("skill_repo")` 添加 module 标签
5. **With 持久字段**：`logger.With(...)` 返回带固定字段的新 logger
6. **Error 日志**：`logx.Err(err)` 自动提取错误类型信息
7. **Context 注入**：`logx.Inject` + `logx.FromContext` 实现请求级字段
8. **预置 helper**：`LogExternalCall` / `LogAuditHint`
9. **动态级别**：`LevelController.SetLevel("debug")` 运行时调整
10. **Noop logger**：`logx.Noop()` 用于禁用场景或测试

## 下一步

如果要看 HTTP access log + panic recovery 完整示例，跑 `examples/logx-http`：

```bash
go run ./examples/logx-http
```

它展示了 logx 在真实 HTTP 服务中的端到端用法。

## 相关文档

- `logx/README.md` — 完整用户指南
- `docs/ai/logx.md` — AI 编码食谱（10 场景）
- `logx/example_test.go` — 所有 API 的 Go 标准 Example
- `logx/example_business_test.go` — 10 个完整业务场景
