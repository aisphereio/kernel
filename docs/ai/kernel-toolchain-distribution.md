# Kernel Toolchain Distribution

Kernel ships two kinds of artifacts:

1. **Runtime libraries** used by services at runtime.
2. **Build-time command tools** used by `make tools`, `make api`, and `make proto-check`.

This distinction is important because generated-code tools are not imported by
business services.

## Build-time tools

The following packages are compiled into binaries:

| Tool | Package | Installed by | Used by | Purpose |
|---|---|---|---|---|
| `protoc-gen-go-http` | `github.com/aisphereio/kernel/cmd/protoc-gen-go-http` | `make tools` | `buf generate` / `make api` | Generate kernel direct HTTP transport bindings. |
| `protoc-gen-go-errors` | `github.com/aisphereio/kernel/cmd/protoc-gen-go-errors` | `make tools` | `buf generate` / `make api` | Generate error contracts. |
| `protoc-gen-go-authz` | `github.com/aisphereio/kernel/cmd/protoc-gen-go-authz` | `make tools` | `buf generate` / `make api` | Generate authz rules, manifest, secure client, and SELF_CHECK guard. |

A service repository normally does not import these packages. It installs the
binaries into local `.bin`:

```bash
make tools
```

Then it runs generation and contract checks:

```bash
make api
make proto-check
```

`layout/Makefile` already installs these tools from the published Kernel module:

```text
GOBIN=.bin go install github.com/aisphereio/kernel/cmd/protoc-gen-go-http@$(KERNEL_VERSION)
GOBIN=.bin go install github.com/aisphereio/kernel/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)
GOBIN=.bin go install github.com/aisphereio/kernel/cmd/protoc-gen-go-authz@$(KERNEL_VERSION)
```

For local Kernel development, the root `Makefile` builds them from local source:

```text
cd cmd/protoc-gen-go-http && GOBIN=.bin go install .
cd cmd/protoc-gen-go-errors && GOBIN=.bin go install .
cd cmd/protoc-gen-go-authz && GOBIN=.bin go install .
```

## Runtime libraries

Business services import runtime packages:

| Package | Runtime purpose |
|---|---|
| `authz` | Guard contract, rule model, resource template resolver, decision model. |
| `contextx` | Request id, trace id, principal, tenant, decision id, scoped/internal token context values. |
| `grpcx` | gRPC server/client interceptors, protovalidate, context propagation, token verification, logging, metrics, recovery. |
| `restx` | HTTP inbound runtime and request validation integration. |
| `grpcgatewayx` | grpc-gateway handler mounting and safe metadata propagation. |

## Contract files

`api/aisphere/options/v1/authz.proto` is not a tool. It is the proto contract
language that lets APIs declare:

```proto
option (aisphere.options.v1.authz) = {
  action: "skill.download"
  resource: "skill:{skill_id}"
  audience: "skill-service"
  mode: SCOPED_TOKEN
};
```

`protoc-gen-go-authz` consumes that option and generates Go code. The relationship is:

```text
api/aisphere/options/v1/authz.proto
  -> defines the annotation syntax

cmd/protoc-gen-go-authz
  -> reads the annotation from descriptors
  -> generates *_authz.pb.go

buf
  -> builds a descriptor set
  -> fails when proto files are invalid or fail buf lint rules
```

## Generated files

For a proto file such as:

```text
api/skill/v1/skill.proto
```

`make api` can generate:

```text
api/skill/v1/skill.pb.go
api/skill/v1/skill_grpc.pb.go
api/skill/v1/skill_http.pb.go
api/skill/v1/skill.gw.pb.go
api/skill/v1/skill_authz.pb.go
docs/openapi/aisphere.swagger.json
```

`*_authz.pb.go` contains:

- `XxxServiceAuthzRules`
- `XxxServiceAuthzManifestJSON`
- `XxxServiceSecureClient`
- `XxxServiceAuthzUnaryServerInterceptor`

## Release rule

When Kernel is released, the command packages must be released with the same
version as the runtime packages. Service templates should pin `KERNEL_VERSION`.

```make
KERNEL_VERSION ?= v0.0.2
```

Because these commands are separate Go modules, each command module needs its
own directory-prefixed tag for the same version:

```bash
git tag v0.1.12
git tag cmd/kernel/v0.1.12
git tag cmd/protoc-gen-go-authz/v0.1.12
git tag cmd/protoc-gen-go-errors/v0.1.12
git tag cmd/protoc-gen-go-http/v0.1.12
git push origin v0.1.12 cmd/kernel/v0.1.12 cmd/protoc-gen-go-authz/v0.1.12 cmd/protoc-gen-go-errors/v0.1.12 cmd/protoc-gen-go-http/v0.1.12
```

Run this before publishing tags to check module paths. After creating local
tags, include `RELEASE_VERSION` to verify the root and command tags exist:

```bash
make release-check
make release-check RELEASE_VERSION=v0.1.12
```

This ensures that:

```text
proto options + code generator + runtime authz/grpcx APIs
```

stay compatible.
