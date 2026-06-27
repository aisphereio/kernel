# configx Acceptance Checklist

## Static checks

- [ ] No import of `github.com/aisphereio/kernel/configx` remains.
- [ ] Business code does not call `os.Getenv` directly.
- [ ] Root README links to `configx/README.md`.
- [ ] `docs/ai/configx.md` exists as a single AI guide.
- [ ] `.cursor/rules/configx.mdc` exists.

## Unit checks

```bash
go test ./configx ./configx/env ./configx/file
```

Required behaviors:

- [ ] `Get[T]` reads string/int/bool/float/struct values.
- [ ] `GetOrDefault` returns fallback for missing values.
- [ ] `MustGet` panics on required missing config.
- [ ] `Load` refreshes cached Value objects.
- [ ] Type changes across reload do not panic or preserve stale values.
- [ ] Multiple observers on one key are called.
- [ ] `Close` is idempotent.

## Example checks

```bash
go test ./configx -run=Example -v
```

## Documentation checks

```bash
./scripts/check-module-docs.sh configx
```

## Manual migration checks

- [ ] Replace `config.New` with `configx.New`.
- [ ] Replace `config/file` with `configx/file`.
- [ ] Replace `config/env` with `configx/env`.
- [ ] Update contrib source imports to return `configx.Source`.
