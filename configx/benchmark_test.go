package configx

import (
	"fmt"
	"testing"
)

// Benchmarks measure hot paths: Get, Value, Scan, Load, Watch notify.
//
// Run with:
//   go test ./configx -bench=. -benchmem

func BenchmarkGet(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{
		"server": {"addr": "0.0.0.0", "port": 8000},
		"database": {"dsn": "postgres://localhost/app"}
	}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get[string](cfg, "server.addr")
	}
}

func BenchmarkGetInt(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"server":{"port":8000}}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get[int](cfg, "server.port")
	}
}

func BenchmarkMustGet(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"server":{"addr":"0.0.0.0"}}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MustGet[string](cfg, "server.addr")
	}
}

func BenchmarkGetOrDefault(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"x":1}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetOrDefault[int](cfg, "missing.key", 42)
	}
}

func BenchmarkValue(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"server":{"addr":"0.0.0.0","port":8000}}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Value("server.addr")
	}
}

func BenchmarkValueString(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"server":{"addr":"0.0.0.0"}}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := cfg.Value("server.addr")
		_, _ = v.String()
	}
}

func BenchmarkValueInt(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{"server":{"port":8000}}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := cfg.Value("server.port")
		_, _ = v.Int()
	}
}

func BenchmarkScan(b *testing.B) {
	cfg := New(WithSource(newJSONSource(`{
		"server": {"addr": "0.0.0.0", "port": 8000},
		"database": {"driver": "postgres", "dsn": "postgres://localhost/app", "max_open_conns": 32}
	}`)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	type ServerConfig struct {
		Addr string `json:"addr"`
		Port int    `json:"port"`
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out ServerConfig
		_ = cfg.Value("server").Scan(&out)
	}
}

func BenchmarkLoad(b *testing.B) {
	data := `{"server":{"addr":"0.0.0.0","port":8000},"database":{"dsn":"postgres://localhost/app"}}`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := New(WithSource(newJSONSource(data)))
		_ = cfg.Load()
		_ = cfg.Close()
	}
}

func BenchmarkLoadAndScan(b *testing.B) {
	data := `{
		"app": {"name": "kernel", "env": "prod", "version": "v1.2.3"},
		"server": {"http": {"addr": "0.0.0.0", "port": 8000}, "grpc": {"addr": "0.0.0.0", "port": 10080}},
		"database": {"driver": "postgres", "dsn": "postgres://localhost/app", "max_open_conns": 32, "max_idle_conns": 8},
		"log": {"level": "info", "format": "json"},
		"features": {"new_home": true, "beta_api": false}
	}`

	type FullConfig struct {
		App struct {
			Name    string `json:"name"`
			Env     string `json:"env"`
			Version string `json:"version"`
		} `json:"app"`
		Server struct {
			HTTP struct {
				Addr string `json:"addr"`
				Port int    `json:"port"`
			} `json:"http"`
		} `json:"server"`
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := New(WithSource(newJSONSource(data)))
		_ = cfg.Load()
		var out FullConfig
		_ = cfg.Scan(&out)
		_ = cfg.Close()
	}
}

func BenchmarkWatchNotify(b *testing.B) {
	src := &mutableJSONSource{data: `{"x":1}`}
	cfg := New(WithSource(src))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}
	calls := 0
	_ = cfg.Watch("x", func(string, Value) { calls++ })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.data = fmt.Sprintf(`{"x":%d}`, i+2)
		_ = cfg.Load()
	}
}

func BenchmarkGetLargeConfig(b *testing.B) {
	// Build a config with ~100 keys.
	data := "{"
	for i := 0; i < 100; i++ {
		if i > 0 {
			data += ","
		}
		data += fmt.Sprintf(`"key%d":"value%d"`, i, i)
	}
	data += "}"

	cfg := New(WithSource(newJSONSource(data)))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get[string](cfg, "key50")
	}
}
