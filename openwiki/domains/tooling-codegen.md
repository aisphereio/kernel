# Tooling & Code Generation

Aisphere Kernel provides a comprehensive toolchain for **contract-driven code generation**. All transport glue, error handling, authorization resolvers, gateway manifests, and service modules are generated from proto contracts.

## CLI (`cmd/kernel`)

The `kernel` CLI scaffolds new services and provides development utilities:

```bash
kernel new todo-service          # Create new service from template
kernel new todo-service --repo <url>  # Use custom template repo
kernel version                   # Print version
```

- Default template: `github.com/aisphereio/kernel-layout`
- Custom template: `--repo` flag or `KERNEL_LAYOUT` env var
- Feature toggling: `--disable` flag
- Version pinning: `--kernel-version` flag

**Source**: `/cmd/kernel/main.go`, `/cmd/kernel/version.go`, `/cmd/kernel/internal/`

## Protoc Plugins

All protoc plugins are **build-time tools only** — business code must NOT import them.

| Plugin | Generated Output | Source |
|---|---|---|
| `protoc-gen-go-kernel` | `<Service>KernelModule()` — service module registration | `/cmd/protoc-gen-go-kernel/main.go` |
| `protoc-gen-go-http` | HTTP codec (request/response encoding) | `/cmd/protoc-gen-go-http/main.go` |
| `protoc-gen-go-errors` | Error code constants from proto | `/cmd/protoc-gen-go-errors/main.go` |
| `protoc-gen-go-authz` | `RequestInfoResolver` — access policy resolvers | `/cmd/protoc-gen-go-authz/main.go` |
| `protoc-gen-go-gateway` | Gateway Manifest, Binding, Invoker | `/cmd/protoc-gen-go-gateway/main.go` |
| `protoc-gen-go-deploy` | Deploy route configuration | `/cmd/protoc-gen-go-deploy/main.go` |
| `protoc-gen-aisphere-gateway` | Aisphere-specific gateway generation | `/cmd/protoc-gen-aisphere-gateway/main.go` |

### Generation Pipeline

```
proto file
  → protoc + protoc-gen-go-kernel → <Service>KernelModule()
  → protoc + protoc-gen-go-http → HTTP codec
  → protoc + protoc-gen-go-errors → error constants
  → protoc + protoc-gen-go-authz → RequestInfoResolver
  → protoc + protoc-gen-go-gateway → GatewayManifest()
  → protoc + protoc-gen-go-deploy → deploy config
```

## Proto Contract Validation (`cmd/buf-check-aisphere`)

`buf-check-aisphere` is a custom buf plugin that validates proto contracts at compile time:

- Checks that every RPC with `google.api.http` also declares `aisphere.access.v1.policy`
- Validates access policy values (PUBLIC, AUTHENTICATED, AUTHORIZED, INTERNAL)
- Ensures error codes follow naming conventions
- Validates service metadata annotations

**Source**: `/cmd/buf-check-aisphere/`

## Proto API Definitions

| Proto Package | Purpose | Source |
|---|---|---|
| `aisphere.access.v1` | Access policy options | `/api/aisphere/access/v1/access.proto` |
| `aisphere.options.v1` | Kernel-specific proto options | `/api/aisphere/options/` |

### Access Policy Annotation

```protobuf
import "aisphere/access/v1/access.proto";

service TodoService {
    rpc CreateTodo(CreateTodoRequest) returns (CreateTodoResponse) {
        option (google.api.http) = {
            post: "/v1/todos"
            body: "*"
        };
        option (aisphere.access.v1.policy) = {
            auth_mode: AUTHZ
        };
    }
}
```

## Proto Options Parsing

The `internal/protooptions` package parses proto options for code generators:

| Function | Purpose | Source |
|---|---|---|
| `ParseAccessPolicy` | Parse `aisphere.access.v1.policy` from proto | `/internal/protooptions/options.go` |
| `ParseGatewayPublish` | Parse gateway publication options | `/internal/protooptions/options.go` |

## External Template Repository

The **kernel-layout** repository (`https://github.com/aisphereio/kernel-layout`) contains:

- Generated service project templates
- Service Makefiles (`make deploy`, `make run`, etc.)
- Deploy manifest templates
- Layout documentation
- Smoke tests for generated services

This separation keeps the Kernel repository focused on runtime packages and tooling.

## Toolchain Distribution

Two artifact types:

| Type | Description | Examples |
|---|---|---|
| **Runtime libraries** | Used by services at runtime | `authz`, `contextx`, `grpcx`, `restx`, `grpcgatewayx` |
| **Build-time commands** | Used by `make tools`, `make api`, `make proto-check` | `protoc-gen-go-http`, `protoc-gen-go-errors`, `protoc-gen-go-authz` |

**Release rule**: Command packages and runtime packages are released with the same version from the root Go module (no nested `go.mod` files).

## Development Workflow

```bash
# 1. Install toolchain
make tools

# 2. Write proto contract
#    (edit api/your-service/v1/*.proto)

# 3. Generate code
make api

# 4. Validate contracts
make proto-check

# 5. Implement business logic
# 6. Run verification
make verify
```

## Related Documentation

- [Quickstart](../quickstart.md)
- [Architecture Overview](../architecture/overview.md)
- [Gateway Architecture](../architecture/gateway.md)
- [docs/ai/kernel-toolchain-distribution.md](../../docs/ai/kernel-toolchain-distribution.md)
- [docs/ai/kernel-development-paradigm.md](../../docs/ai/kernel-development-paradigm.md)
- [docs/getting-started.md](../../docs/getting-started.md)