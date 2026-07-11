# Core Runtime Packages

This page covers the foundational runtime packages that every Aisphere Kernel service uses. These packages provide infrastructure for application lifecycle, service assembly, transport, middleware, context, errors, logging, metrics, configuration, and security.

## Application Lifecycle (`kernel` package)

The root `github.com/aisphereio/kernel` package manages the application lifecycle.

| Component | Purpose | Source |
|---|---|---|
| `App` | Application lifecycle manager (start/stop, signal handling, errgroup) | `/app.go` |
| `AppInfo` interface | ID, Name, Version, Metadata, Endpoint | `/app.go` |
| `Option` | Functional options pattern | `/options.go` |
| `NewApp` | Create a new App with options | `/app.go` |

```go
app := kernel.NewApp(
    kernel.Name("my-service"),
    kernel.Version("v1.0.0"),
    kernel.WithTransport(grpcServer, httpServer),
)
app.Run()
```

**Source files**: `/app.go`, `/options.go`, `/app_metrics.go`, `/app_observability.go`, `/app_dtm.go`, `/metrics_server.go`, `/doc.go`

## Service Assembly (`serverx`)

`serverx` is the **opinionated service assembly layer** — the single entry point for configuring and starting a service.

| Component | Purpose | Source |
|---|---|---|
| `Config` | Service configuration | `/serverx/serverx.go` |
| `ServiceModule` | Generated service contract (from protoc-gen-go-kernel) | `/serverx/module.go` |
| `RuntimeProviders` | Wiring contract for providers | `/serverx/providers.go` |
| `Catalog` | Module registry | `/serverx/catalog.go` |
| `ConfigLoader` | Configuration loading | `/serverx/config_loader.go` |
| `RegisterGatewayRoutes` | Gateway route registration | `/serverx/gateway_routes.go` |

**Key rule**: Business code registers via `serverx.ServiceModule` and uses `serverx` for all transport and middleware assembly. Do not hand-wire middleware chains.

## Transport (`transportx`)

| Package | Purpose | Source |
|---|---|---|
| `transportx` | Transport interfaces (Server, Endpointer, Header, Transporter) | `/transportx/transport.go` |
| `transportx/grpc` | gRPC server/client, interceptors, load balancing, errorx mapping | `/transportx/grpc/` |
| `transportx/http` | HTTP server/client, routing, codec, CORS, streaming | `/transportx/http/` |
| `grpcx` | gRPC runtime helpers (default middleware chain) | `/grpcx/` |
| `restx` | HTTP/REST runtime helpers (filters, CSRF, validation) | `/restx/` |
| `grpcgatewayx` | gRPC-gateway HTTP↔gRPC transcoding | `/grpcgatewayx/` |

## Middleware Chain (`middleware/`)

The middleware chain is **automatically assembled** by `middleware/autowire` from `RuntimeProviders`.

| Middleware | Purpose | Source |
|---|---|---|
| `autowire` | Automatic middleware chain assembly | `/middleware/autowire/autowire.go` |
| `authn` | Authentication (extract credentials, call Authenticator) | `/middleware/authn/authn.go` |
| `access` | Access control (authn + authz + audit orchestration) | `/middleware/access/access.go` |
| `logging` | Request logging | `/middleware/logging/logging.go` |
| `metadata` | Metadata propagation | `/middleware/metadata/metadata.go` |
| `validate` | Request validation | `/middleware/validate/validate.go` |
| `retry` | Retry logic | `/middleware/retry/retry.go` |
| `timeout` | Timeout handling | `/middleware/timeout/timeout.go` |
| `circuitbreaker` | Circuit breaker | `/middleware/circuitbreaker/circuitbreaker.go` |
| `recovery` | Panic recovery | `/middleware/recovery/recovery.go` |
| `requestinfo` | Request info extraction | `/middleware/requestinfo/requestinfo.go` |
| `ctxinject` | Context value injection | `/middleware/ctxinject/ctxinject.go` |
| `selector` | Route/operation-based middleware selection | `/middleware/selector/selector.go` |

## Request Context (`contextx`)

`contextx` provides request-scoped value injection and extraction. All `XxxFromContext` functions are **nil-safe** (return zero values, never panic).

| Function | Purpose |
|---|---|
| `WithRequestID` / `RequestIDFromContext` | Request ID propagation |
| `WithTraceID` / `TraceIDFromContext` | Trace ID propagation |
| `WithPrincipal` / `PrincipalFromContext` | Lossless mirror of `authn.Principal` |
| `WithTenant` / `TenantFromContext` | Tenant ID |
| `WithLogger` / `LoggerFromContext` | Request-scoped logger |
| `WithServiceContext` / `ServiceContextFromContext` | Service context |
| `WithAuthnPrincipal` | Authn principal (lossless mirror) |
| `WithAuthzDecisionID` | Authz decision tracking |
| `WithScopedToken` | Scoped token |
| `Merge` | Merge two contexts |

