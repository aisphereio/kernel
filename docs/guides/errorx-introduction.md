# errorx 对外介绍稿

## 30 秒介绍

`errorx` 是 Aisphere Kernel 的统一错误语义包。它要求业务错误必须有稳定 `error_code`，并同时携带 `message`、`http_status`、`grpc_code`、`retryable`、`metadata`、`request_id`、`trace_id` 等信息。这样 HTTP 响应、gRPC 响应、日志、审计、指标、Worker 重试都可以从同一个错误对象中提取语义，不再各写各的错误处理逻辑。

## 适合放到 README 的介绍

在 Aisphere Kernel 中，错误不是简单字符串，而是系统契约。

`errorx` 负责定义这套错误契约：

```text
error_code      稳定机器语义，用于前端、日志、审计、指标和告警
message         人类可读文案
http_status     HTTP 响应映射
grpc_code       gRPC 状态映射
retryable       Worker 和外部调用是否可重试
cause           保留底层错误链
metadata        内部排障上下文
public_metadata 可安全返回给前端的上下文
request_id      请求追踪
trace_id        分布式链路追踪
category        错误分类
severity        日志严重级别
stack           可选调用栈
```

业务代码只需要创建或包装 `errorx`：

```go
return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "技能不存在",
    errorx.WithMetadata("skill_id", skillID),
)
```

框架层可以统一消费：

```go
errorx.CodeOf(err)
errorx.HTTPStatusOf(err)
errorx.GRPCCodeOf(err)
errorx.RetryableOf(err)
errorx.Fields(err)
errorx.MetricsLabels(err)
```

这样 `httpx/logx/auditx/metricsx/workerx` 都围绕同一个错误语义工作。

## 核心价值

```text
1. 统一错误码：所有业务错误有稳定 error_code
2. 统一响应：HTTP/gRPC 自动映射
3. 统一日志：logx 自动提取 error_code、status、retryable、trace_id
4. 统一审计：auditx 记录失败原因和错误码
5. 统一指标：metricsx 按低基数 error_code 聚合
6. 统一重试：workerx 根据 retryable 判断是否重试
7. 保留 cause：支持 errors.Is / errors.As
8. 控制暴露：metadata 内部使用，public_metadata 才能返回前端
9. 方便 AI 编码：明确禁止裸 errors.New / fmt.Errorf 作为业务错误
```

## 不做什么

`errorx` 不打印日志，不写 HTTP response，不直接依赖 gRPC，不做审计，不上报指标。

它只定义错误语义，其他模块消费错误语义。
