package configx

import (
	"context"
	"fmt"
	"strings"
)

type exampleSource struct {
	key    string
	format string
	data   string
}

func (s exampleSource) Load() ([]*KeyValue, error) {
	key := s.key
	if key == "" {
		key = "config.json"
	}
	return []*KeyValue{{Key: key, Format: s.format, Value: []byte(s.data)}}, nil
}

func (s exampleSource) Watch() (Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return exampleWatcher{ctx: ctx, cancel: cancel}, nil
}

type exampleWatcher struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (w exampleWatcher) Next() ([]*KeyValue, error) {
	<-w.ctx.Done()
	return nil, w.ctx.Err()
}

func (w exampleWatcher) Stop() error {
	w.cancel()
	return nil
}

func ExampleNew() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"app":{"name":"hub"}}`,
	}))
	defer cfg.Close()

	_ = cfg.Load()
	name, _ := Get[string](cfg, "app.name")
	fmt.Println(name)

	// Output: hub
}

func ExampleGet() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"server":{"port":8080}}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	port, _ := Get[int](cfg, "server.port")
	fmt.Println(port)

	// Output: 8080
}

func ExampleMustGet() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"service":{"name":"aihub"}}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "service.name"))

	// Output: aihub
}

func ExampleGetOrDefault() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"server":{"addr":":8000"}}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(GetOrDefault[int](cfg, "server.port", 8000))

	// Output: 8000
}

func ExampleWithSource() {
	cfg := New(WithSource(
		exampleSource{format: "json", data: `{"app":{"name":"kernel"}}`},
		exampleSource{format: "json", data: `{"app":{"env":"dev"}}`},
	))
	defer cfg.Close()
	_ = cfg.Load()

	name, _ := Get[string](cfg, "app.name")
	env, _ := Get[string](cfg, "app.env")
	fmt.Println(name, env)

	// Output: kernel dev
}

func ExampleWithDecoder() {
	decoder := func(kv *KeyValue, target map[string]any) error {
		parts := strings.SplitN(string(kv.Value), "=", 2)
		if len(parts) == 2 {
			target[parts[0]] = parts[1]
		}
		return nil
	}
	cfg := New(
		WithSource(exampleSource{key: "app.name", data: "name=kernel"}),
		WithDecoder(decoder),
	)
	defer cfg.Close()
	_ = cfg.Load()

	name, _ := Get[string](cfg, "name")
	fmt.Println(name)

	// Output: kernel
}

func ExampleWithResolver() {
	resolver := func(values map[string]any) error {
		values["service"] = map[string]any{"name": "resolved"}
		return nil
	}
	cfg := New(
		WithSource(exampleSource{format: "json", data: `{}`}),
		WithResolver(resolver),
	)
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "service.name"))

	// Output: resolved
}

func ExampleWithResolveActualTypes() {
	cfg := New(
		WithSource(exampleSource{
			format: "json",
			data:   `{"PORT":"8080","server":{"port":"${PORT}"}}`,
		}),
		WithResolveActualTypes(true),
	)
	defer cfg.Close()
	_ = cfg.Load()

	port, _ := Get[int](cfg, "server.port")
	fmt.Printf("%T %d\n", port, port)

	// Output: int 8080
}

func ExampleWithMergeFunc() {
	replaceMerge := func(dst, src any) error {
		d := dst.(*map[string]any)
		*d = src.(map[string]any)
		return nil
	}
	cfg := New(
		WithSource(
			exampleSource{format: "json", data: `{"name":"first"}`},
			exampleSource{format: "json", data: `{"name":"second"}`},
		),
		WithMergeFunc(replaceMerge),
	)
	defer cfg.Close()
	_ = cfg.Load()

	fmt.Println(MustGet[string](cfg, "name"))

	// Output: second
}

func ExampleConfig_Value() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"debug":true}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	debug, _ := cfg.Value("debug").Bool()
	fmt.Println(debug)

	// Output: true
}

func ExampleConfig_Scan() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"server":{"addr":":8000","port":8000}}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	var out struct {
		Server struct {
			Addr string `json:"addr"`
			Port int    `json:"port"`
		} `json:"server"`
	}
	_ = cfg.Scan(&out)
	fmt.Println(out.Server.Addr, out.Server.Port)

	// Output: :8000 8000
}

func ExampleConfig_Watch() {
	cfg := New(WithSource(exampleSource{
		format: "json",
		data:   `{"server":{"port":8000}}`,
	}))
	defer cfg.Close()
	_ = cfg.Load()

	_ = cfg.Watch("server.port", func(key string, value Value) {
		port, _ := value.Int()
		fmt.Println(key, port)
	})
	cfg.(*config).notify("server.port", cfg.Value("server.port"))

	// Output: server.port 8000
}

func ExampleValue_Bool() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"enabled":"true"}`}))
	defer cfg.Close()
	_ = cfg.Load()
	v, _ := cfg.Value("enabled").Bool()
	fmt.Println(v)

	// Output: true
}

func ExampleValue_Int() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"port":"8080"}`}))
	defer cfg.Close()
	_ = cfg.Load()
	v, _ := cfg.Value("port").Int()
	fmt.Println(v)

	// Output: 8080
}

func ExampleValue_Float() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"ratio":"0.75"}`}))
	defer cfg.Close()
	_ = cfg.Load()
	v, _ := cfg.Value("ratio").Float()
	fmt.Println(v)

	// Output: 0.75
}

func ExampleValue_String() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"name":123}`}))
	defer cfg.Close()
	_ = cfg.Load()
	v, _ := cfg.Value("name").String()
	fmt.Println(v)

	// Output: 123
}

func ExampleValue_Slice() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"endpoints":["a","b"]}`}))
	defer cfg.Close()
	_ = cfg.Load()
	items, _ := cfg.Value("endpoints").Slice()
	for _, item := range items {
		v, _ := item.String()
		fmt.Println(v)
	}

	// Output:
	// a
	// b
}

func ExampleValue_Map() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"server":{"addr":":8000"}}`}))
	defer cfg.Close()
	_ = cfg.Load()
	m, _ := cfg.Value("server").Map()
	addr, _ := m["addr"].String()
	fmt.Println(addr)

	// Output: :8000
}

func ExampleValue_Scan() {
	cfg := New(WithSource(exampleSource{format: "json", data: `{"server":{"addr":":8000"}}`}))
	defer cfg.Close()
	_ = cfg.Load()
	var server struct {
		Addr string `json:"addr"`
	}
	_ = cfg.Value("server").Scan(&server)
	fmt.Println(server.Addr)

	// Output: :8000
}
