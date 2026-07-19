# 原生 HTTP 协议接入契约

## 适用范围

普通管理 API 必须继续使用 protobuf、`google.api.http`、access policy 和 Kernel generator。只有 Git Smart HTTP、Git LFS、OCI Registry 等已经存在且必须保留原始 wire protocol 的接口，才使用原生 HTTP 协议接入。

## 标准链路

```text
HTTP route
  -> ProtocolDescriptor
  -> ProtocolRequest(operation + structured payload)
  -> requestinfo
  -> authn
  -> rate limit
  -> access/authz/audit
  -> admission
  -> native http.Handler
```

协议 handler 必须通过 `HandleProtocol` 或 `HandleProtocolPrefix` 注册。直接使用 `Handle`/`HandlePrefix` 不会进入 service middleware，只允许用于 health、debug 等已经由其他边界治理的基础设施路由。

## Descriptor 约束

`ProtocolDescriptor` 位于 transport adapter 边界，可以读取 HTTP method、path、query 和有限大小的协议元数据，并将它们转换成结构化 payload。业务 resolver 和 service 不得再次解析 raw path。

```go
type GitRequest struct {
	Repository string
	Action     string
}

func describeGit(r *http.Request) (khttp.ProtocolRequest, error) {
	return khttp.ProtocolRequest{
		Operation: "/git.v1.Protocol/Fetch",
		Payload: GitRequest{
			Repository: "example",
			Action:     "view",
		},
		Request: r,
	}, nil
}
```

`Operation` 必须稳定，供 selector、request info、access resolver、指标和审计使用；不能直接使用带仓库名等高基数字段的 URL。

## 流式约束

- Git pack、OCI blob 等大请求体不得在 Kernel 中读取或缓冲。
- terminal handler 收到 middleware 注入后的 context 和原始 request body。
- response writer 保留 `Flush`、streaming 和取消语义。
- descriptor 失败或 access middleware 拒绝时，native handler 不得执行。
- 如果某个协议的控制消息必须分类，可以限制读取大小并恢复 `request.Body`；不得对大对象数据使用该做法。

## 与 protobuf 的边界

协议数据面不伪装成 protobuf RPC。资源创建、元数据、权限关系、工作流和审计查询等管理面仍然 proto-first。Gateway 对管理面使用 generated Route Manifest；原生协议只发布必要的固定 prefix，并在后端再次执行完整 access guard。

