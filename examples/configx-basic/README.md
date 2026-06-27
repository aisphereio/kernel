# configx-basic

Minimal runnable demo for `configx`.

Run from repository root:

```bash
go run ./examples/configx-basic
```

Expected output:

```text
kernel/dev listens on 0.0.0.0:8000
```

The example writes a temporary JSON file, loads it through `configx/file`, scans
it into a typed struct, and prints the final runtime configuration.
