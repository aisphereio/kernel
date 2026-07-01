# Kernel 边界与分层

Kernel 的目标不是提供一堆工具包，而是提供一套可检查、可生成、可装配、可验证的开发框架。

## 1. 边界原则

Kernel 负责：

- API 契约：proto option、access policy、error code、request metadata。
- 编译期检查：buf-check-aisphere、生成器校验、规范约束。
- 代码生成：HTTP/gRPC glue、AccessResolver、RequestInfoResolver。
- 运行时装配：serverx、autowire、requestx、admissionx、errorx。
- 服务治理：ctx/trace/request-id、authn、authz、audit、ratelimit、timeout、retry、breaker、metrics。
- 系统路由：`/healthz`、`/readyz`、`/version`、`/metrics`。
- 启动校验：provider 缺失、多副本限流误用、内部服务认证缺失。

业务组件负责：

- 声明 proto contract。
- 实现领域 service。
- 通过 serverx 注册服务。
- 通过 Kernel client factory 调用下游。
- 编写业务测试和场景验证。

业务组件不应该：

- 自己解析 HTTP path 或 gRPC full method。
- 自己手写 authn/authz/audit/ratelimit/retry/breaker/timeout wiring。
- 自己挂载 healthz/readyz/version/metrics。
- 自己裸用 `grpc.Dial`、`http.Client`、Redis limiter、Casdoor SDK、Casbin enforcer。

## 2. 推荐分层

```text
api/                 契约和 proto option
cmd/protoc-gen-*     生成器
cmd/buf-check-*      规范检查
requestx/            请求元信息中心
admissionx/          准入链
middleware/          中间件实现
serverx/             服务装配入口
bootx/               启动治理校验
ratelimitx/          限流 provider 抽象
clientpolicyx/       服务间调用策略
errorx/logx/...      基础能力
validation/          验证业务代码，不属于 Kernel 核心
layout/              项目模板和生成物
```

## 3. 请求进入后的标准链路

```text
proto contract
  -> buf-check
  -> codegen
  -> serverx
  -> autowire.Server
  -> requestx.Info
  -> authn / ratelimit / authz / audit
  -> admissionx
  -> business service
  -> errorx negotiated response
```

## 4. 服务间调用标准链路

```text
DownstreamPolicy
  -> serverx/client factory
  -> autowire.Client
  -> requestx.Info client direction
  -> metadata propagation
  -> timeout
  -> client rate limit
  -> circuit breaker
  -> retry
  -> errorx decode
```

## 5. 验证代码边界

`validation/` 用来验证 Kernel 范式，不参与 Kernel 核心 API 设计。比如 IAM + Gateway mock demo 放在：

```text
validation/iamgateway
```

它可以依赖 Kernel 的所有公开 API，但 Kernel 核心包不能依赖 validation 代码。
