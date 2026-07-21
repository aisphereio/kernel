# kubernetesx Kubernetes 客户端契约

## 1. 定位

`kubernetesx` 是 Kernel 唯一的 Kubernetes 客户端抽象。它把 `controller-runtime` 和 `client-go` 封装成应用层接口，让 Hub 业务层不直接接触 client-go 类型：

```text
HTTP/gRPC Request
  -> authz (Hub)
  -> kubernetesx.Client
  -> Kubernetes API Server
  -> response
```

Kernel 只负责 Kubernetes SDK 表面：REST Config 构造、Scheme 注册、Typed/Unstructured Client、Server-Side Apply、Discovery、Probe、超时/QPS/Burst、日志、指标、错误归一化、Fake Client。

Kernel **不**负责：集群信息持久化、kubeconfig 存储、组织/用户/分享/权限、Cluster/Namespace 产品语义、Hub API。这些归 Hub / IAM / SpiceDB。

## 2. 第一阶段不启动 Manager

Cluster/Namespace CRUD 是请求驱动操作，第一阶段直接使用非缓存 `controller-runtime/client.Client`，不为每个远程集群启动 Manager、Informer 或 Cache：

- 多集群 Manager 生命周期复杂；
- 每个集群均会建立 Watch 和缓存；
- Hub 多副本时会重复 Watch；
- 凭据轮换后需重启 Manager；
- Namespace CRUD 不依赖持续 Reconcile。

周期 Probe 和状态同步通过 `taskx` 完成。后续只有明确需要长期 Watch、WarmPool 或自定义 Controller 时，再增加独立 Controller Runtime 进程。

## 3. 包位置

```text
kernel/
└── kubernetesx/
    ├── doc.go
    ├── config.go
    ├── credential.go
    ├── client.go
    ├── factory.go
    ├── scheme.go
    ├── options.go
    ├── apply.go
    ├── discovery.go
    ├── probe.go
    ├── unstructured.go
    ├── errors.go
    ├── metrics.go
    ├── fake/
    │   └── client.go
    ├── kubernetesx_test.go
    └── contract_test.go
```

`kubernetesx` 是 Kernel 根模块下的 runtime 包；`k8s.io/* v0.36.1` 和 `sigs.k8s.io/controller-runtime v0.24.x` 进根 `go.mod`。这与 `objectstorex`(minio-go 在根)、`taskx`(dapr 在根) 的核心 runtime API 先例一致，使 `Client` 接口可直接暴露 `client.Object` / `*rest.Config` / `*runtime.Scheme` / `dynamic.Interface` / `discovery.DiscoveryInterface`，无需类型重导出层。

依赖锁定：

```text
sigs.k8s.io/controller-runtime v0.24.x
k8s.io/api                       v0.36.x
k8s.io/apimachinery              v0.36.x
k8s.io/client-go                 v0.36.x
```

`controller-runtime v0.24` 与 `k8s.io/* v0.36`、Go 1.26 是官方测试组合。所有 `k8s.io/*` 与 controller-runtime minor 必须一致，禁止混用不同 minor。

## 4. Client 接口

```go
type Client interface {
    Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
    List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
    Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
    Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
    Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
    Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

    Apply(ctx context.Context, obj client.Object, opts ApplyOptions) error
    ApplyUnstructured(ctx context.Context, obj client.Object, opts ApplyOptions) error

    ServerVersion(ctx context.Context) (VersionInfo, error)
    Discover(ctx context.Context) (Capabilities, error)
    Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error)

    Scheme() *runtime.Scheme
    RESTConfig() *rest.Config
    Dynamic() dynamic.Interface
    Discovery() discovery.DiscoveryInterface
}
```

`RESTConfig()`、`Dynamic()` 和 `Discovery()` 是基础设施层 escape hatch：**仅 Hub Data Adapter 可使用**；Hub Biz 层不得依赖这些类型。

构造：

```go
client, err := kubernetesx.New(cfg,
    kubernetesx.WithScheme(agentSandboxScheme),
    kubernetesx.WithAddToScheme(kserveScheme),
)
```

## 5. Scheme

`DefaultScheme()` 默认注册：

- `core/v1`
- `apps/v1`
- `batch/v1`
- `networking.k8s.io/v1`
- `rbac.authorization.k8s.io/v1`
- `apiextensions.k8s.io/v1`

第三方 Scheme 通过 `WithScheme`（替换 base）或 `WithAddToScheme`（叠加）注入。对未引入 Go 类型的第三方 CRD，使用 `unstructured.Unstructured` + GVK。

## 6. Config 与 Credential

```go
type Config struct {
    Host, Kubeconfig []byte, Context string
    QPS, Burst, Timeout, UserAgent, FieldManager
    InsecureSkipVerify bool
    Logger logx.Logger; Metrics metricsx.Manager; MetricsEnabled bool
}

type Credential struct {
    Kind CredentialKind  // KUBECONFIG | IN_CLUSTER | SERVICE_ACCOUNT
    Kubeconfig []byte; Context, Host, Token string; CACert []byte
}
```

