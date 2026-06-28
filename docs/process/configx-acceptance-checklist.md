# configx Acceptance Checklist

## Static checks

- [ ] No import of `github.com/aisphereio/kernel/config"` (old path) remains anywhere in the repo.
- [ ] Business code does not call `os.Getenv` / `os.LookupEnv` directly (boot layer and tests exempt).
- [ ] Business code does not import `gopkg.in/yaml.v3`, `encoding/json` for config parsing (using `configx.Scan` instead).
- [ ] Business code does not import `github.com/spf13/viper` or other third-party config libraries.
- [ ] Root `README.md` links to `configx/README.md`.
- [ ] `docs/ai/configx.md` exists as the single AI guide.
- [ ] `docs/contracts/configx.md` exists.
- [ ] `docs/design/configx.md` exists.
- [ ] `configx/README.md` is single-entry user guide (>200 lines, UTF-8 clean).
- [ ] `configx/doc.go` exists with package-level doc.
- [ ] `examples/configx-basic/` exists with README + main.go.
- [ ] `scripts/check-configx-usage.sh` exists and is executable.

## Unit checks

```bash
go test ./configx ./configx/env ./configx/file -count=1
go test ./configx -race -count=1
go test ./configx -cover -count=1
```

Required behaviors:

- [ ] `Get[T]` reads string / int / bool / float / struct values.
- [ ] `GetOrDefault` returns fallback for missing values.
- [ ] `GetOrDefault` returns fallback for type-mismatch values.
- [ ] `MustGet` panics on required missing config.
- [ ] `MustGet` panics on type-mismatch.
- [ ] `Get` returns `ErrNilConfig` when Config is nil.
- [ ] `Load` refreshes cached Value objects after reload.
- [ ] Type changes across reload do not panic or preserve stale values.
- [ ] Multiple observers on one key are called.
- [ ] `Watch` on missing key returns `ErrNotFound`.
- [ ] `Watch` with nil observer returns `ErrInvalidObserver`.
- [ ] `Close` is idempotent.
- [ ] `Close` returns nil even when no watchers were registered.
- [ ] `Load` after `Close` returns `ErrClosed`.
- [ ] `WithSource` ignores nil sources without panic.
- [ ] `WithResolveActualTypes(true)` converts `${PORT}` to int.
- [ ] Default resolver expands `${KEY}` and `${KEY:default}`.
- [ ] Default merge recursively merges nested maps.
- [ ] Default merge replaces arrays as a whole.
- [ ] `Value.Bool()` accepts `"true" / "false" / 0 / 1 / bool`.
- [ ] `Value.Int()` accepts `int / float / numeric string`.
- [ ] `Value.Duration()` accepts nanosecond int and parseable duration string.
- [ ] `Value.Scan` populates struct with `json` tags.
- [ ] `Value.Scan` populates `proto.Message` via `protojson`.
- [ ] `Value` for missing key returns `errValue` with `ErrNotFound` on every method.

## Example checks

```bash
go test ./configx -run=Example -v
```

Required examples (one per public API):

- [ ] `ExampleNew`
- [ ] `ExampleGet`
- [ ] `ExampleMustGet`
- [ ] `ExampleGetOrDefault`
- [ ] `ExampleWithSource`
- [ ] `ExampleWithDecoder`
- [ ] `ExampleWithResolver`
- [ ] `ExampleWithResolveActualTypes`
- [ ] `ExampleWithMergeFunc`
- [ ] `ExampleConfig_Value`
- [ ] `ExampleConfig_Scan`
- [ ] `ExampleConfig_Watch`
- [ ] `ExampleValue_Bool`
- [ ] `ExampleValue_Int`
- [ ] `ExampleValue_Float`
- [ ] `ExampleValue_String`
- [ ] `ExampleValue_Slice`
- [ ] `ExampleValue_Map`
- [ ] `ExampleValue_Scan`

Required business examples (10 scenarios):

- [ ] `Example_businessBootstrapHTTPServer`
- [ ] `Example_businessLayeredFileAndEnv`
- [ ] `Example_businessScanDatabaseConfig`
- [ ] `Example_businessFeatureFlag`
- [ ] `Example_businessUpstreamTimeoutConfig`
- [ ] `Example_businessResolvePlaceholder`
- [ ] `Example_businessActualTypedPlaceholder`
- [ ] `Example_businessObserveRuntimeChange`
- [ ] `Example_businessValidateRequiredConfig`
- [ ] `Example_businessCustomDecoderForDotEnv`

## Contract checks

```bash
go test ./configx -run=TestErrorxContract -v   # if cross-module contract exists
go test ./configx -run=TestContract -v
```

## Integration checks

```bash
go test ./configx -run=TestIntegration -v
```

Required integration scenarios:

- [ ] Watch fires observer after Load with new value.
- [ ] Multiple observers fire in registration order.
- [ ] Reload after type change preserves Value atomicity.
- [ ] Close stops all watchers without leaking goroutines.
- [ ] Cross-package usage: `configx` + `logx` consumer pattern works.
- [ ] Cross-package usage: `configx` + `dbx` Config Scan pattern works.

## Coverage checks

```bash
go test ./configx -cover -count=1
go test ./configx/env -cover -count=1
go test ./configx/file -cover -count=1
```

Expected coverage:

- [ ] `configx` package: >= 85%
- [ ] `configx/env` package: >= 90%
- [ ] `configx/file` package: >= 80%

## Fuzz checks

```bash
go test ./configx -run=^$ -fuzz=FuzzConfigLoad -fuzztime=30s
go test ./configx -run=^$ -fuzz=FuzzValueScan -fuzztime=30s
```

Required fuzz invariants:

- [ ] Any byte sequence as JSON source does not panic `Load`.
- [ ] Any byte sequence as YAML source does not panic `Load`.
- [ ] Any `Value.Scan` target with arbitrary JSON does not panic.

## Benchmark checks

```bash
go test ./configx -bench=. -benchmem
```

Expected performance (rough targets, M1/Linux x86_64):

- [ ] `BenchmarkGet` < 200 ns/op, 0 allocs/op
- [ ] `BenchmarkValue` < 100 ns/op, 0 allocs/op
- [ ] `BenchmarkScan` < 5000 ns/op
- [ ] `BenchmarkLoad` < 50000 ns/op

## Documentation checks

```bash
./scripts/check-module-docs.sh configx
```

## Manual migration checks

- [ ] Replace `config.New` with `configx.New`.
- [ ] Replace `config/file` with `configx/file`.
- [ ] Replace `config/env` with `configx/env`.
- [ ] Update contrib source imports to return `configx.Source`.
- [ ] All `defer cfg.Close()` added.
- [ ] All startup `Load` errors propagate (not swallowed).
- [ ] No `os.Getenv` in handler/service/repository/worker code.
- [ ] All module Config structs use `json` tags (not `yaml` / `mapstructure`).

## Windows commands

Windows can use Make directly:

```powershell
make tools
make test-cmd
```

Or run the wrapper scripts:

```bat
scripts\tools.cmd
scripts\test-cmd.cmd
```

The `.cmd` wrappers launch PowerShell with process-level `ExecutionPolicy Bypass` and do not change user or machine policy.
