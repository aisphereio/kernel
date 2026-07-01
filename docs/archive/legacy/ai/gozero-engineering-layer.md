# Kernel go-zero-style engineering layer

This patch keeps `transportx/http` and the existing proto-first transport intact.
It adds a thin engineering layer inspired by go-zero where it is useful for
Kernel service teams:

- `servicecontextx`: a small ServiceContext-style dependency container for
  generated or hand-written handler/logic code.
- `gatewayx`: route/service descriptors that generated code can emit and a
  Gateway/BFF toolkit for session cookies, internal JWT and reverse proxying.
- `restx`: a standard REST filter chain comparable to go-zero's
  `RestConf.Middlewares` toggles.
- `dbx` raw SQL helpers: a direct-SQL escape hatch under the canonical `dbx` entrypoint; no separate database package is introduced.

## What this intentionally does not do

- It does not replace `transportx/http`.
- It does not introduce go-zero as a dependency.
- It does not implement a new `.api` parser yet.
- It does not move business authz into generic middleware.
- It does not replace `authn`, `authz`, `auditx`, `dbx`, `objectstorex`, or `dtmx`.
- It does not replace `protoc-gen-go-http`; grpc-gateway support is additive.

## Target pattern

```text
cmd/aisphere-gateway
  internal/handler   generated or thin HTTP handlers
  internal/logic     BFF logic, explicit AuthzGuard calls
  internal/svc       ServiceContext wiring
  internal/types     request/response DTOs
```

Generated code can target:

```go
svc := gatewayx.Service{
    Name:   "skills",
    Prefix: "/api/v1",
    Filters: []httpx.FilterFunc{
        gatewayx.SessionMiddleware(...),
    },
    Routes: []gatewayx.Route{
        {Method: http.MethodGet, Path: "/skills/{name}", Handler: getSkill},
    },
}
_ = gatewayx.RegisterServices(server.Route("/"), svc)
```

## REST standard chain

`restx.DefaultMiddlewares()` provides a go-zero-like default chain:

```text
Trace
RequestContext
AccessLog
Metrics
SanitizeHeaders
SecurityHeaders
MaxConns
Breaker
Shedding
Timeout
Recover
MaxBytes
Gunzip
```

Optional filters:

```text
CORS
CSRF
SessionMiddleware
InternalJWT verifier/injector
```

The chain maps to existing Kernel components where possible:

- breaker uses `internal/circuitbreaker`
- adaptive shedding uses `internal/ratelimit`
- timeout uses request context deadlines
- security/CRLF header checks live at Gateway edge

## Browser session and CSRF

`gatewayx.SessionMiddleware` loads an opaque cookie session from a `SessionStore`
and injects `authn.Principal` into the request context.

`restx.CSRF` validates unsafe methods using an HMAC token bound to the session
cookie. This is the BFF/browser path; machine-to-machine APIs can still use JWT
or internal service credentials.

## Proto HTTP/JSON gateway generation

Kernel can generate both the existing `*_http.pb.go` files and grpc-gateway
`*.gw.pb.go` reverse-proxy files from the same proto annotations. Use
`google.api.http` as the single contract source; `protoc-gen-grpc-gateway`
produces REST/JSON-to-gRPC adapters while `protoc-gen-go-http` keeps the
existing transportx/http path available.

Generated grpc-gateway handlers should be mounted behind `restx`/`gatewayx`
filters with `grpcgatewayx.NewHandlerFromEndpoint` or
`grpcgatewayx.NewHandlerFromConn`. `make api` now also emits OpenAPI v2 docs
through `protoc-gen-openapiv2` into `docs/openapi`.

## Gateway to internal services

`gatewayx.InternalJWT` signs short-lived HS256 tokens for Gateway -> service
calls. `gatewayx.NewReverseProxy` can inject this token while stripping browser
cookies before forwarding.

For BFF-heavy routes, prefer logic -> RPC/client calls instead of reverse proxy.
Reverse proxy is for pass-through routes and migration windows.

## Next migration steps

1. Add `kernel api` generator that reads a small DSL or annotated YAML and emits
   `gatewayx.Service` / `gatewayx.Route` declarations.
2. Add `rpcx` client/server runtime with a standard chain similar to go-zero's
   zrpc: trace, recover, stat, prometheus, breaker, shedding, timeout, auth.
3. Add Redis-backed `gatewayx.SessionStore` on top of `cachex/redis`.
4. Wire `restx.Conf` into the layout template so new services get the standard
   REST chain by default.
5. Keep authz explicit in logic via `accessx.Guard` / IAM, especially list and
   resource mutation APIs.


See also: `docs/ai/gateway-runtime.md` for the current Gateway/BFF runtime decision.