`Config.MergeCredential(Credential)` 是 Hub 从存储凭据构造 per-cluster Config 的标准方式。`Credential.Validate()` 强制安全不变量：

- 拒绝 kubeconfig exec plugin；
- 拒绝 kubeconfig 引用外部证书 / token / CA 文件；
- 拒绝 impersonation；
- 拒绝 `file://` URI；
- SERVICE_ACCOUNT 必须有 Host 和 Token。

`*rest.Config` 解析顺序：注入的 `WithRESTConfig` → Kubeconfig 字节（`clientcmd.Load`，不走文件系统）→ Host → in-cluster。

## 7. Server-Side Apply

所有 AISphere 声明式管理的资源优先使用 SSA：

```text
FieldManager: aisphere-hub
```

按控制器拆分：

```text
aisphere-hub-namespace
aisphere-hub-sandbox
aisphere-hub-workload
aisphere-hub-network
```

`ApplyOptions{FieldManager, ForceOwnership, DryRun}`：

1. 只 Apply AISphere 明确拥有的字段；
2. 默认不强占其他 Manager 字段；
3. 对 AISphere 独占资源可显式 `ForceOwnership=true`；
4. 冲突转换为 `KUBERNETES_FIELD_CONFLICT`，不得静默覆盖；
5. 保留 `managedFields`，不手工清除。

实现走 `client.Patch(ctx, obj, client.Apply, FieldOwner(fm), ...)`。

## 8. Probe

```go
type ProbeResult struct {
    Reachable       bool
    ServerVersion   VersionInfo
    ClusterUID      string
    APIs            []APIResource
    NamespaceAccess AccessReview
    Latency         time.Duration
    Warnings        []string
}
```

接入集群时验证：API Server 可达、TLS/CA 正确、凭据有效、能读 ServerVersion、能读 Namespace、能 create/update/delete Namespace（或标记只读）。Cluster UID 取 `kube-system` namespace UID。`AccessReview` 通过 `SelfSubjectAccessReview` 判定。禁止依赖本地 exec credential plugin。

## 9. 错误归一化

```text
KUBERNETES_CONFIG_INVALID
KUBERNETES_CREDENTIAL_INVALID
KUBERNETES_UNAUTHORIZED
KUBERNETES_FORBIDDEN
KUBERNETES_NOT_FOUND
KUBERNETES_ALREADY_EXISTS
KUBERNETES_CONFLICT
KUBERNETES_FIELD_CONFLICT
KUBERNETES_TIMEOUT
KUBERNETES_UNREACHABLE
KUBERNETES_API_UNAVAILABLE
KUBERNETES_OPERATION_FAILED   // 兜底
KUBERNETES_TOO_MANY_REQUESTS
```

`NormalizeError(err)` 把 `apierrors.StatusError`、context 超时、dial/TLS/x509 等错误归一化为 `errorx.Code`，携带 HTTP status 和 retryable。已包装的 errorx 透传。metadata 至少包含 `api_group, kind, namespace, name, reason, retryable`，**绝不**包含 kubeconfig、token、私钥。

## 10. 指标

```text
kernel_kubernetesx_operations_total
kernel_kubernetesx_operation_duration_seconds
kernel_kubernetesx_probe_total
kernel_kubernetesx_probe_duration_seconds
```

`Metrics==nil` 或 `MetricsEnabled==false` 时所有注册/记录为 no-op。观测装饰器（`observedClient`）在后续 PR 加入。

## 11. 测试

- Fake Client（`kubernetesx/fake`）：纯单元测试，CRUD + Apply 幂等 + Probe/Discover 桩；
- envtest（后续 PR）：SSA 字段冲突、CRD、真实 API Server 语义；
- Kind（后续 PR）：真实 API Server 集成；
- Go race（后续 PR）：Client Pool 并发；
- 依赖兼容检查（后续 PR）：所有 `k8s.io/*` 与 controller-runtime minor 一致。

## 12. 禁止

- 业务代码 import `k8s.io/client-go`、`k8s.io/apimachinery`、`sigs.k8s.io/controller-runtime`；
- Hub Biz 层调用 `Client.RESTConfig()` / `Dynamic()` / `Discovery()`；
- 在请求路径手工清除 `managedFields`；
- 把 kubeconfig / token / 私钥写入错误 metadata 或日志；
- 为每个远程集群启动 Manager / Informer / Cache（第一阶段）。

## 13. 后续（不在本 PR）

- metrics `observedClient` 装饰器；
- envtest / Kind / race / 依赖兼容 CI；
- Pod、Job、Generic CRD、Agent Sandbox、OpenSandbox、Exec / Logs / Files / TTL（设计 §11）。
