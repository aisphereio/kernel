# errorx basic example

最小可运行示例，展示 errorx 的构造、包装、检查三件套，不涉及 HTTP。

## 运行

```bash
go run ./examples/errorx-basic
```

## 预期输出

```text
code: AIHUB_SKILL_CREATE_FAILED
status: 500
retryable: true
request_id: req_123
```

## 这个示例展示了什么

```go
cause := errors.New("pq: connection refused")          // 底层错误
err := errorx.Wrap(cause, ErrSkillCreateFailed,         // 包装成业务错误
    errorx.WithMessage("创建技能失败"),
    errorx.WithHTTPStatus(500),
    errorx.WithRetryable(true),                         // 标记可重试
    errorx.WithMetadata("skill_id", "skill_001"),       // 内部 metadata
    errorx.WithRequestID("req_123"),                    // 请求 ID
    errorx.WithTraceID("trace_abc"),                    // 链路追踪 ID
)

// inspect helpers 从 err 中提取语义，无需类型断言
errorx.CodeOf(err)       // AIHUB_SKILL_CREATE_FAILED
errorx.HTTPStatusOf(err) // 500
errorx.RetryableOf(err)  // true
errorx.RequestIDOf(err)  // req_123
```

## 下一步

如果要看 HTTP handler 完整示例，跑 `examples/errorx-http`：

```bash
go run ./examples/errorx-http
```

它展示了 errorx 在真实 HTTP 接口中的端到端用法（包括 JSON 响应、日志记录、metadata 分层）。

## 相关文档

- `errorx/README.md` — 完整用户指南
- `docs/ai/errorx.md` — AI 编码食谱
