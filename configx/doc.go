// Package configx provides the unified configuration API for Aisphere Kernel.
//
// configx is the ONLY runtime configuration reader in Kernel. It replaces the
// old Kratos-derived config package and ad-hoc os.Getenv / yaml.Unmarshal
// patterns. Business code should depend on Config or a module-specific struct
// produced by Config.Scan; it should not call os.Getenv or parse YAML/JSON
// directly.
//
// # Design principle
//
// configx only LOADS, MERGES, and RESOLVES configuration. It does NOT validate
// business fields, start services, record audit, or bind to a specific config
// center SDK. Other Kernel modules (logx, dbx, httpx, grpcx, authx) consume
// configx through the stable Config / Value / Scan APIs.
//
// configx depends only on the Go standard library plus the encodingx and logx
// packages; it does not import third-party config libraries.
//
// # 30-second quickstart
//
// The common startup flow is:
//
//	cfg := configx.New(configx.WithSource(file.NewSource("configs/app.yaml")))
//	defer cfg.Close()
//
//	if err := cfg.Load(); err != nil {
//	    return err
//	}
//
//	addr := configx.MustGet[string](cfg, "server.http.addr")
//	port := configx.GetOrDefault[int](cfg, "server.http.port", 8000)
//
// Use Value when a single key is needed:
//
//	enabled, err := cfg.Value("features.new_home").Bool()
//	timeout, err := cfg.Value("upstream.timeout_ns").Duration()
//
// Use Scan when a module owns a structured configuration section:
//
//	type HTTPConfig struct {
//	    Addr string `json:"addr"`
//	    Port int    `json:"port"`
//	}
//
//	var httpCfg HTTPConfig
//	if err := cfg.Value("server.http").Scan(&httpCfg); err != nil {
//	    return err
//	}
//
// # Sources
//
// A Source turns a backend into a slice of KeyValue values:
//
//	type Source interface {
//	    Load() ([]*KeyValue, error)
//	    Watch() (Watcher, error)
//	}
//
// Built-in local sources live in subpackages:
//
//	file.NewSource("configs/app.yaml")
//	env.NewSource("KERNEL_")
//
// Remote and platform sources live under contrib/config:
//
//	contrib/config/apollo
//	contrib/config/nacos
//	contrib/config/etcd
//	contrib/config/consul
//	contrib/config/kubernetes
//	contrib/config/polaris
//
// Each contrib source returns configx.Source so it can be dropped into
// WithSource without adapting business code.
//
// # Merge order
//
// Sources are loaded in the order passed to WithSource. Later sources override
// earlier leaf values while nested maps are merged recursively. Arrays are
// replaced as a whole. The recommended order is defaults first, environment or
// deployment-specific overrides last:
//
//	cfg := configx.New(configx.WithSource(
//	    file.NewSource("configs/common.yaml"),
//	    file.NewSource("configs/prod.yaml"),
//	    env.NewSource("KERNEL_"),
//	))
//
// # Placeholder resolution
//
// The default resolver expands placeholders in string values:
//
//	${KEY}
//	${KEY:default}
//	${server.http.addr}
//
// WithResolveActualTypes(true) converts a whole-value placeholder into bool,
// int64, or float64 when possible. Mixed strings remain strings:
//
//	"${PORT}"          -> 8080
//	"http://:${PORT}"  -> "http://:8080"
//
// Placeholders are resolved after merge, so a placeholder in source A can
// reference a value defined in source B.
//
// # Value API
//
// Value provides typed conversions. Each method returns (T, error); none
// panic on type mismatch:
//
//	Bool()      bool-like values (bool / "true" / "false" / numbers)
//	Int()       integer-like values (int / uint / float / numeric string)
//	Float()     floating-point values (int / uint / float / numeric string)
//	String()    string representation (any convertible type)
//	Duration()  time.Duration from integer nanoseconds
//	Slice()     []Value from []any
//	Map()       map[string]Value from map[string]any
//	Scan(any)   JSON/proto-compatible struct scan
//
// Value is backed by an atomic store, so a cached Value reference will
// reflect updates after Watch or Load without needing to re-call
// Config.Value.
//
// # Helper API
//
//	Get[T](cfg, key)              returns a typed value and an error
//	MustGet[T](cfg, key)          panics on error; use only at startup
//	GetOrDefault[T](cfg, key, v)  returns fallback on missing/invalid values
//
// Get supports bool / int / int64 / float64 / string natively. Other types
// fall through to Value.Scan.
//
// # Watch
//
// Watch registers one or more observers for a key. Observers are called when a
// loaded or watched config update changes that key. Observer callbacks should be
// short and non-blocking; use them to update local atomic settings or send a
// lightweight signal, not to perform network calls.
//
//	cfg.Watch("log.level", func(_ string, v Value) {
//	    level, _ := v.String()
//	    logLevel.Store(level)
//	})
//
// Watch requires the key to exist at registration time; this fails fast on
// startup misconfiguration rather than silently never firing.
//
// # Errors
//
//	ErrNotFound         key is absent
//	ErrClosed           Config was closed
//	ErrInvalidObserver  Watch was called with nil observer
//	ErrNilConfig        helper received a nil Config
//
// These errors are not business errors. Do not wrap them in errorx; surface
// them at startup as fail-fast failures.
//
// # Forbidden patterns
//
// Do not read os.Getenv in business code. Do not parse YAML/JSON inside every
// module. Do not hide config errors by silently using zero values for required
// fields. Do not do expensive work in Watch callbacks.
//
// Test code is exempt: it may use os.Setenv to construct env source fixtures.
//
// # Migration from old config package
//
// Replace:
//
//	import "github.com/aisphereio/kernel/config"
//	cfg := config.New(config.WithSource(file.NewSource("app.yaml")))
//
// with:
//
//	import "github.com/aisphereio/kernel/configx"
//	cfg := configx.New(configx.WithSource(file.NewSource("app.yaml")))
//
// The old config/ package has been removed; new code must use configx/ and
// its subpackages configx/file and configx/env.
//
// # Documentation
//
// See configx/README.md for the single-source-of-truth user guide, and
// docs/ai/configx.md for the AI coding recipe. docs/contracts/configx.md
// lists behaviors that cannot change without a major version bump.
package configx
