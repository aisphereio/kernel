package configx

import (
	"fmt"
	"strings"
)

func Example_businessBootstrapHTTPServer() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"server":{"http":{"addr":":8000"}}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	addr := MustGet[string](cfg, "server.http.addr")
	fmt.Println("listen", addr)

	// Output: listen :8000
}

func Example_businessLayeredFileAndEnv() {
	cfg := New(WithSource(
		exampleSource{format: "json", data: `{"app":{"name":"kernel","env":"dev"}}`},
		exampleSource{key: "app.env", data: "prod"},
	))
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "app.name"), MustGet[string](cfg, "app.env"))

	// Output: kernel prod
}

func Example_businessScanDatabaseConfig() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"database":{"driver":"postgres","max_idle":8}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	var db struct {
		Driver  string `json:"driver"`
		MaxIdle int    `json:"max_idle"`
	}
	_ = cfg.Value("database").Scan(&db)
	fmt.Println(db.Driver, db.MaxIdle)

	// Output: postgres 8
}

func Example_businessFeatureFlag() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"features":{"new_home":"true"}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	enabled, _ := cfg.Value("features.new_home").Bool()
	fmt.Println("new_home", enabled)

	// Output: new_home true
}

func Example_businessUpstreamTimeoutConfig() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"upstream":{"timeout_ns":5000000000}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	timeout, _ := cfg.Value("upstream.timeout_ns").Duration()
	fmt.Println(timeout.String())

	// Output: 5s
}

func Example_businessResolvePlaceholder() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"HOST":"127.0.0.1","server":{"addr":"${HOST}:8000"}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "server.addr"))

	// Output: 127.0.0.1:8000
}

func Example_businessActualTypedPlaceholder() {
	cfg := New(
		WithSource(exampleSource{format: "json", data: `{"PORT":"9000","server":{"port":"${PORT}"}}`}),
		WithResolveActualTypes(true),
	)
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[int](cfg, "server.port") + 1)

	// Output: 9001
}

func Example_businessObserveRuntimeChange() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"log":{"level":"info"}}`}))
	defer cfg.Close()
	_ = cfg.Load()

	_ = cfg.Watch("log.level", func(_ string, value Value) {
		level, _ := value.String()
		fmt.Println("reload", level)
	})
	cfg.(*config).Value("log.level").Store("debug")
	cfg.(*config).notify("log.level", cfg.Value("log.level"))

	// Output: reload debug
}

func Example_businessValidateRequiredConfig() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{}`}))
	defer cfg.Close()
	_ = cfg.Load()

	_, err := Get[string](cfg, "jwt.secret")
	fmt.Println(err == ErrNotFound)

	// Output: true
}

func Example_businessCustomDecoderForDotEnv() {
	decoder := func(kv *KeyValue, target map[string]any) error {
		for _, line := range strings.Split(string(kv.Value), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok {
				target[key] = value
			}
		}
		return nil
	}
	cfg := New(
		WithSource(exampleSource{key: ".env", data: "APP_NAME=kernel\nAPP_ENV=dev"}),
		WithDecoder(decoder),
	)
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "APP_NAME"), MustGet[string](cfg, "APP_ENV"))

	// Output: kernel dev
}
