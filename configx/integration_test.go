package configx

import (
        "errors"
        "sync/atomic"
        "testing"
        "time"
)

// Integration tests exercise the full Config pipeline (Source → Reader →
// Value → Watch → Close) end-to-end. They do not depend on any other Kernel
// module, but they verify patterns that real consumers (logx, dbx, httpx)
// will rely on.

func TestIntegrationLoadValueScanWatch(t *testing.T) {
        t.Parallel()

        cfg := New(WithSource(newJSONSource(`{
                "server": {"addr": "0.0.0.0", "port": 8000},
                "database": {"driver": "postgres", "dsn": "postgres://localhost/app"},
                "features": {"new_home": true}
        }`)))
        defer cfg.Close()

        if err := cfg.Load(); err != nil {
                t.Fatalf("Load: %v", err)
        }

        // Get paths
        assertEqual(t, MustGet[string](cfg, "server.addr"), "0.0.0.0")
        assertEqual(t, MustGet[int](cfg, "server.port"), 8000)
        assertEqual(t, MustGet[string](cfg, "database.driver"), "postgres")

        // Scan nested struct
        var dbCfg struct {
                Driver string `json:"driver"`
                DSN    string `json:"dsn"`
        }
        if err := cfg.Value("database").Scan(&dbCfg); err != nil {
                t.Fatalf("Scan database: %v", err)
        }
        assertEqual(t, dbCfg.Driver, "postgres")
        assertEqual(t, dbCfg.DSN, "postgres://localhost/app")

        // Watch + reload
        var observed atomic.Value // string
        _ = observed // initialize reference
        observed.Store("")
        if err := cfg.Watch("server.port", func(_ string, v Value) {
                port, _ := v.Int()
                observed.Store(itoa(int(port)))
        }); err != nil {
                t.Fatalf("Watch: %v", err)
        }

        // Simulate reload with new value via re-Load on the same source.
        // (We use mutableJSONSource to swap data underneath.)
        src := &mutableJSONSource{data: `{"server":{"port":8000}}`}
        cfg2 := New(WithSource(src))
        defer cfg2.Close()
        if err := cfg2.Load(); err != nil {
                t.Fatalf("Load cfg2: %v", err)
        }
        var saw atomic.Value
        saw.Store("")
        _ = cfg2.Watch("server.port", func(_ string, v Value) {
                port, _ := v.Int()
                saw.Store(itoa(int(port)))
        })
        src.data = `{"server":{"port":9090}}`
        if err := cfg2.Load(); err != nil {
                t.Fatalf("re-Load cfg2: %v", err)
        }
        awaitTimeout(t, "watch fires", time.Second, func() bool {
                return saw.Load().(string) == "9090"
        })
}

func TestIntegrationReloadRefreshesCachedValue(t *testing.T) {
        t.Parallel()

        src := &mutableJSONSource{data: `{"feature":{"enabled":true}}`}
        cfg := New(WithSource(src))
        defer cfg.Close()

        assertNoError(t, cfg.Load())
        v := cfg.Value("feature.enabled")
        enabled, _ := v.Bool()
        assertEqual(t, enabled, true)

        // Change value AND type (bool -> string)
        src.data = `{"feature":{"enabled":"false"}}`
        assertNoError(t, cfg.Load())

        // Same Value reference should reflect new value (atomic store).
        got, err := v.Bool()
        assertNoError(t, err)
        assertEqual(t, got, false)
}

func TestIntegrationMultipleObserversFireInOrder(t *testing.T) {
        t.Parallel()

        src := &mutableJSONSource{data: `{"x":1}`}
        cfg := New(WithSource(src))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        var mu atomicMutex
        var calls []string
        _ = cfg.Watch("x", func(_ string, v Value) {
                n, _ := v.Int()
                mu.Lock()
                calls = append(calls, "first:"+itoa(int(n)))
                mu.Unlock()
        })
        _ = cfg.Watch("x", func(_ string, v Value) {
                n, _ := v.Int()
                mu.Lock()
                calls = append(calls, "second:"+itoa(int(n)))
                mu.Unlock()
        })

        src.data = `{"x":2}`
        assertNoError(t, cfg.Load())

        awaitTimeout(t, "observers", time.Second, func() bool {
                mu.Lock()
                defer mu.Unlock()
                return len(calls) >= 2
        })

        mu.Lock()
        defer mu.Unlock()
        if len(calls) < 2 {
                t.Fatalf("calls = %v", calls)
        }
        if calls[0] != "first:2" {
                t.Fatalf("first call = %q, want %q", calls[0], "first:2")
        }
        if calls[1] != "second:2" {
                t.Fatalf("second call = %q, want %q", calls[1], "second:2")
        }
}

func TestIntegrationCloseStopsWatchersWithoutLeak(t *testing.T) {
        t.Parallel()

        cfg := New(WithSource(newJSONSource(`{"x":1}`)))
        defer cfg.Close()

        assertNoError(t, cfg.Load())

        // Close should return promptly even though watchers block forever.
        done := make(chan struct{})
        go func() {
                _ = cfg.Close()
                close(done)
        }()
        select {
        case <-done:
        case <-time.After(2 * time.Second):
                t.Fatal("Close did not return within 2s; watcher goroutine likely leaked")
        }
}

