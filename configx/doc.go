// Package configx provides the unified configuration API for Aisphere Kernel.
//
// The package loads configuration from one or more Source implementations,
// merges them into a single tree, resolves placeholders, and exposes typed
// reads through Value and Scan. Business code should depend on Config or a
// module-specific struct produced by Config.Scan; it should not call os.Getenv
// or parse YAML/JSON directly.
//
// # Quickstart
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
//	"http://:${PORT}" -> "http://:8080"
//
// # Value API
//
// Value provides typed conversions:
//
//	Bool()      bool-like values
//	Int()       integer-like values
//	Float()     floating-point values
//	String()    string representation
//	Duration()  time.Duration from integer nanoseconds
//	Slice()     []Value
//	Map()       map[string]Value
//	Scan(any)   JSON/proto-compatible struct scan
//
// Helper API
//
//	Get[T](cfg, key)              returns a typed value and an error
//	MustGet[T](cfg, key)          panics on error; use only at startup
//	GetOrDefault[T](cfg, key, v)  returns fallback on missing/invalid values
//
// # Watch
//
// Watch registers one or more observers for a key. Observers are called when a
// loaded or watched config update changes that key. Observer callbacks should be
// short and non-blocking; use them to update local atomic settings or send a
// lightweight signal, not to perform network calls.
//
// Errors
//
//	ErrNotFound         key is absent
//	ErrClosed           Config was closed
//	ErrInvalidObserver  Watch was called with nil observer
//	ErrNilConfig        helper received a nil Config
//
// # Forbidden patterns
//
// Do not read os.Getenv in business code. Do not parse YAML/JSON inside every
// module. Do not hide config errors by silently using zero values for required
// fields. Do not do expensive work in Watch callbacks.
//
// # Documentation
//
// See configx/README.md for the single-entry guide, docs/ai/configx.md for AI
// usage rules, docs/design/configx.md for design details, and
// docs/contracts/configx.md for behavior that cannot change without a major
// version bump.
package configx
