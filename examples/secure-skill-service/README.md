# Secure Skill Service example

This example is the end-to-end contract-runtime loop for Kernel's proto-first service model.

It demonstrates:

1. `buf.validate` request validation in proto.
2. `aisphere.authz`, `aisphere.audit`, and `aisphere.capability` method options.
3. `make api` generation of gRPC, kernel HTTP, grpc-gateway, OpenAPI, gateway, deploy and authz artifacts.
4. Generated secure client usage for internal service calls.
5. Generated `SELF_CHECK` server guard usage for high-risk resource-service authorization.
6. `grpcx` propagation of request id, trace id, decision id, and internal/scoped token metadata.

## Generate

From the repository root:

```bash
make tools
make api
make deploy
make proto-check
```

Generated files are disposable outputs unless this example explicitly commits a runnable generated package. Do not hand-write long-lived generated shapes in this directory just to make an example compile; fix the Kernel generator instead.

Typical generated outputs include:

```text
examples/secure-skill-service/api/skill/v1/skill.pb.go
examples/secure-skill-service/api/skill/v1/skill_grpc.pb.go
examples/secure-skill-service/api/skill/v1/skill_http.pb.go
examples/secure-skill-service/api/skill/v1/skill.gw.pb.go
examples/secure-skill-service/api/skill/v1/skill_authz.pb.go
deploy/generated/gateway/**
docs/openapi/aisphere.swagger.json
```

## Guard naming rule

Generated authz code may expose a small contract interface for scoped/internal token issuance. That interface is not the request-time access orchestrator.

Use this distinction:

| Name | Meaning |
|---|---|
| generated authz contract guard | client/server token contract required by generated secure-call code |
| `accessx.Guard` | runtime request guard that runs principal resolution, authorization, SkipPolicy and audit |
| `authz.Authorizer` | provider-neutral permission decision interface |

## Runtime flow

```text
workflow-service or gateway
  -> generated secure client
      -> generated authz contract guard for SCOPED_TOKEN methods
      -> contextx.WithScopedToken / contextx.WithAuthzDecisionID
      -> grpcx client interceptor injects metadata
  -> skill-service
      -> grpcx server token verifier validates scoped/internal token
      -> generated SELF_CHECK resolver maps request to accessx.Check
      -> accessx.Guard handles authn + authz + audit
      -> business handler
```

## Packaging rule

`cmd/protoc-gen-go-authz` and `cmd/buf-check-aisphere` are not libraries used by business services. They are command-line tools installed into `.bin` by `make tools` and invoked by `make api` / `make proto-check`.

Business code imports generated API packages and runtime packages such as `accessx`, `authz`, `contextx`, and `grpcx`; it should not import `cmd/...` packages.
