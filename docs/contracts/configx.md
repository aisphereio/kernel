# configx Contract

These behaviors cannot change without a major version bump.

## Import path

The stable import path is `github.com/aisphereio/kernel/configx`.

## Public API

`Config`, `Source`, `Watcher`, `Value`, `KeyValue`, `New`, `Get`, `MustGet`,
`GetOrDefault`, and all `WithXxx` options remain source compatible.

## Merge contract

Sources are merged in the order passed to `WithSource`. Later leaf values
override earlier leaf values. Nested maps merge recursively. Arrays replace.

## Placeholder contract

`${KEY}` and `${KEY:default}` remain supported. Missing values without defaults
resolve to an empty string.

## Error contract

Missing keys return `ErrNotFound`. Closed configs return `ErrClosed`. Nil
observers return `ErrInvalidObserver`. Nil helper config returns `ErrNilConfig`.

## Watch contract

`Watch(key, observer)` requires the key to exist at registration time. Multiple
observers for one key are allowed. Callbacks receive the key and current Value.

## Close contract

`Close` is idempotent.

## Migration contract

The old `config/` package is removed from the root module. New code must use
`configx/` and `configx/file`, `configx/env`.
