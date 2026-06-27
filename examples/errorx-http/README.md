# errorx HTTP example

完整 HTTP handler 示例，展示 errorx 在真实业务场景下的端到端用法。

## 运行

```bash
go run ./examples/errorx-http
```

服务器监听 `:18080`。

## 测试不同错误场景

打开另一个终端，依次执行：

```bash
# 200 OK — 成功
curl -i 'http://localhost:18080/skills?id=skill_001'

# 400 Bad Request — 参数缺失
curl -i 'http://localhost:18080/skills?id='

# 404 Not Found — 资源不存在
curl -i 'http://localhost:18080/skills?id=missing'

# 500 Internal — 数据库失败（保留 cause，retryable=true）
curl -i 'http://localhost:18080/skills?id=boom'

# 400 Bad Request — 创建时名字为空
curl -i -X POST http://localhost:18080/skills -d '{"name":""}'

# 403 Forbidden — 无权限
curl -i -X POST http://localhost:18080/skills -d '{"name":"forbidden"}'

# 409 Conflict — 资源已存在
curl -i -X POST http://localhost:18080/skills -d '{"name":"demo"}'
```

## 场景 → errorx 构造器 → HTTP 响应对应表

| # | curl 场景 | errorx 构造器 | HTTP | error_code | 对应 Example |
|---|---|---|---:|---|---|
| 1 | `?id=skill_001` | （无错误） | 200 | — | — |
| 2 | `?id=` (空) | `errorx.BadRequest` | 400 | `AIHUB_SKILL_ID_REQUIRED` | `ExampleBadRequest` |
| 3 | `?id=missing` | `errorx.NotFound` | 404 | `AIHUB_SKILL_NOT_FOUND` | `ExampleNotFound` |
| 4 | `?id=boom` | `errorx.Wrap` (Internal) | 500 | `AIHUB_SKILL_QUERY_FAILED` | `ExampleWrap`, `ExampleInternal` |
| 5 | POST `{"name":""}` | `errorx.BadRequest` | 400 | `AIHUB_SKILL_NAME_REQUIRED` | `ExampleBadRequest` |
| 6 | POST `{"name":"forbidden"}` | `errorx.Forbidden` | 403 | `AIHUB_SKILL_CREATE_DENIED` | `ExampleForbidden` |
| 7 | POST `{"name":"demo"}` | `errorx.Conflict` | 409 | `AIHUB_SKILL_ALREADY_EXISTS` | `ExampleConflict` |

每个错误场景对应 `errorx/example_test.go` 里的一个 Example，方便对照学习。

## 期望响应

每个错误响应都是统一的 JSON 格式，HTTP 状态码与 errorx 构造器一致：

```http
HTTP/1.1 404 Not Found
Content-Type: application/json; charset=utf-8

{
  "code": "AIHUB_SKILL_NOT_FOUND",
  "message": "skill not found",
  "metadata": {
    "resource": "skill"
  }
}
```

服务端日志会记录完整错误信息（含脱敏后的内部 metadata）：

```json
{
  "level": "ERROR",
  "msg": "request failed",
  "error_code": "AIHUB_SKILL_NOT_FOUND",
  "http_status": 404,
  "retryable": false,
  "category": "not_found",
  "metadata": {
    "skill_id": "missing"
  },
  "error": "skill not found"
}
```

## 学到什么

这个示例展示了 errorx 的核心用法：

1. **错误码声明**：模块顶部声明 `errorx.Code` 常量，全大写蛇形
2. **构造器选择**：根据语义选 `NotFound` / `BadRequest` / `Forbidden` / `Conflict` 等，不手写 HTTP 状态
3. **保留 cause**：用 `errorx.Wrap(err, code, ...)` 包装底层错误，支持 `errors.Is`
4. **Metadata 分层**：`WithMetadata` 放内部信息（日志用），`WithPublicMetadata` 放可返回前端的信息
5. **统一响应**：`writeErrorx` 是唯一把 errorx 转 HTTP 的地方，HTTP 状态码和 JSON body 都从 errorx 提取
6. **日志消费**：用 `errorx.CodeOf` / `HTTPStatusOf` / `SafeMetadataOf` 提取字段，不类型断言
7. **响应只暴露 public**：`errorResponse.Metadata` 用 `PublicMetadataOf`，绝不会泄漏 `Metadata`

## 在真实项目中

`writeErrorx` 在真实项目中由 `kernel httpx` 中间件提供，业务 handler 只需 `return errorx.NotFound(...)`，
中间件自动渲染响应。本示例为了独立性把渲染逻辑内联了。

## 相关文档与示例

| 想学什么 | 看哪里 |
|---|---|
| errorx 完整指南 | `errorx/README.md` |
| AI 编码食谱（10 场景） | `docs/ai/errorx.md` |
| 所有构造器 Example | `errorx/example_test.go` |
| 完整业务场景 Example | `errorx/example_business_test.go` |
| 最小可运行示例 | `examples/errorx-basic/` |
| 本 HTTP 示例 | `examples/errorx-http/`（当前目录） |
