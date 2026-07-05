# Aisphere Kernel Service

This project was generated from the local Aisphere Kernel layout.

It is a Kernel-first service skeleton. Application code should use Kernel
modules instead of importing infrastructure SDKs directly.

## Authn

Kernel provides a provider-neutral OIDC/JWKS JWT verification package at `authn/oidcx/`.

```text
authn/oidcx/
├── config.go       # Config struct with Normalized() defaults
├── jwks.go         # JWKS discovery, fetch, cache, RSA/EC key parsing
└── verifier.go     # JWT signature verification + claims validation + Principal mapping
```

It supports:
- OIDC Discovery URL auto-configuration
- JWKS public key caching (in-process, TTL configurable)
- JWT signature verification (RSA and ECDSA)
- Claims validation: `iss`, `aud`, `exp`, `nbf`, `iat`, `alg`, Casdoor `owner`
- Configurable claim name mapping for provider-neutral usage
- Principal normalization into `authn.Principal`

Additional authn utilities:
- `authn/cached_authenticator.go` — token verification result cache (SHA-256 key, TTL capped by token exp)
- `authn/trusted_headers.go` — Gateway trusted header injection, stripping, and reconstruction
- `authn/trusted_authenticator.go` — TrustedHeaderAuthenticator for `gateway_trusted` mode

### Gateway Integration

The Gateway (`aisphere-gateway`) uses `authn/oidcx` to verify external JWTs locally before forwarding trusted identity headers to internal services. See `gatewayx/dispatcher.go` for the authn orchestration flow:

1. `StripTrustedHeaders` — remove any client-supplied X-Aisphere-* headers
2. Extract Bearer token from `Authorization` header or `aisphere_access_token` cookie
3. `oidcx.Verifier.Authenticate()` — verify JWT signature + claims
4. `InjectTrustedHeaders` — inject verified Principal as X-Aisphere-* headers
5. Forward to upstream gRPC service

### Backend Service Modes

Backend services (IAM, Hub) support two authentication modes:

| Mode | Description | Config |
| --- | --- | --- |
| `casdoor_jwt` | Backend verifies the JWT locally using `oidcx.Verifier` | `security.authn.mode: casdoor_jwt` |
| `gateway_trusted` | Backend trusts Gateway-injected X-Aisphere-* headers | `security.authn.mode: gateway_trusted` |

See `docs/design/authn-full-flow.md` for the full design document.

## Included Defaults

- Features: `__KERNEL_FEATURES__`
- DB: `dbx` with `__KERNEL_DB_DRIVER__`
- Cache: `cachex` with `__KERNEL_CACHE_DRIVER__`
- Object storage: `objectstorex` with `__KERNEL_OBJECTSTORE_DRIVER__`
- Authn: `__KERNEL_AUTHN_PROVIDER__`
- Authz: `__KERNEL_AUTHZ_PROVIDER__`
- Audit: `auditx` memory recorder by default
- Logging: `logx` console output for local development
- Metrics: shared `metricsx.Manager`, optional admin `/metrics` server
- DTM: optional `dtmx.Manager`, provider-neutral Saga abstraction backed by DTM when enabled
- Config: `configx` file source
- Transports: Kernel HTTP and gRPC servers with access log and metrics hooks
- API example: protobuf-first Todo CRUD, HTTP binding, gRPC binding, streaming

External dependencies are present in `configs/config.yaml`, but DB, cache,
object storage, authn, authz, and DTM are disabled by default so the service starts
without local Postgres, Redis, Minio, Casdoor, SpiceDB, or DTM.

## Boot Order

The generated `cmd/server/main.go` follows the current Kernel startup pattern:

```text
configx load
  -> logx.New and install via kernel.LogxLogger
  -> metricsx manager creation
  -> optional dtmx.New
  -> dbx/cachex/objectstorex/authn/authz resource initialization with shared logger/metrics
  -> HTTP/gRPC server construction with access log + metrics
  -> kernel.New(..., kernel.Metrics(...), kernel.DTM(...))
```

This order ensures component initialization logs and metrics use the same logger
and metric registry as transport access logs.

## Layout

```text
api/                 Protobuf APIs and generated HTTP/gRPC bindings
cmd/server/          Application entrypoint
configs/             Local config with Kernel module defaults
internal/conf/        Config DTOs scanned by configx
internal/server/      Kernel HTTP and gRPC server construction
internal/service/     Transport-facing Todo service
internal/biz/         Use cases, domain contracts, errorx errors
internal/data/        Repositories and Kernel resource initialization
```

## Run

```bash
go run ./cmd/server -conf ./configs
```

Default HTTP endpoints:

- `GET /healthz`
- `GET /readyz`
- `POST /v1/todos/create`
- `GET /v1/todos/{id}`
- `GET /v1/todos/list`
- `PUT /v1/todos/update`
- `DELETE /v1/todos/{id}`
- `GET /v1/todos/watch`
- `GET /v1/todos/sync`

Default transport ports:

- HTTP: `0.0.0.0:8000`
- gRPC: `0.0.0.0:9000`
- Metrics admin server: `127.0.0.1:9090/metrics` when `metrics.enabled=true`

## DTM

`dtm.enabled=false` by default. When enabled, import the concrete DTM driver once:

```go
import _ "github.com/aisphereio/kernel/dtmx/dtm"
```

Application code should still depend only on `dtmx.Manager`. DTM server storage
and deployment are external infrastructure concerns; the app config only points
to the DTM API endpoint and its own branch callback base URL.

`dtmx/dtm` already supports the branch header mechanism. When `branch_secret` is
configured, DTM branch callbacks carry:

```text
X-Kernel-DTM-Branch-Token: <branch_secret>
```

Business branch endpoints should wrap internal routes with `dtmx.BranchAuthMiddleware`
or call `dtmx.ValidateBranchRequest`. This protects branch callback endpoints but
does not replace private networking, firewall rules, mTLS, or service mesh policy
for the DTM server itself.

## Generate

```bash
buf generate --template buf.gen.yaml
```

## Verify

```bash
go test ./...
```

The generated `go.mod` contains a local `replace github.com/aisphereio/kernel => ..`
for development inside this repository. Remove or adjust it when publishing the
service outside the Kernel source tree.
