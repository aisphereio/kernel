# Aisphere Kernel Service

This project was generated from the local Aisphere Kernel layout.

It is a Kernel-first service skeleton. Application code should use Kernel
modules instead of importing infrastructure SDKs directly.

## Included Defaults

- Features: `__KERNEL_FEATURES__`
- DB: `dbx` with `__KERNEL_DB_DRIVER__`
- Cache: `cachex` with `__KERNEL_CACHE_DRIVER__`
- Object storage: `objectstorex` with `__KERNEL_OBJECTSTORE_DRIVER__`
- Authn: `__KERNEL_AUTHN_PROVIDER__`
- Authz: `__KERNEL_AUTHZ_PROVIDER__`
- Audit: `auditx` memory recorder by default
- Logging: `logx` console output for local development
- Config: `configx` file source
- Transports: Kernel HTTP and gRPC servers
- API example: protobuf-first Todo CRUD, HTTP binding, gRPC binding, streaming

External dependencies are present in `configs/config.yaml`, but DB, cache,
object storage, authn, and authz are disabled by default so the service starts
without local Postgres, Redis, Minio, Casdoor, or SpiceDB.

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
