# configx-env

Demonstrates layered `file + env` configuration with configx.

Run from repository root:

```bash
go run ./examples/configx-env
```

Expected output:

```text
loaded config:
app.name=kernel
app.env=prod       (overridden by KERNEL_APP_ENV)
server.port=9000   (overridden by KERNEL_SERVER_PORT)
server.addr=0.0.0.0 (from base file)
```

The example writes a temporary JSON base file, then simulates environment
overrides via `KERNEL_*` variables. `configx.New(configx.WithSource(...))`
combines them: later sources override earlier leaf values, while nested
maps merge recursively.

For the full pattern (including `WithResolveActualTypes`, `Watch`, and
custom decoders), see `configx/example_test.go` and
`configx/example_business_test.go`.
