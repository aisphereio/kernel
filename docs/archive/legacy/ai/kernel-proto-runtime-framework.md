# Kernel proto-first runtime framework

This document describes the recommended development model after integrating
`grpcx`, Buf Protovalidate, grpc-gateway/OpenAPI, and `protoc-gen-go-authz`.

## 1. Architecture boundary

```text
Traefik / APISIX / LB
  Edge traffic gateway: TLS, routing, WAF, coarse limits, access logs.

Aisphere Gateway / BFF
  Business entrypoint: session, CSRF, principal, IAM authz, capability, audit,
  HTTP/JSON -> gRPC, internal/scoped token creation.

IAM
  Policy decision point: Check, LookupResources, IssueScopedToken,
  VerifyDecision, audit persistence.

Business services
  gRPC first. Services verify internal/scoped tokens, restore contextx, validate
  requests, enforce high-risk self-checks, and run business logic.
```

Gateway is **not** an internal service bus. Internal services call each other
with gRPC/HTTP clients directly, carrying context through interceptors.

## 2. Context propagation contract

```text
HTTP inbound:
  headers -> restx -> contextx

HTTP outbound:
  contextx -> http client middleware -> headers

gRPC inbound:
  metadata -> grpcx server interceptor -> contextx

gRPC outbound:
  contextx -> grpcx client interceptor -> metadata
```

`contextx` carries request-scoped values only:

- `request_id`
- trace id / trace context
- principal / actor
- tenant / org
- `authz_decision_id`
- scoped/internal token
- request logger

It does not carry DB, Redis, S3, or service clients. Those belong in
`servicecontextx`.

## 3. Proto contract style

Every externally visible RPC should be defined in proto and annotated with:

```proto
import "google/api/annotations.proto";
import "buf/validate/validate.proto";
import "api/aisphere/options/v1/authz.proto";

rpc DownloadSkillPackage(DownloadSkillPackageRequest) returns (DownloadSkillPackageReply) {
  option (google.api.http) = {
    get: "/api/v1/skills/{skill_id}/package"
  };
  option (aisphere.options.v1.authz) = {
    action: "skill.download"
    resource: "skill:{skill_id}"
    audience: "skill-service"
    mode: SCOPED_TOKEN
  };
  option (aisphere.options.v1.audit) = {
    event: "skill.package.download"
    risk: "medium"
  };
  option (aisphere.options.v1.capability) = {
    group: "skill"
    name: "download"
  };
}

message DownloadSkillPackageRequest {
  string skill_id = 1 [(buf.validate.field).string.min_len = 1];
}
```

The important design principle is: API, validation, authz, audit and capability
rules live in the proto contract instead of being scattered across handlers.

## 4. Generated outputs

`make api` runs `buf generate` and now supports:

```text
*.pb.go                  protobuf messages
*_grpc.pb.go             gRPC server/client
*_http.pb.go             kernel direct HTTP transport
*.gw.pb.go               grpc-gateway HTTP/JSON -> gRPC bridge
*_authz.pb.go            authz rule table, manifest, secure client wrapper
docs/openapi/*.json      OpenAPI v2 docs
```

The generated authz file contains:

- `<Service>AuthzRules`
- `<Service>AuthzManifestJSON`
- `<Service>SecureClient`

The secure client wrapper does not know your IAM implementation. It depends on
`authz.Guard`, which lets deployments back the decision with SpiceDB, OpenFGA,
OPA, local dev allow-all, or decision cache.

## 5. Runtime setup

### gRPC service

```go
validator, err := grpcx.NewValidator()
if err != nil { panic(err) }

srv := grpc.NewServer(grpcx.ServerOptions(grpcx.ServerConfig{
    ServiceName:      "skill-service",
    Logger:           logger,
    Metrics:          metrics,
    Validator:        validator,
    EnableValidation: true,
    EnableRecovery:   true,
})...)
```

This installs the standard inbound gRPC runtime:

```text
otelgrpc stats handler
metadata -> contextx
protovalidate
logx request log
metricsx counters/histograms
custom auth/authz interceptors, if supplied
panic recovery
```

### gRPC client

```go
conn, err := grpc.DialContext(ctx, target,
    append(
        grpcx.DialOptions(grpcx.DefaultClientConfig("workflow-service")),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )...,
)
```

The outbound client chain propagates request id, trace id, decision id and
scoped/internal tokens from `contextx` into gRPC metadata.

### HTTP/go-http validation

For generated `protoc-gen-go-http` handlers:

```go
validator, _ := grpcx.NewValidator()
restx.InstallProtovalidateRequestValidator(validator)
```

Generated handlers call `http.ValidateRequest(ctx, &in)` after binding
path/query/body and before business middleware.

### BFF/Gateway validation

BFF code that manually decodes protobuf requests can call:

```go
if err := grpcx.ValidateProto(validator, req); err != nil {
    return err
}
```

## 6. Internal authorization pattern

Internal services do not route through the business Gateway. They call each
other directly and delegate decisions to IAM.

```text
workflow-service
  -> authz.Guard.RequireScopedToken(action=skill.download, resource=skill:demo)
  -> contextx.WithScopedToken(ctx, token)
  -> grpcx client interceptor propagates token
  -> skill-service grpcx server interceptor restores context
  -> skill-service verifies token / scope / audience
```

Generated secure clients provide a safer wrapper:

```go
secureSkill := skillv1.NewSkillServiceSecureClient(rawSkillClient, authzGuard)
reply, err := secureSkill.DownloadSkillPackage(ctx, &skillv1.DownloadSkillPackageRequest{
    SkillId: "demo",
})
```

The generated wrapper resolves `skill:{skill_id}`, asks `authz.Guard` for a
check or scoped token according to the proto rule, stores the result in
`contextx`, then calls the raw gRPC client.

## 7. Development workflow

1. Define the proto message and service method.
2. Add `buf.validate` field constraints.
3. Add `google.api.http` only for external REST/JSON exposure.
4. Add `aisphere.authz`, and add `aisphere.audit` for high-risk operations.
5. Run `make api`.
6. Implement the gRPC server interface.
7. Use generated secure clients for internal calls.
8. Wire runtime with `grpcx.ServerOptions`, `grpcx.DialOptions`, `restx`, and `servicecontextx`.
9. Run proto lint/breaking checks and Go tests.

## 8. Framework upgrade roadmap

### Next near-term upgrades

- Add `buf-check-aisphere` to fail CI when external APIs lack authz/audit rules.
- Generate richer authz manifests for IAM bootstrap and capability docs.
- Add generated server-side guard binding for high-risk `SELF_CHECK` methods.
- Add generated TypeScript SDK via Buf/protobuf-es or Connect when frontend type-safety becomes a priority.

### Later upgrades

- Add decision cache and batch-check to `authz.Guard`.
- Add mTLS-based service identity alongside internal/scoped JWT.
- Add secure client generation for streaming RPCs.
- Add policy simulation tests generated from authz manifests.
- Add BSR/remote plugin mode for controlled enterprise builds.
