# README.md Examples 索引补丁

> 把下面的内容追加到 `errorx/README.md` 的末尾（在"15. 设计哲学一句话"之后）。
> 这是一份"按场景找 Example"的索引表，让 AI 和人类都能快速定位示例。

---

## 16. Examples 索引（按场景查找）

**所有 Example 都是可运行的**：`go test ./errorx -run=Example -v` 可以看到输出。
也可以用 `go doc errorx.Xxx` 在终端查看单个 Example。

### 16.1 构造器示例（11 个）

| 构造器 | Example 函数 | 何时用 |
|---|---|---|
| `BadRequest` | `ExampleBadRequest` | 参数校验失败 |
| `Unauthorized` | `ExampleUnauthorized` | 未登录 / token 无效 |
| `Forbidden` | `ExampleForbidden` | 已登录但无权限 |
| `NotFound` | `ExampleNotFound` | 资源不存在 |
| `Conflict` | `ExampleConflict` | 资源已存在 / 状态冲突 |
| `RequestTimeout` | `ExampleRequestTimeout` | 本服务处理请求超时 |
| `TooManyRequests` | `ExampleTooManyRequests` | 触发限流 |
| `ClientClosed` | `ExampleClientClosed` | 客户端主动断开 |
| `Internal` | `ExampleInternal` | 服务器内部错误 |
| `Unavailable` | `ExampleUnavailable` | 上游依赖不可用 |
| `Timeout` | `ExampleTimeout` | 上游依赖超时 |

### 16.2 核心构造（3 个）

| 函数 | Example | 何时用 |
|---|---|---|
| `New` | `ExampleNew` | 自定义错误，需要非默认 HTTP 状态 |
| `Wrap` | `ExampleWrap`, `ExampleWrap_nil`, `ExampleWrap_seesThroughFmtErrorf` | 包装底层错误，保留 cause |
| `From` | `ExampleFrom`, `ExampleFrom_withOptions`, `ExampleFrom_foreignStatusCode` | 把第三方错误转成 errorx |

### 16.3 Option 示例（12 个）

| Option | Example | 何时用 |
|---|---|---|
| `WithMessage` | `ExampleWithMessage` | 覆盖默认 message |
| `WithHTTPStatus` | `ExampleWithHTTPStatus` | 覆盖默认 HTTP 状态（少用） |
| `WithGRPCCode` | `ExampleWithGRPCCode` | 覆盖默认 gRPC code（少用） |
| `WithRetryable` | `ExampleWithRetryable` | 覆盖默认 retryable |
| `WithCause` | `ExampleWithCause` | 保留底层错误（用于 errors.Is） |
| `WithMetadata` | `ExampleWithMetadata` | 加内部 metadata（日志/审计） |
| `WithMetadataMap` | `ExampleWithMetadataMap` | 一次加多个 metadata |
| `WithPublicMetadata` | `ExampleWithPublicMetadata` | 加公开 metadata（前端可见） |
| `WithPublicMetadataMap` | `ExampleWithPublicMetadataMap` | 一次加多个公开 metadata |
| `WithRequestID` | `ExampleWithRequestID` | 加请求 ID |
| `WithTraceID` | `ExampleWithTraceID` | 加链路追踪 ID |
| `WithCategory` | `ExampleWithCategory` | 覆盖默认分类（少用） |
| `WithSeverity` | `ExampleWithSeverity` | 覆盖默认日志严重级别 |
| `WithStack` | `ExampleWithStack` | 捕获调用栈（仅 5xx） |

### 16.4 Inspect 示例（13 个）

| 函数 | Example | 消费方 |
|---|---|---|
| `CodeOf` | `ExampleCodeOf`, `ExampleCodeOf_nil`, `ExampleCodeOf_foreignError` | httpx, auditx |
| `MessageOf` | `ExampleMessageOf` | httpx, auditx |
| `HTTPStatusOf` | `ExampleHTTPStatusOf`, `ExampleHTTPStatusOf_nil`, `ExampleHTTPStatusOf_foreignError` | httpx |
| `GRPCCodeOf` | `ExampleGRPCCodeOf` | grpcx |
| `RetryableOf` | `ExampleRetryableOf` | workerx |
| `MetadataOf` | `ExampleMetadataOf` | logx, auditx |
| `SafeMetadataOf` | `ExampleSafeMetadataOf` | logx（自动脱敏） |
| `PublicMetadataOf` | `ExamplePublicMetadataOf` | httpx, grpcx |
| `RequestIDOf` | `ExampleRequestIDOf` | logx, auditx |
| `TraceIDOf` | `ExampleTraceIDOf` | logx, auditx |
| `CategoryOf` | `ExampleCategoryOf` | metricsx, 告警 |
| `SeverityOf` | `ExampleSeverityOf` | logx |
| `StackOf` | `ExampleStackOf` | logx（仅 debug） |

### 16.5 谓词示例（12 个）

