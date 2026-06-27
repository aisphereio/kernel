# configx AI Coding Guide

This is the single AI recipe for `configx`. Use it whenever code needs config.

## Hard rule

Use `github.com/aisphereio/kernel/configx`. Do not import the old
`github.com/aisphereio/kernel/configx` path. Do not call `os.Getenv` from
business packages.

## Startup pattern

```go
cfg := configx.New(configx.WithSource(
    file.NewSource("configs/common.yaml"),
    file.NewSource("configs/dev.yaml"),
    env.NewSource("KERNEL_"),
))
defer cfg.Close()

if err := cfg.Load(); err != nil {
    return err
}
```

## Read scalar values

```go
addr := configx.MustGet[string](cfg, "server.http.addr")
port := configx.GetOrDefault[int](cfg, "server.http.port", 8000)
```

Use `MustGet` only during process startup. In request-time code, prefer an
already-validated struct.

## Scan module config

```go
type DBConfig struct {
    Driver string `json:"driver"`
    DSN    string `json:"dsn"`
}

var db DBConfig
if err := cfg.Value("database").Scan(&db); err != nil {
    return err
}
```

## Watch runtime changes

```go
_ = cfg.Watch("log.level", func(_ string, value configx.Value) {
    level, _ := value.String()
    logLevel.Store(level)
})
```

Callbacks must be short, local, and non-blocking.

## Example lookup table

| Need | Example |
|---|---|
| Create config | `ExampleNew` |
| Read typed value | `ExampleGet` |
| Required startup value | `ExampleMustGet` |
| Optional default | `ExampleGetOrDefault` |
| Combine sources | `ExampleWithSource` |
| Custom decoder | `ExampleWithDecoder` |
| Custom resolver | `ExampleWithResolver` |
| Convert placeholders to actual types | `ExampleWithResolveActualTypes` |
| Custom merge | `ExampleWithMergeFunc` |
| Read raw Value | `ExampleConfig_Value` |
| Scan struct | `ExampleConfig_Scan` |
| Watch key | `ExampleConfig_Watch` |
| Bool conversion | `ExampleValue_Bool` |
| Int conversion | `ExampleValue_Int` |
| Float conversion | `ExampleValue_Float` |
| String conversion | `ExampleValue_String` |
| Slice conversion | `ExampleValue_Slice` |
| Map conversion | `ExampleValue_Map` |
| Partial scan | `ExampleValue_Scan` |

## Business examples

| Scenario | Example |
|---|---|
| Bootstrap HTTP server | `Example_businessBootstrapHTTPServer` |
| Layered file/env override | `Example_businessLayeredFileAndEnv` |
| Scan database config | `Example_businessScanDatabaseConfig` |
| Feature flag | `Example_businessFeatureFlag` |
| Upstream timeout config | `Example_businessUpstreamTimeoutConfig` |
| Resolve placeholder | `Example_businessResolvePlaceholder` |
| Actual typed placeholder | `Example_businessActualTypedPlaceholder` |
| Observe runtime change | `Example_businessObserveRuntimeChange` |
| Validate required config | `Example_businessValidateRequiredConfig` |
| Custom dotenv decoder | `Example_businessCustomDecoderForDotEnv` |

## Forbidden patterns

```go
// Forbidden
os.Getenv("DATABASE_DSN")
yaml.Unmarshal(data, &cfg)
json.Unmarshal(data, &cfg)

// Required
cfg.Value("database.dsn").String()
cfg.Value("database").Scan(&dbCfg)
```

## Import migration

Old:

```go
import "github.com/aisphereio/kernel/configx"
```

New:

```go
import "github.com/aisphereio/kernel/configx"
```

Subpackages also moved:

```go
import "github.com/aisphereio/kernel/configx/file"
import "github.com/aisphereio/kernel/configx/env"
```

## Rules for generated code

1. Generate config structs with json tags.
2. Use `Scan` at startup, not inside hot request paths.
3. Treat `ErrNotFound` as startup validation failure for required fields.
4. Do not swallow `Load` errors.
5. Always `defer cfg.Close()` in examples and short-lived tools.
