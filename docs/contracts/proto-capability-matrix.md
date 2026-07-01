# Proto 能力矩阵

当前主线：proto contract 先声明服务、HTTP 绑定、策略元数据，再由 buf-check 和 protoc generators 生成运行时 glue。

已接入能力：

```text
protoc-gen-go
protoc-gen-go-grpc
protoc-gen-go-http
protoc-gen-grpc-gateway
protoc-gen-go-errors
protoc-gen-go-authz
protoc-gen-go-gateway
protoc-gen-go-kernel
protoc-gen-openapiv2
```

运行时对应：

```text
transportx/http
transportx/grpc
errorx
requestx
accessx
gatewayx
serverx
ratelimitx
clientpolicyx
```

状态：

```text
Go DTO: stable
gRPC: stable
HTTP binding: active
OpenAPI: active
errorx helpers: stable
request/access resolver: active
gateway manifest/invoker: active
serverx service module: partial
admission proto option: planned
downstream policy proto option: planned
```

规则：full profile 应展示完整 contract；MVP profile 只用于最小运行验证。生成器缺口优先在 Kernel 修复，不在业务项目长期手写 glue。