| 谓词 | Example |
|---|---|
| `IsBadRequest` | `ExampleIsBadRequest` |
| `IsUnauthorized` | `ExampleIsUnauthorized` |
| `IsForbidden` | `ExampleIsForbidden` |
| `IsNotFound` | `ExampleIsNotFound` |
| `IsConflict` | `ExampleIsConflict` |
| `IsRequestTimeout` | `ExampleIsRequestTimeout` |
| `IsTooManyRequests` | `ExampleIsTooManyRequests` |
| `IsClientClosedRequest` | `ExampleIsClientClosedRequest` |
| `IsInternal` | `ExampleIsInternal` |
| `IsUnavailable` | `ExampleIsUnavailable` |
| `IsTimeout` | `ExampleIsTimeout` |
| `IsCode`（精确匹配 code） | `ExampleIsCode` |

### 16.6 消费方示例（logx / metricsx / auditx / workerx / httpx）

| 消费方 | Example | 展示内容 |
|---|---|---|
| logx | `ExampleFields`, `ExampleFields_nil`, `ExampleBusiness_logEntry` | 提取日志字段，自动脱敏 |
| metricsx | `ExampleMetricsLabels`, `ExampleBusiness_metricsLabels` | 低基数 Prometheus 标签 |
| auditx | `ExampleBusiness_auditRecord` | 从错误构建审计记录 |
| workerx | `ExampleBusiness_workerRetry` | 判断重试 vs 失败 |
| httpx | `ExampleBusiness_httpResponse` | JSON 响应格式 |

### 16.7 完整业务场景示例（10 个，在 `example_business_test.go`）

| 场景 | Example | 层级 |
|---|---|---|
| 仓库层 DB 错误 → errorx | `ExampleBusiness_repositoryLayer` | repository |
| 服务层校验 + 冲突检测 | `ExampleBusiness_serviceLayer` | service |
| 上游模型超时 | `ExampleBusiness_upstreamTimeout` | service |
| 鉴权拒绝（带 metadata） | `ExampleBusiness_authzDenied` | service |
| Worker 重试决策 | `ExampleBusiness_workerRetry` | worker |
| HTTP 响应 JSON 格式 | `ExampleBusiness_httpResponse` | handler |
| 审计记录 | `ExampleBusiness_auditRecord` | auditx |
| 日志条目（含脱敏） | `ExampleBusiness_logEntry` | logx |
| 指标标签（低基数） | `ExampleBusiness_metricsLabels` | metricsx |
| 多层 Wrap（保留链） | `ExampleBusiness_multiLayerWrap` | 跨层 |

### 16.8 高级示例

| 主题 | Example |
|---|---|
| Clone（深拷贝 + 覆盖） | `ExampleError_Clone` |
| %+v 调试格式 | `ExampleError_Format`, `ExampleError_Format_verbose` |
| As（从链中提取） | `ExampleAs` |
| IsKernelError | `ExampleIsKernelError` |
| 第三方 StatusCode() 兼容 | `ExampleFrom_foreignStatusCode`, `ExampleHTTPStatusOf_foreignError` |
| 第三方 ErrorCode() 兼容 | `ExampleCodeOf_foreignError` |
| 错误码校验 | `ExampleIsValidCode`, `ExampleNormalizeCode` |
| nil 安全 | `ExampleCodeOf_nil`, `ExampleHTTPStatusOf_nil`, `ExampleFields_nil` |
| Context 集成 | `ExampleWithContext` |

### 16.9 可运行 HTTP 示例

完整 HTTP 服务器，7 种错误场景：

```bash
go run ./examples/errorx-http
```

然后测试：

```bash
curl -i 'http://localhost:18080/skills?id=skill_001'      # 200
curl -i 'http://localhost:18080/skills?id=missing'        # 404 AIHUB_SKILL_NOT_FOUND
curl -i 'http://localhost:18080/skills?id=boom'           # 500 AIHUB_SKILL_QUERY_FAILED
curl -i 'http://localhost:18080/skills?id='               # 400 AIHUB_SKILL_ID_REQUIRED
curl -i -X POST http://localhost:18080/skills -d '{"name":""}'         # 400 AIHUB_SKILL_NAME_REQUIRED
curl -i -X POST http://localhost:18080/skills -d '{"name":"forbidden"}' # 403 AIHUB_SKILL_CREATE_DENIED
curl -i -X POST http://localhost:18080/skills -d '{"name":"demo"}'      # 409 AIHUB_SKILL_ALREADY_EXISTS
```

详见 `examples/errorx-http/README.md`。

### 16.10 如何用 `go doc` 查看 Example

```bash
# 查看包级 Example
go doc errorx

# 查看某个构造器的 Example
go doc errorx.NotFound
go doc errorx.Wrap
go doc errorx.WithMetadata

# 查看某个 inspect 函数的 Example
go doc errorx.CodeOf
go doc errorx.Fields

# 查看所有 Example（运行测试）
go test ./errorx -run=Example -v
```

---

## 应用方法

把上面 `## 16. Examples 索引（按场景查找）` 整段内容追加到 `errorx/README.md` 末尾。