**Tenant resolution priority**: explicit `WithTenant` > `Principal.TenantID` > `Principal.OrgID` (Casdoor owner fallback) > `""`.

**Source**: `/contextx/context.go`, `/contextx/authn.go`, `/contextx/security.go`, `/contextx/service_context.go`, `/contextx/merge.go`

## Request Metadata (`requestx`)

`requestx.Info` is the canonical request metadata model, inspired by Kubernetes apiserver RequestInfo:

```go
type Info struct {
    Protocol    string // HTTP, gRPC
    Direction   string // inbound, outbound
    Method      string // GET, POST, etc.
    Path        string // /api/v1/...
    Operation   string // Full RPC name
    Resource    string // Resource type
    SubResource string // Sub-resource
    ...
}
```

**Source**: `/requestx/info.go`

## Error Handling (`errorx`)

`errorx` provides structured, cross-protocol error semantics. **Business code must use `errorx` for all business/API errors.**

| Constructor | HTTP Status | gRPC Code |
|---|---|---|
| `BadRequest` | 400 | InvalidArgument |
| `Unauthorized` | 401 | Unauthenticated |
| `Forbidden` | 403 | PermissionDenied |
| `NotFound` | 404 | NotFound |
| `Conflict` | 409 | AlreadyExists |
| `RequestTimeout` | 408 | DeadlineExceeded |
| `TooManyRequests` | 429 | Unavailable |
| `ClientClosed` | 499 | Canceled |
| `Internal` | 500 | Internal |
| `Unavailable` | 503 | Unavailable |
| `Timeout` | 504 | DeadlineExceeded |

**Error code naming convention**: `{DOMAIN}_{RESOURCE}_{REASON}` (e.g., `AIHUB_SKILL_NOT_FOUND`).

**Source**: `/errorx/error.go`, `/errorx/constructors.go`, `/errorx/code.go`, `/errorx/category.go`, `/errorx/severity.go`, `/errorx/fields.go`, `/errorx/grpc.go`, `/errorx/http.go`

## Logging (`logx`)

`logx` provides structured, context-aware logging built on `log/slog`.

**Key rules**:
- Business code uses `ctx.Log().Info(...)` or `app.Logger().Info(...)`
- Logs auto-carry context fields (request_id, trace_id, service, user_id, org_id)
- Forbidden: `log.Printf`, `fmt.Println`, `slog`, `zap`, `logrus`

**Source**: `/logx/log.go`, `/logx/loggerx.go`, `/logx/builder.go`, `/logx/config.go`, `/logx/field.go`, `/logx/context.go`, `/logx/access.go`, `/logx/rpc.go`

## Metrics (`metricsx`)

`metricsx` wraps OTel metrics with a simplified interface:

| Type | Purpose |
|---|---|
| `Counter` | Count events |
| `Histogram` | Measure distributions |
| `Gauge` | Track current values |
| `UpDownCounter` | Track values that go up and down |

**Label cardinality rules**: Low-cardinality labels allowed (method, status, tenant, operation, error_code). High-cardinality labels forbidden (request_id, user_id, skill_id, trace_id).

**Source**: `/metricsx/manager.go`, `/metricsx/handler.go`, `/metricsx/context.go`, `/metricsx/store.go`

## Configuration (`configx`)

`configx` provides unified runtime configuration from file, env, and remote sources with hot-reload support.

**Key rules**:
- Business code uses `configx.Get`, `configx.Scan`, `configx.Watch`
- Forbidden: `os.Getenv`, `yaml.Unmarshal`, `json.Unmarshal`, `viper.Get`

**Source**: `/configx/config.go`, `/configx/options.go`, `/configx/reader.go`, `/configx/value.go`, `/configx/source.go`, `/configx/merge.go`

## Security Runtime (`securityx`)

`securityx` constructs the provider-neutral security runtime from configuration:

| Component | Purpose | Source |
|---|---|---|
| `Config` | Security configuration (authn, internal_call) | `/securityx/config.go` |
| `Runtime` | Constructed runtime (AuthnBoundaryRuntime, SkipPolicyResolver) | `/securityx/runtime.go` |
| `AuthnBoundaryConfig` | Authentication boundary configuration | `/securityx/authn_boundary.go` |

## Startup Validation (`bootx`)

`bootx` validates deployment topology, rate limits, downstream policies, and authn/authz/audit configuration **before** the service starts serving.

**Source**: `/bootx/validation.go`

## Related Documentation

- [Quickstart](../quickstart.md)
- [Architecture Overview](../architecture/overview.md)
- [Data & Storage](data-storage.md)
- [docs/ai/errorx.md](../../docs/ai/errorx.md)
- [docs/ai/logx.md](../../docs/ai/logx.md)
- [docs/ai/configx.md](../../docs/ai/configx.md)
- [docs/ai/contextx.md](../../docs/ai/contextx.md)
- [docs/ai/metricsx.md](../../docs/ai/metricsx.md)
- [docs/ai/dbx.md](../../docs/ai/dbx.md)
- [docs/ai/dtmx.md](../../docs/ai/dtmx.md)