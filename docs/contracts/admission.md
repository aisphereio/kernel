# Admission 准入契约

Kernel admission 参考 Kubernetes apiserver，把准入分成 mutating 和 validating 两段。

## 1. 链路位置

```text
requestinfo -> authn -> authz/audit -> mutating admission -> validating admission -> business handler
```

## 2. Mutating Admission

用于框架统一默认值和归一化：

- 默认 owner/project/tenant。
- 规范化 labels/tags。
- 默认分页大小。
- 默认 visibility/lifecycle。

## 3. Validating Admission

用于跨资源、跨接口、可复用的策略校验：

- 状态机变更。
- 发布/删除安全检查。
- 租户边界检查。
- 业务不变量校验。

## 4. Agent 规则

Agent 不得把跨接口默认值和策略校验散落到 handler/service。只要规则会被多个接口复用，就应该实现为 `admissionx` 插件，并通过 `autowire.WithAdmission` 挂载。
