# Architecture Overview

Aisphere Kernel is a **specification-driven microservice framework** inspired by the Kubernetes apiserver design pattern. It enforces a strict development pipeline where business logic is expressed as proto contracts, and all infrastructure glue is generated or assembled automatically.

## Core Development Paradigm

```
Proto Contract
  → buf-check-aisphere (compile-time validation)
  → protoc-gen-* plugins (code generation)
  → serverx + autowire (runtime assembly)
  → middleware chain (governance)
  → business handler
  → errorx response
```

### The 11-Step Development Flow

1. Write proto contract (service, messages, `google.api.http`, `aisphere.access.v1.policy`)
2. Run `make api` to generate code
3. Run `make proto-check` to validate contracts
4. Implement generated service interface
5. Register via `serverx.ServiceModule`
6. Configure `securityx` for authn/authz/audit
7. Write admission plugins (optional)
8. Write targeted tests
9. Run `make verify`
10. Run `make deploy` (generated from kernel-layout)
11. CI runs full governance tests

## K8s Apiserver Pattern Mapping

| Kubernetes Apiserver | Aisphere Kernel |
|---|---|
| RequestInfo | `requestx.Info` |
| Filters | `middleware/autowire` |
| Authn Chain | `middleware/authn` |
| Authorization | `accessx.Check` |
| Flow Control | `ratelimitx` |
| Admission | `admissionx` |
| Audit | `auditx` |
| Negotiated Status/Error | `errorx` |

## Standard Inbound Chain

```
Client Request
  → Envoy Gateway (OIDC/JWT verify, header sanitize)
  → Backend Service
    → requestx.Info (normalize request metadata)
    → authn (extract/verify identity)
    → ratelimitx (rate limit check)
    → accessx (authn + authz + audit orchestration)
    → admissionx (mutating/validating plugins)
    → Business Handler
    → errorx (structured error response)
```

## Standard Outbound Chain

```
Business Handler
  → clientpolicyx (downstream policy)
  → serverx/client factory
  → autowire.Client
  → requestx.Info (propagate metadata)
  → timeout
  → client rate limit
  → circuit breaker
  → retry
  → errorx decode
```

## Provider Neutrality

Kernel defines **provider-neutral interfaces** in each domain. Concrete implementations live in sub-packages:

| Interface Package | Default Provider | Sub-package |
|---|---|---|
| `authn` | Casdoor | `authn/casdoor` |
| `authz` | SpiceDB | `authz/spicedb` |
| `cachex` | Redis | `cachex/redis` |
| `dbx` | GORM (Postgres/MySQL) | `dbx/postgres`, `dbx/mysql` |
| `objectstorex` | MinIO | `objectstorex/minio` |
| `dtmx` | DTM | `dtmx/dtm` |

Business code must **only import the interface packages** — never the provider SDKs.

## Package Layering

```
┌─────────────────────────────────────────────┐
│              Business Services               │
├─────────────────────────────────────────────┤
│  serverx  │  gatewayx  │  accessx  │  iamx  │
├───────────┴────────────┴───────────┴────────┤
│  authn  │  authz  │  auditx  │  admissionx  │
├───────────┴────────────┴───────────┴────────┤
│  ratelimitx  │  clientpolicyx  │  bootx     │
├─────────────────────────────────────────────┤
│  contextx  │  requestx  │  servicecontextx  │
├─────────────────────────────────────────────┤
│  errorx  │  logx  │  metricsx  │  configx   │
├─────────────────────────────────────────────┤
│  dbx  │  cachex  │  objectstorex  │  dtmx   │
├─────────────────────────────────────────────┤
│  registry  │  selectorx  │  encodingx       │
├─────────────────────────────────────────────┤
│  transportx/http  │  transportx/grpc        │
└─────────────────────────────────────────────┘
```

## Key Source Files

| File | Purpose |
|---|---|
| `/app.go` | Application lifecycle manager (`App` struct) |
| `/options.go` | Functional options for App configuration |
| `/doc.go` | Package documentation and surface overview |
| `/serverx/serverx.go` | Service assembly and module registration |
| `/middleware/autowire/autowire.go` | Automatic middleware chain assembly |
| `/securityx/config.go` | Security configuration model |
| `/bootx/validation.go` | Startup governance validation |

## Related Documentation

- [Quickstart](../quickstart.md)
- [Security Architecture](security.md)
- [Gateway Architecture](gateway.md)
- [Core Packages](../domains/core-packages.md)
- [docs/architecture/kernel-boundary.md](../../docs/architecture/kernel-boundary.md)
- [docs/ai/kernel-development-paradigm.md](../../docs/ai/kernel-development-paradigm.md)
- [docs/design/k8s-apiserver-lessons.md](../../docs/design/k8s-apiserver-lessons.md)