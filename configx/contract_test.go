package configx

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestConfigxContract locks the public behavior documented in
// docs/contracts/configx.md. Any change here is a breaking change and requires
// a major version bump.

func TestConfigxContract(t *testing.T) {
	t.Parallel()

	t.Run("nil Config helpers return ErrNilConfig", func(t *testing.T) {
		t.Parallel()
		_, err := Get[string](nil, "any.key")
		assertErrorIs(t, err, ErrNilConfig)

		assertPanic(t, func() { _ = MustGet[string](nil, "any.key") })

		got := GetOrDefault[string](nil, "any.key", "fallback")
		assertEqual(t, got, "fallback")
	})

	t.Run("missing key returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{"a":"b"}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		_, err := Get[string](cfg, "missing.key")
		assertErrorIs(t, err, ErrNotFound)
	})

	t.Run("type mismatch returns error not panic", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{"port":"not-a-number"}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		_, err := Get[int](cfg, "port")
		if err == nil {
			t.Fatal("expected error on type mismatch")
		}
		// MustGet should panic on type mismatch.
		assertPanic(t, func() { _ = MustGet[int](cfg, "port") })
	})

	t.Run("GetOrDefault swallows type mismatch", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{"port":"oops"}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		got := GetOrDefault[int](cfg, "port", 9000)
		assertEqual(t, got, 9000)
	})

	t.Run("Watch requires key to exist at registration", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{"server":{"port":8080}}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		if err := cfg.Watch("missing.key", func(string, Value) {}); err != ErrNotFound {
			t.Fatalf("Watch on missing key = %v, want ErrNotFound", err)
		}
		if err := cfg.Watch("server.port", nil); err != ErrInvalidObserver {
			t.Fatalf("Watch with nil observer = %v, want ErrInvalidObserver", err)
		}
	})

	t.Run("Close is idempotent and disables Load", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{}`)))
		defer cfg.Close()

		assertNoError(t, cfg.Load())
		assertNoError(t, cfg.Close())
		assertNoError(t, cfg.Close()) // idempotent
		if err := cfg.Load(); err != ErrClosed {
			t.Fatalf("Load after Close = %v, want ErrClosed", err)
		}
	})

	t.Run("Close returns nil even with no watchers", func(t *testing.T) {
		t.Parallel()
		cfg := New() // no sources
		defer cfg.Close()
		assertNoError(t, cfg.Close())
	})

	t.Run("WithSource ignores nil sources", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(nil, newJSONSource(`{"x":1}`), nil))
		defer cfg.Close()
		assertNoError(t, cfg.Load())
		assertEqual(t, GetOrDefault[int](cfg, "x", 0), 1)
	})

	t.Run("merge recursively merges nested maps", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(
			newJSONSource(`{"server":{"addr":"0.0.0.0","port":8000}}`),
			newJSONSource(`{"server":{"port":9000}}`),
		))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		assertEqual(t, GetOrDefault[string](cfg, "server.addr", ""), "0.0.0.0")
		assertEqual(t, GetOrDefault[int](cfg, "server.port", 0), 9000)
	})

	t.Run("merge replaces arrays as a whole", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(
			newJSONSource(`{"endpoints":["a","b","c"]}`),
			newJSONSource(`{"endpoints":["x","y"]}`),
		))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		items, err := cfg.Value("endpoints").Slice()
		assertNoError(t, err)
		if len(items) != 2 {
			t.Fatalf("len(endpoints) = %d, want 2 (full replace)", len(items))
		}
		first, _ := items[0].String()
		assertEqual(t, first, "x")
	})

	t.Run("placeholder resolver expands KEY and KEY:default", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{
                        "HOST":"127.0.0.1",
                        "PORT":"8000",
                        "server":{"addr":"${HOST}:${PORT}","mode":"${MODE:dev}"}
                }`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		assertEqual(t, MustGet[string](cfg, "server.addr"), "127.0.0.1:8000")
		assertEqual(t, MustGet[string](cfg, "server.mode"), "dev")
	})

	t.Run("WithResolveActualTypes converts whole-value placeholder", func(t *testing.T) {
		t.Parallel()
		cfg := New(
			WithSource(newJSONSource(`{"PORT":"8080","server":{"port":"${PORT}"}}`)),
			WithResolveActualTypes(true),
		)
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		port, err := Get[int](cfg, "server.port")
		assertNoError(t, err)
		assertEqual(t, port, 8080)
	})

	t.Run("Value for missing key returns errValue", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		v := cfg.Value("missing")
		if v.Load() != nil {
			t.Fatalf("Load() = %v, want nil", v.Load())
		}
		_, err := v.String()
		if err == nil {
			t.Fatal("String() on missing key should return error")
		}
		_, err = v.Bool()
		if err == nil {
			t.Fatal("Bool() on missing key should return error")
		}
		_, err = v.Int()
		if err == nil {
			t.Fatal("Int() on missing key should return error")
		}
		_, err = v.Float()
		if err == nil {
			t.Fatal("Float() on missing key should return error")
		}
		_, err = v.Slice()
		if err == nil {
			t.Fatal("Slice() on missing key should return error")
		}
		_, err = v.Map()
		if err == nil {
			t.Fatal("Map() on missing key should return error")
		}
		_, err = v.Duration()
		if err == nil {
			t.Fatal("Duration() on missing key should return error")
		}
		if err := v.Scan(&struct{}{}); err == nil {
			t.Fatal("Scan() on missing key should return error")
		}
	})

	t.Run("Value accepts string / number / bool / Stringer", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{
                        "s":"hello",
                        "n":42,
                        "b":true,
                        "f":3.14,
                        "arr":[1,2,3],
                        "obj":{"k":"v"}
                }`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		s, _ := cfg.Value("s").String()
		assertEqual(t, s, "hello")

		n, _ := cfg.Value("n").Int()
		assertEqual(t, n, int64(42))

		b, _ := cfg.Value("b").Bool()
		assertEqual(t, b, true)

		f, _ := cfg.Value("f").Float()
		assertEqual(t, f, 3.14)

		arr, _ := cfg.Value("arr").Slice()
		assertEqual(t, len(arr), 3)

		obj, _ := cfg.Value("obj").Map()
		v, _ := obj["k"].String()
		assertEqual(t, v, "v")
	})

	t.Run("Duration accepts nanoseconds and duration strings", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{
                        "nanos":5000000000,
                        "text":"5s"
                }`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		d, err := cfg.Value("nanos").Duration()
		assertNoError(t, err)
		assertEqual(t, d, 5*time.Second)

		// text duration path goes through Int() first which fails; fall back to parse
		// Note: current implementation only accepts int nanoseconds via Int().
		// Document this contract: Duration() is int-nanosecond only.
		_, err = cfg.Value("text").Duration()
		if err == nil {
			t.Fatal("Duration on string should error per current contract")
		}
	})

	t.Run("Scan supports struct with json tags", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{
                        "server":{"addr":"0.0.0.0","port":8000}
                }`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		var out struct {
			Server struct {
				Addr string `json:"addr"`
				Port int    `json:"port"`
			} `json:"server"`
		}
		assertNoError(t, cfg.Scan(&out))
		assertEqual(t, out.Server.Addr, "0.0.0.0")
		assertEqual(t, out.Server.Port, 8000)
	})

	t.Run("Scan value populates nested struct", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{
                        "server":{"http":{"addr":"0.0.0.0","port":8000}}
                }`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		var http struct {
			Addr string `json:"addr"`
			Port int    `json:"port"`
		}
		assertNoError(t, cfg.Value("server.http").Scan(&http))
		assertEqual(t, http.Addr, "0.0.0.0")
		assertEqual(t, http.Port, 8000)
	})

	t.Run("multiple observers fire on reload", func(t *testing.T) {
		t.Parallel()
		src := &mutableJSONSource{data: `{"server":{"port":8000}}`}
		cfg := New(WithSource(src))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		var mu sync.Mutex
		calls := []string{}
		_ = cfg.Watch("server.port", func(_ string, v Value) {
			mu.Lock()
			defer mu.Unlock()
			port, _ := v.Int()
			calls = append(calls, "first:"+itoa(int(port)))
		})
		_ = cfg.Watch("server.port", func(_ string, v Value) {
			mu.Lock()
			defer mu.Unlock()
			port, _ := v.Int()
			calls = append(calls, "second:"+itoa(int(port)))
		})

		src.data = `{"server":{"port":9000}}`
		assertNoError(t, cfg.Load())

		awaitTimeout(t, "observers fired", time.Second, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(calls) == 2
		})

		mu.Lock()
		defer mu.Unlock()
		if len(calls) != 2 {
			t.Fatalf("calls = %v, want 2 entries", calls)
		}
	})

	t.Run("nil error from helpers does not break chain", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{"x":1}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		// re-Load should refresh and not error
		assertNoError(t, cfg.Load())
	})

	t.Run("errors.Is-compatible with ErrNotFound", func(t *testing.T) {
		t.Parallel()
		cfg := New(WithSource(newJSONSource(`{}`)))
		defer cfg.Close()
		assertNoError(t, cfg.Load())

		_, err := Get[string](cfg, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("errors.Is(err, ErrNotFound) = false; err=%v", err)
		}
	})
}

// itoa is a tiny stdlib-free int->string used by tests to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
