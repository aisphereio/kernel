# contextx basic example

最小可运行示例，展示 contextx 的核心用法。

## 运行

```bash
go run ./examples/contextx-basic
```

## 预期输出

```text
request_id: req_abc
trace_id: trace_xyz
subject_id: u_123
tenant: t_acme
is_admin: true
can_write: true
for errorx: request_id=req_abc trace_id=trace_xyz
anonymous subject_id:
feature X enabled
merged trace_id: trace_parent
merged request_id: req_job
```

## 这个示例展示了什么

1. **InjectRequestContext**：handler 边界一次性注入所有请求字段
2. **提取值**：request_id / trace_id / principal / tenant / logger
3. **Principal 方法**：HasRole / HasScope / IsAuthenticated
4. **RequestIDAndTraceIDFromContext**：一次取两个 ID（给 errorx 用）
5. **nil 安全**：匿名请求的 SubjectIDFromContext 返回 ""
6. **自定义值**：With / From 用于特性开关等
7. **Merge**：合并两个 context，继承两者的值

## 相关文档

- `contextx/README.md` — 完整用户指南
- `docs/ai/contextx.md` — AI 编码食谱（10 场景）
- `contextx/example_test.go` — 所有 API 的 Go 标准 Example
- `contextx/example_business_test.go` — 10 个完整业务场景
