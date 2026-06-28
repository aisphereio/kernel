# configx-watch

Demonstrates configx Watch hot reload.

Run from repository root:

```bash
go run ./examples/configx-watch
```

Expected output (timing may vary):

```text
loaded config: log.level=info
watching log.level... (rewrite the file to trigger reload)
rewrote file: log.level=debug
reload detected: log.level=debug
done
```

The example writes a config file, loads it, registers a Watch callback on
`log.level`, then rewrites the file to trigger a reload. The watcher (backed
by `fsnotify`) detects the file change and fires the callback.

Watch contract:

- `Watch(key, observer)` requires the key to exist at registration time.
- Multiple observers per key are supported.
- Observers fire synchronously inside `Load`; keep them short and non-blocking.
- Use an `atomic.Value` to ferry the new value back to your application code.

For the full pattern (including `WithResolveActualTypes`, layered env override,
and custom decoders), see `configx/example_test.go` and
`configx/example_business_test.go`.
