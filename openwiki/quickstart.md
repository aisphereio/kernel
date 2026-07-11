# Aisphere Kernel — Quickstart

**Aisphere Kernel** is a specification-driven, AI-native Go microservice foundation framework. It follows a **contract → check → generate → assemble → govern → verify** development paradigm inspired by the Kubernetes apiserver design.

## Core Principle

> **Business declares contracts; Kernel handles checking, generation, assembly, governance, and verification.**

Business projects write proto contracts and domain logic. Kernel tools generate transport glue, error handling, authorization resolvers, gateway manifests, and service modules. Kernel runtime packages provide fixed infrastructure for authentication, authorization, audit, rate limiting, admission, observability, and error semantics.

## Quick Start

```bash
go install github.com/aisphereio/kernel/cmd/kernel@latest
kernel version
kernel new todo-service
```

`kernel new` scaffolds a new service from the [kernel-layout](https://github.com/aisphereio/kernel-layout) template repository.

### Local Verification

```bash
make tools       # Install protoc-gen-* plugins
make api         # Generate code from proto
make proto-check # Validate proto contracts
make verify      # Run all checks and tests
```

## Architecture at a Glance

```
Proto Contract
  → buf-check-aisphere (contract validation)
  → protoc-gen-* (code generation)
  → serverx + autowire (service assembly)
  → middleware chain: requestinfo → authn → accessx → auditx → admissionx → business handler
  → errorx response
```

- **Edge**: Envoy Gateway handles OIDC/JWT verification, header sanitization, and route matching.
- **Identity**: Casdoor is the sole OIDC provider and identity directory.
- **Authorization**: SpiceDB (ReBAC) via provider-neutral `authz` interfaces.
- **Access Orchestration**: `accessx` combines authn + authz + audit in a single guard.
- **Gateway**: Proto-annotated routes generate Envoy Gateway manifests via `gatewayx`.

## Package Index

| Category | Packages | Status |
|---|---|---|
| **Infrastructure** | `errorx`, `logx`, `configx`, `metricsx`, `contextx` | Stable |
| **Service Assembly** | `serverx`, `bootx`, `securityx` | Active |
| **Transport** | `transportx/http`, `transportx/grpc`, `requestx` | Stable |
| **Security** | `authn`, `authz`, `accessx`, `auditx` | Active |
| **Gateway** | `gatewayx`, `admissionx`, `ratelimitx`, `clientpolicyx` | Active |
| **Data** | `dbx`, `migrationx`, `dbrepo`, `cachex`, `objectstorex`, `dtmx` | Active |
| **Service Discovery** | `registry`, `selectorx` | Stable |
| **Encoding** | `encodingx` | Stable |
| **Tooling** | `cmd/kernel`, `cmd/protoc-gen-*`, `cmd/buf-check-aisphere` | Active |

## Key Design Decisions

| Decision | Choice |
|---|---|
| Identity Provider | Casdoor (OAuth/OIDC, user/org/group management) |
| Authorization Engine | SpiceDB (ReBAC/ABAC) |
| Edge AuthN | Envoy Gateway (OIDC/JWT verification, header sanitization) |
| Backend AuthN Mode | `gateway_trusted` (trust Envoy-injected headers + internal token) |
| Framework Pattern | Kubernetes apiserver-inspired (contract → codegen → middleware chain → business) |
| Configuration | `configx` (file + env + remote, hot-reloadable) |
| Logging | `logx` (structured, context-aware, slog-based) |
| Error Handling | `errorx` (structured, cross-protocol, with error codes) |
| Database | `dbx` (GORM wrapper with built-in patterns) |
| Distributed Transactions | `dtmx` (DTM Saga) |
| Provider Neutrality | Business code depends on Kernel interfaces only, never provider SDKs |

## Wiki Navigation

- **[Architecture Overview](architecture/overview.md)** — Contract-driven development, K8s apiserver pattern, code generation pipeline
- **[Security Architecture](architecture/security.md)** — AuthN/AuthZ/Audit, Envoy Gateway + Casdoor OIDC, internal service tokens
- **[Gateway Architecture](architecture/gateway.md)** — Envoy Gateway routing, route generation, publication policy
- **[Core Runtime Packages](domains/core-packages.md)** — App lifecycle, serverx, transportx, middleware, contextx, errorx, logx, metricsx, configx
- **[Data & Storage](domains/data-storage.md)** — Database, caching, object storage, distributed transactions
- **[Tooling & Code Generation](domains/tooling-codegen.md)** — CLI, protoc plugins, buf-check, kernel-layout
- **[Testing & Operations](operations/testing.md)** — Testing strategy, acceptance checklists, CI pipeline

## Important Rules

- **Business code must NOT** import provider SDKs (Casdoor, SpiceDB, etc.), use raw `errors.New`/`fmt.Errorf`, `log.Printf`/`slog`/`zap`, `os.Getenv`, `database/sql`, or `grpc.Dial`/`http.Client` directly.
- **Every RPC must declare an access policy** via `aisphere.access.v1.policy` proto annotation.
- **System routes** are managed by `serverx`, not in business proto.
- **Documentation** is maintained in Chinese; contract changes must sync `docs/contracts/*.md`.

## External Repositories

- [kernel-layout](https://github.com/aisphereio/kernel-layout) — Generated service templates, Makefiles, deploy manifests, smoke tests