# ServiceModule 自动装载契约

Kernel 的目标是：业务服务只写 proto 和 handler，不手写认证、授权、审计、Gateway 分发和 transport 装配。

## 生成链路

```text
proto google.api.http + aisphere.access.v1.policy
  -> protoc-gen-go-authz
  -> RequestInfoResolver / AccessResolver
  -> protoc-gen-go-gateway
  -> GatewayManifest / GatewayBind / GatewayInvoker
  -> ServiceModule
  -> serverx.BuildService
```

## ServiceModule

`serverx.ServiceModule` 是 `make api` 后生成代码与运行时之间的稳定契约：

```go
type ServiceModule struct {
    Name string
    GatewayManifest gatewayx.Manifest
    RequestInfoResolver requestx.Resolver
    AccessResolver accessmw.Resolver
    RegisterGRPC func(*grpc.Server, any) error
    RegisterHTTP func(*http.Server, any) error
    RegisterGatewayInvokers func(*gatewayx.InvokerRegistry, any) error
}
```

业务启动不再手工传 resolver：

```go
cfg, _ := serverx.LoadConfigFile("configs/services/skill-service.yaml")
app, _ := serverx.BuildService(ctx, cfg, skillv1.SkillServiceKernelModule(), NewSkillService(),
    serverx.WithProviderFactory(platformProviders),
)
return app.Run()
```

## ProviderFactory

`ProviderFactory` 根据配置创建 provider-neutral 的 Kernel provider：

```yaml
security:
  authn:
    provider: casdoor
    required: true
  authz:
    provider: spicedb
    required: true
  audit:
    enabled: true
    provider: memory
```

生产实现：

```text
casdoor client -> authn.Provider
spicedb client -> authz.Provider
audit store -> auditx.Recorder
```

业务 handler 不得直接 import Casdoor SDK 或 SpiceDB/Authzed SDK。

## 权威执行点

Gateway 只做 route match、passive authn 和 token relay；SkillService/IAMService 等业务服务内部的 Kernel server chain 才执行权威 authn/authz/audit。

```text
Gateway
  -> generated gRPC invoker
  -> Service Kernel server chain
  -> authn provider
  -> generated AccessResolver
  -> authz provider
  -> audit recorder
  -> business handler
```
