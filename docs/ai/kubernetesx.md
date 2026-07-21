# kubernetesx — AI 编码指南

> AI 写 Aisphere Kernel / Hub 业务代码时的 Kubernetes 客户端规范。**只看本文件即可写对所有集群操作场景。**

---

## 0. 一句话规则

> 业务代码（handler/service/repository/worker）操作 Kubernetes 时，**必须**使用 `github.com/aisphereio/kernel/kubernetesx` 的 `Client` 接口。
> **禁止** import `k8s.io/client-go`、`k8s.io/apimachinery`、`sigs.k8s.io/controller-runtime`。Hub Biz 层**禁止**调用 `Client.RESTConfig()` / `Dynamic()` / `Discovery()`（仅 Hub Data Adapter 可用）。

---

## 1. 速查：什么场景用什么 API

| 业务场景 | API | 何时使用 |
|---|---|---|
| 构造客户端 | `kubernetesx.New(cfg, opts...)` | 启动期 / per-cluster |
| 从存储凭据构造 Config | `cfg.MergeCredential(cred)` | Hub Cluster 接入 |
| 读单个对象 | `client.Get(ctx, key, obj)` | 命名空间、Pod 查询 |
| 列对象 | `client.List(ctx, list, opts...)` | 分页、过滤 |
| 创建 | `client.Create(ctx, obj)` | 显式 create |
| 声明式管理 | `client.Apply(ctx, obj, ApplyOptions{FieldManager: ...})` | **AISphere 管理的资源首选** |
| 第三方 CRD（无 Go 类型） | `client.ApplyUnstructured(ctx, u, opts)` | GVK + unstructured |
| 删除 | `client.Delete(ctx, obj)` | 收尾 |
| 接入前健康检查 | `client.Probe(ctx, ProbeRequest{})` | Cluster onboarding |
| 能力发现 | `client.Discover(ctx)` | 判断是否支持某 CRD |
| 服务器版本 | `client.ServerVersion(ctx)` | 兼容性判断 |
| 错误判定 | `errorx.CodeOf(err) == kubernetesx.CodeNotFound` | **不要** `errors.Is(err, kubernetesx.ErrXxx)`（cause 是 apierrors） |
| 单测 | `fake.NewClient(fake.WithObjects(...))` | 无 API Server |

**关键原则**：AISphere 声明式管理的资源用 `Apply`（SSA），不用 `Create`/`Update`。每类资源用自己的 FieldManager。

---

## 2. 标准食谱

### 2.1 从 Credential 构造 Client

```go
import "github.com/aisphereio/kernel/kubernetesx"

cred := kubernetesx.Credential{Kind: kubernetesx.CredentialKindKubeconfig, Kubeconfig: bytes}
if err := cred.Validate(); err != nil { return err }

cfg, err := kubernetesx.Config{FieldManager: "aisphere-hub"}.MergeCredential(cred)
if err != nil { return err }
cfg.Logger = logx.DefaultLogger()
cfg.Metrics = metricsx.Noop()

client, err := kubernetesx.New(cfg)
if err != nil { return err }

result, err := client.Probe(ctx, kubernetesx.ProbeRequest{})  // 接入前验证
```

### 2.2 SSA 创建 Namespace

```go
ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
err := client.Apply(ctx, ns, kubernetesx.ApplyOptions{
    FieldManager: "aisphere-hub-namespace",
})
```

幂等：同一 FieldManager 重复 Apply 不报错。字段冲突报 `KUBERNETES_FIELD_CONFLICT`，**不要**静默 `ForceOwnership` 覆盖其他 Manager 字段。

### 2.3 第三方 CRD（unstructured）

```go
u := kubernetesx.NewUnstructured(
    schema.GroupVersionKind{Group: "serving.kserve.io", Version: "v1beta1", Kind: "InferenceService"},
    "demo", "isvc-1",
)
err := client.ApplyUnstructured(ctx, u, kubernetesx.ApplyOptions{FieldManager: "aisphere-hub-workload"})
```

### 2.4 错误处理

```go
err := client.Get(ctx, key, obj)
switch errorx.CodeOf(err) {
case kubernetesx.CodeNotFound:
    // 404
case kubernetesx.CodeForbidden:
    // 403 — 凭据权限不足
case kubernetesx.CodeFieldConflict:
    // SSA 字段冲突 — 不要自动 force，需业务决策
case errorx.CodeOK:
    // 成功（err == nil）
default:
    // 其他
}
```

**禁止** `errors.Is(err, kubernetesx.ErrNotFound)`：`NormalizeError` 包装的 cause 是 `*apierrors.StatusError`，不是 sentinel。用 `errorx.CodeOf`。

### 2.5 单元测试

```go
import "github.com/aisphereio/kernel/kubernetesx/fake"

c := fake.NewClient(
    fake.WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}),
    fake.WithProbeResult(kubernetesx.ProbeResult{Reachable: true}),
)
```

Fake client 的 SSA 路径要求对象带 GVK（真实 API server 在 accept 时填充）。Apply 前显式 `obj.SetGroupVersionKind(gvk)`；若 Apply 会重新序列化对象（清空 GVK），re-apply 前重设。

---

## 3. 禁止清单

- ❌ 业务代码 import `k8s.io/*` 或 `sigs.k8s.io/controller-runtime`；
- ❌ Hub Biz 层调用 `RESTConfig()` / `Dynamic()` / `Discovery()`；
- ❌ 请求路径手工清除 `managedFields`；
- ❌ kubeconfig / token / 私钥写入错误 metadata 或日志；
- ❌ 为每个远程集群启动 Manager / Informer / Cache（第一阶段）；
- ❌ 用 `errors.Is` 判定归一化后的错误码（用 `errorx.CodeOf`）；
- ❌ 对字段冲突静默 `ForceOwnership` 覆盖其他 Manager 字段。

---

## 4. 主线文档

- [docs/contracts/kubernetesx.md](../contracts/kubernetesx.md) — 完整契约
- 六阶段设计文档 `docs/ai/kubernetes-environment-management-design.md` 位于 `aisphere-hub` 仓库（Kernel 本 PR 只实现第一阶段 Kernel SDK）
