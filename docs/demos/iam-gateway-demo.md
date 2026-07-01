# IAM + Gateway 治理链路验证

验证代码位置：

```text
validation/iamgateway
```

它不属于 Kernel 核心包，而是一个业务形态的 mock 集成测试，用来验证 Kernel 是否真的支持 Gateway 和 IAM 这类组件的开发范式。

## 验证目标

```text
外部请求
  -> gateway server chain
  -> requestx.Info
  -> admissionx
  -> gateway handler
  -> Kernel-governed downstream IAM client
  -> IAM mock
```

覆盖能力：

- Gateway 入口生成 `requestx.Info`。
- Admission 默认补 tenant。
- Admission 拒绝 forbidden path。
- Gateway 调 IAM 必须走 `autowire.Client`。
- DownstreamPolicy 校验 target、timeout、rate_limit、retry、service_auth。
- request_id / trace_id / tenant 透传到 IAM mock。
- 服务间 client-side rate limit 在本地拒绝第二次请求，不打到 IAM。

## 运行

```bash
go test ./validation/iamgateway
```

如果本地沙箱因为依赖或编译时间限制失败，不要绕过 Kernel；参考：

```text
docs/ai/github-actions-build-delegation.md
```
