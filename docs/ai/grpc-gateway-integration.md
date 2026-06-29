# grpc-gateway integration

Kernel remains proto-first. grpc-gateway is added as an optional generated
HTTP/JSON bridge beside the existing `protoc-gen-go-http` transport path.

## What grpc-gateway does

grpc-gateway's `protoc-gen-grpc-gateway` reads protobuf services annotated with
`google.api.http` and emits `*.gw.pb.go` reverse-proxy code. The generated proxy
accepts REST/JSON HTTP requests and invokes the backing gRPC service.

This gives one proto contract and multiple generated outputs:

```text
foo.proto
  -> foo.pb.go          protobuf messages
  -> foo_grpc.pb.go     gRPC client/server stubs
  -> foo_http.pb.go     Kernel transportx/http binding, existing path
  -> foo.gw.pb.go       grpc-gateway REST/JSON -> gRPC bridge
docs/openapi/*      OpenAPI v2 documentation from protoc-gen-openapiv2
```

## Why keep both Kernel HTTP and grpc-gateway

`protoc-gen-go-http` is the existing Kernel/Kratos-style in-process HTTP binding.
It is still the best fit when a service wants HTTP and business implementation in
one process using `transportx/http`.

grpc-gateway is better when the canonical service is gRPC and HTTP/JSON should be
a gateway/BFF or edge adapter. It is especially useful for `aisphere-gateway` and
service APIs where gRPC remains the internal protocol.

## Generation

Install tools. `make tools` installs pinned grpc-gateway and OpenAPI plugins:

```text
protoc-gen-grpc-gateway@v2.29.0
protoc-gen-openapiv2@v2.29.0
```

Then run:

```bash
make tools
```

Generate with the normal Kernel flow:

```bash
make api
# or
make proto
# or
kernel proto client api
```

The layout template also includes grpc-gateway generation in `layout/buf.gen.yaml`.

## Runtime mounting

Generated grpc-gateway registration functions look like:

```go
RegisterSkillServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
RegisterSkillServiceHandler(ctx, mux, conn)
```

Kernel provides `grpcgatewayx` helpers so the generated mux can be mounted behind
`restx` and `gatewayx` filters without replacing `transportx/http`:

```go
h, err := grpcgatewayx.NewHandlerFromEndpoint(ctx, grpcgatewayx.HandlerConfig{
    Endpoint: "127.0.0.1:9000",
    DialOptions: []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(gatewayx.InternalJWTUnaryClientInterceptor(jwt, "skill-service")),
    },
}, skillv1.RegisterSkillServiceHandlerFromEndpoint)
```

## Boundary

grpc-gateway does protocol translation. It does not provide browser session,
CSRF, ReBAC, OPA, audit, or capability APIs. Those remain Kernel Gateway/IAM
responsibilities.


`grpcgatewayx.DefaultMuxOptions` propagates request IDs, trace IDs, authz decision IDs, and authorization metadata from the Gateway HTTP request into outgoing gRPC metadata.