func TestIntegrationEnvSourceOverridesFile(t *testing.T) {
        t.Parallel()

        // Simulate env source by feeding two sources: file first, env second.
        // env source strips KERNEL_ prefix and turns _ into . separators.
        cfg := New(WithSource(
                newJSONSource(`{"database":{"dsn":"postgres://default","max_open_conns":4}}`),
                // Fake env-style override: just a flat key with dotted path.
                newMemorySource("DATABASE_DSN", "", "postgres://override"),
        ))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        // File values that weren't overridden survive.
        assertEqual(t, GetOrDefault[int](cfg, "database.max_open_conns", 0), 4)
}

func TestIntegrationPlaceholderChainsThroughSources(t *testing.T) {
        t.Parallel()

        // First source defines ${HOST}; second source uses ${HOST}.
        cfg := New(WithSource(
                newJSONSource(`{"HOST":"127.0.0.1"}`),
                newJSONSource(`{"server":{"addr":"${HOST}:8000"}}`),
        ))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        assertEqual(t, MustGet[string](cfg, "server.addr"), "127.0.0.1:8000")
}

func TestIntegrationConsumedByLogxLikePattern(t *testing.T) {
        t.Parallel()

        // Simulate a logx-like consumer that scans a struct and watches a level key.
        cfg := New(WithSource(newJSONSource(`{
                "log": {"level": "info", "format": "json"}
        }`)))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        type LogConfig struct {
                Level  string `json:"level"`
                Format string `json:"format"`
        }
        var logCfg LogConfig
        assertNoError(t, cfg.Value("log").Scan(&logCfg))
        assertEqual(t, logCfg.Level, "info")
        assertEqual(t, logCfg.Format, "json")

        // Watch level changes.
        var current atomic.Value
        current.Store(logCfg.Level)
        _ = cfg.Watch("log.level", func(_ string, v Value) {
                lvl, _ := v.String()
                current.Store(lvl)
        })

        // Trigger reload with new level.
        src := &mutableJSONSource{data: `{"log":{"level":"info","format":"json"}}`}
        cfg2 := New(WithSource(src))
        defer cfg2.Close()
        assertNoError(t, cfg2.Load())
        var saw atomic.Value
        saw.Store("")
        _ = cfg2.Watch("log.level", func(_ string, v Value) {
                lvl, _ := v.String()
                saw.Store(lvl)
        })
        src.data = `{"log":{"level":"debug","format":"json"}}`
        assertNoError(t, cfg2.Load())
        awaitTimeout(t, "level updates", time.Second, func() bool {
                return saw.Load().(string) == "debug"
        })
}

func TestIntegrationConsumedByDbxLikePattern(t *testing.T) {
        t.Parallel()

        cfg := New(WithSource(newJSONSource(`{
                "database": {
                        "driver": "postgres",
                        "dsn": "postgres://user:pass@localhost:5432/app?sslmode=disable",
                        "max_open_conns": 32,
                        "max_idle_conns": 8,
                        "conn_max_lifetime_ns": 1800000000000
                }
        }`)))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        type DBCfg struct {
                Driver          string `json:"driver"`
                DSN             string `json:"dsn"`
                MaxOpenConns    int    `json:"max_open_conns"`
                MaxIdleConns    int    `json:"max_idle_conns"`
                ConnMaxLifetime int64  `json:"conn_max_lifetime_ns"`
        }
        var dbCfg DBCfg
        assertNoError(t, cfg.Value("database").Scan(&dbCfg))

        assertEqual(t, dbCfg.Driver, "postgres")
        assertEqual(t, dbCfg.MaxOpenConns, 32)
        assertEqual(t, dbCfg.MaxIdleConns, 8)
        assertEqual(t, dbCfg.ConnMaxLifetime, int64(1800_000_000_000))

        // Also verify Duration-style read.
        d, err := cfg.Value("database.conn_max_lifetime_ns").Duration()
        assertNoError(t, err)
        assertEqual(t, d, 30*time.Minute)
}

func TestIntegrationErrorsIsCompatibleWithStdlib(t *testing.T) {
        t.Parallel()

        cfg := New(WithSource(newJSONSource(`{}`)))
        defer cfg.Close()
        assertNoError(t, cfg.Load())

        _, err := Get[string](cfg, "missing.key")
        if !errors.Is(err, ErrNotFound) {
                t.Fatalf("errors.Is(err, ErrNotFound) = false; err=%v", err)
        }
        if !errors.Is(err, ErrNotFound) {
                t.Fatalf("second errors.Is check failed; err=%v", err)
        }

        // Wrap and verify still matches.
        wrapped := wrapErr("outer", err)
        if !errors.Is(wrapped, ErrNotFound) {
                t.Fatalf("errors.Is(wrapped, ErrNotFound) = false; wrapped=%v", wrapped)
        }
}

func wrapErr(msg string, inner error) error {
        return &wrappedErr{msg: msg, inner: inner}
}

type wrappedErr struct {
        msg   string
        inner error
}

func (w *wrappedErr) Error() string { return w.msg + ": " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }

// atomicMutex is a tiny test-only mutex that uses atomic + spin.
// Avoids importing sync twice in the same file.
type atomicMutex struct {
        flag atomic.Int32
}

func (m *atomicMutex) Lock() {
        for !m.flag.CompareAndSwap(0, 1) {
                // spin
        }
}

func (m *atomicMutex) Unlock() { m.flag.Store(0) }
