package configx

import (
	"strings"
	"testing"
)

// Fuzz tests ensure that arbitrary inputs do not cause panics or hangs in
// the Load / Value / Scan pipeline.
//
// Run with:
//   go test ./configx -run=^$ -fuzz=FuzzConfigLoad -fuzztime=30s
//   go test ./configx -run=^$ -fuzz=FuzzValueScan -fuzztime=30s

// FuzzConfigLoad feeds arbitrary byte sequences as a JSON source and verifies
// that Load never panics.
func FuzzConfigLoad(f *testing.F) {
	f.Add(`{"a":1}`)
	f.Add(`{"a":{"b":"c"}}`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`{"`)
	f.Add(`{"a":`)
	f.Add(`{"a":"${MISSING}"}`)
	f.Add(`{"a":"${KEY:default}"}`)
	f.Add(`{"a":[1,2,3],"b":{"c":"d"}}`)

	f.Fuzz(func(t *testing.T, data string) {
		cfg := New(WithSource(newJSONSource(data)))
		defer cfg.Close()
		// Load may error but must not panic.
		_ = cfg.Load()

		// Reading various keys must not panic either.
		_, _ = Get[string](cfg, "a")
		_, _ = Get[int](cfg, "a.b")
		_, _ = Get[bool](cfg, "a.b.c")
		_ = GetOrDefault[string](cfg, "missing", "fallback")

		// Value reads.
		_ = cfg.Value("a")
		_ = cfg.Value("a.b")
		_ = cfg.Value("")
		_, _ = cfg.Value("a").String()
		_, _ = cfg.Value("a").Int()
		_, _ = cfg.Value("a").Bool()
		_, _ = cfg.Value("a").Float()
		_, _ = cfg.Value("a").Duration()
		_, _ = cfg.Value("a").Slice()
		_, _ = cfg.Value("a").Map()

		var s struct {
			A any `json:"a"`
		}
		_ = cfg.Scan(&s)
	})
}

// FuzzValueScan feeds arbitrary JSON and target types to Value.Scan.
func FuzzValueScan(f *testing.F) {
	f.Add(`{"x":1}`, "x")
	f.Add(`{"x":"hello"}`, "x")
	f.Add(`{"x":[1,2,3]}`, "x")
	f.Add(`{"x":{"a":"b"}}`, "x")
	f.Add(`{"x":null}`, "x")
	f.Add(`{"x":true}`, "x")
	f.Add(`{"x":3.14}`, "x")
	f.Add(`{"a":{"b":{"c":"deep"}}}`, "a.b.c")
	f.Add(`{}`, "missing")

	f.Fuzz(func(t *testing.T, data, key string) {
		if len(data) > 4096 {
			t.Skip("data too large")
		}
		// Reject obviously non-JSON inputs to keep the corpus focused.
		trimmed := strings.TrimSpace(data)
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			t.Skip("not JSON")
		}

		cfg := New(WithSource(newJSONSource(data)))
		defer cfg.Close()
		if err := cfg.Load(); err != nil {
			return // bad JSON, skip
		}

		v := cfg.Value(key)
		if v.Load() == nil {
			return // missing key, nothing to scan
		}

		// Scan into various targets — none should panic.
		var s string
		_ = v.Scan(&s)
		var i int
		_ = v.Scan(&i)
		var b bool
		_ = v.Scan(&b)
		var m map[string]any
		_ = v.Scan(&m)
		var sl []any
		_ = v.Scan(&sl)
	})
}

// FuzzResolver feeds arbitrary placeholder expressions and verifies the
// resolver does not panic or loop infinitely.
func FuzzResolver(f *testing.F) {
	f.Add(`${KEY}`)
	f.Add(`${KEY:default}`)
	f.Add(`prefix-${KEY}-suffix`)
	f.Add(`${KEY}-${OTHER}`)
	f.Add(`no-placeholder`)
	f.Add(`${`)
	f.Add(`$}`)
	f.Add(`${${${KEY}}}`)

	f.Fuzz(func(t *testing.T, expr string) {
		data := `{"KEY":"value","OTHER":"other","server":{"addr":"` + expr + `"}}`
		cfg := New(WithSource(newJSONSource(data)))
		defer cfg.Close()
		// Load may error but must not panic or hang.
		_ = cfg.Load()
		_, _ = Get[string](cfg, "server.addr")
	})
}
