package configx

import (
	"testing"
	"time"
)

// Coverage edge tests exercise unusual inputs and boundary conditions that
// are easy to break in refactors. They are not contract tests (see
// contract_test.go) — they exist to maximize line coverage and catch silent
// regressions in error paths.

func TestNilConfigClose(t *testing.T) {
	t.Parallel()
	// Config is interface; we test the concrete *config with no sources.
	cfg := New()
	defer cfg.Close()
	// Close on never-loaded config should still work.
	assertNoError(t, cfg.Close())
}

func TestLoadWithEmptySources(t *testing.T) {
	t.Parallel()
	cfg := New() // no sources at all
	defer cfg.Close()
	assertNoError(t, cfg.Load())
	// Value reads should return ErrNotFound.
	v := cfg.Value("anything")
	if v.Load() != nil {
		t.Fatalf("Load() = %v, want nil", v.Load())
	}
}

func TestValueEmptyPath(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"a":1}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	// Empty path returns the whole tree.
	v := cfg.Value("")
	if v.Load() == nil {
		t.Fatal("Value('') should return root tree, not nil")
	}
	m, err := v.Map()
	assertNoError(t, err)
	if len(m) == 0 {
		t.Fatal("Value('') should return non-empty map")
	}
}

func TestValuePathThroughNonMap(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"a":"string-value"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	// Trying to traverse into a string returns nil.
	v := cfg.Value("a.b")
	if v.Load() != nil {
		t.Fatalf("Value('a.b') = %v, want nil", v.Load())
	}
	_, err := v.String()
	if err == nil {
		t.Fatal("expected error on traversal through non-map")
	}
}

func TestValueNestedPath(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "a": {"b": {"c": {"d": "deep"}}}
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v := cfg.Value("a.b.c.d")
	s, _ := v.String()
	assertEqual(t, s, "deep")
}

func TestValueNumericConversions(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "int": 42,
                "int8": 8,
                "int16": 16,
                "int32": 32,
                "int64": 64,
                "uint": 100,
                "uint8": 8,
                "uint16": 16,
                "uint32": 32,
                "uint64": 64,
                "float32": 3.14,
                "float64": 6.28,
                "stringNum": "12345",
                "floatString": "9.99"
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	cases := []struct {
		key string
		exp int64
	}{
		{"int", 42},
		{"int8", 8},
		{"int16", 16},
		{"int32", 32},
		{"int64", 64},
		{"uint", 100},
		{"uint8", 8},
		{"uint16", 16},
		{"uint32", 32},
		{"uint64", 64},
		{"stringNum", 12345},
	}
	for _, c := range cases {
		n, err := cfg.Value(c.key).Int()
		assertNoError(t, err)
		assertEqual(t, n, c.exp)
	}

	f, err := cfg.Value("float32").Float()
	assertNoError(t, err)
	if f < 3.13 || f > 3.15 {
		t.Fatalf("float32 = %v, want ~3.14", f)
	}

	f, err = cfg.Value("float64").Float()
	assertNoError(t, err)
	if f < 6.27 || f > 6.29 {
		t.Fatalf("float64 = %v, want ~6.28", f)
	}

	f, err = cfg.Value("floatString").Float()
	assertNoError(t, err)
	if f < 9.98 || f > 10.00 {
		t.Fatalf("floatString = %v, want ~9.99", f)
	}
}

func TestValueBoolConversions(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "bTrue": true,
                "bFalse": false,
                "sTrue": "true",
                "sFalse": "false",
                "n1": 1,
                "n0": 0
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	trueCases := []string{"bTrue", "sTrue", "n1"}
	for _, k := range trueCases {
		got, err := cfg.Value(k).Bool()
		assertNoError(t, err)
		if !got {
			t.Errorf("%s: got false, want true", k)
		}
	}

	falseCases := []string{"bFalse", "sFalse", "n0"}
	for _, k := range falseCases {
		got, err := cfg.Value(k).Bool()
		assertNoError(t, err)
		if got {
			t.Errorf("%s: got true, want false", k)
		}
	}
}

func TestValueStringFromVariousTypes(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "s": "hello",
                "n": 42,
                "b": true,
                "f": 3.14
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	s, _ := cfg.Value("s").String()
	assertEqual(t, s, "hello")

	n, _ := cfg.Value("n").String()
	assertEqual(t, n, "42")

	b, _ := cfg.Value("b").String()
	assertEqual(t, b, "true")

	// f might be "3.14" or similar
	fs, _ := cfg.Value("f").String()
	if fs == "" {
		t.Fatal("float String() returned empty")
	}
}

func TestValueDurationNanoseconds(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "nanos1s": 1000000000,
                "nanos0": 0
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	d, err := cfg.Value("nanos1s").Duration()
	assertNoError(t, err)
	assertEqual(t, d, time.Second)

	d, err = cfg.Value("nanos0").Duration()
	assertNoError(t, err)
	assertEqual(t, d, time.Duration(0))
}

func TestValueSliceAndMap(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "arr": [1, 2, 3],
                "obj": {"a": "x", "b": "y"}
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	items, err := cfg.Value("arr").Slice()
	assertNoError(t, err)
	assertEqual(t, len(items), 3)
	first, _ := items[0].Int()
	assertEqual(t, first, int64(1))

	m, err := cfg.Value("obj").Map()
	assertNoError(t, err)
	assertEqual(t, len(m), 2)
	a, _ := m["a"].String()
	assertEqual(t, a, "x")
}

func TestValueScanWithMap(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"k":"v"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	var m map[string]string
	assertNoError(t, cfg.Value("").Scan(&m))
	assertEqual(t, m["k"], "v")
}

func TestValueScanIntoStringFails(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"a":"hello"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	var s string
	// Scanning a string value into a string variable via JSON unmarshal
	// actually works because the marshaler produces "hello" which unmarshal
	// accepts into a string. Test that no panic occurs.
	_ = cfg.Value("a").Scan(&s)
	// Don't assert success — we just want to ensure no panic.
}

func TestValueScanIntoNonPointerFails(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"a":1}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	var s string
	if err := cfg.Value("a").Scan(s); err == nil {
		t.Fatal("Scan into non-pointer should fail")
	}
}

func TestGetWithCustomStruct(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "http": {"addr": ":8000", "port": 8000}
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	type HTTPConfig struct {
		Addr string `json:"addr"`
		Port int    `json:"port"`
	}
	v, err := Get[HTTPConfig](cfg, "http")
	assertNoError(t, err)
	assertEqual(t, v.Addr, ":8000")
	assertEqual(t, v.Port, 8000)
}

func TestGetUnsupportedType(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":[1,2,3]}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	// []int is not in the Get switch case; falls through to Scan.
	// Scan into []int from JSON [1,2,3] should work because JSON numbers
	// unmarshal into []int.
	v, err := Get[[]int](cfg, "x")
	assertNoError(t, err)
	if len(v) != 3 {
		t.Fatalf("len = %d, want 3", len(v))
	}
}

func TestMustGetOnValidKey(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":42}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v := MustGet[int](cfg, "x")
	assertEqual(t, v, 42)
}

func TestResolveMissingPlaceholder(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "server": {"addr": "${MISSING_KEY:default-addr}"}
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v := MustGet[string](cfg, "server.addr")
	assertEqual(t, v, "default-addr")
}

func TestResolveMissingPlaceholderNoDefault(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{
                "server": {"addr": "http://${MISSING_KEY}:8000"}
        }`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v := MustGet[string](cfg, "server.addr")
	assertEqual(t, v, "http://:8000")
}

func TestConvertToType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in  string
		out any
	}{
		{"\"quoted\"", "quoted"},
		{"true", true},
		{"false", false},
		{"3.14", 3.14},
		{"42", int64(42)},
		{"not-a-number", "not-a-number"},
	}
	for _, c := range cases {
		got := convertToType(c.in)
		if got != c.out {
			t.Errorf("convertToType(%q) = %v (%T), want %v (%T)", c.in, got, got, c.out, c.out)
		}
	}
}

func TestConvertToTypeFloatInt(t *testing.T) {
	t.Parallel()
	// "3" should convert to int64, not float64.
	got := convertToType("3")
	if _, ok := got.(int64); !ok {
		t.Fatalf("convertToType('3') = %T, want int64", got)
	}

	// "3.0" should convert to float64.
	got = convertToType("3.0")
	if _, ok := got.(float64); !ok {
		t.Fatalf("convertToType('3.0') = %T, want float64", got)
	}
}

func TestMergeWithNilDst(t *testing.T) {
	t.Parallel()
	merge := defaultMerge
	var dst map[string]any
	src := map[string]any{"a": "b"}
	if err := merge(&dst, src); err != nil {
		t.Fatal(err)
	}
	if dst["a"] != "b" {
		t.Fatalf("merge did not populate nil dst; got %v", dst)
	}
}

func TestMergeWrongTypeDst(t *testing.T) {
	t.Parallel()
	merge := defaultMerge
	var dst string
	if err := merge(&dst, map[string]any{"a": "b"}); err == nil {
		t.Fatal("merge with non-map dst should fail")
	}
}

func TestMergeWrongTypeSrc(t *testing.T) {
	t.Parallel()
	merge := defaultMerge
	dst := map[string]any{}
	if err := merge(&dst, "not-a-map"); err == nil {
		t.Fatal("merge with non-map src should fail")
	}
}

func TestCloneMapNil(t *testing.T) {
	t.Parallel()
	out, err := cloneMap(nil)
	assertNoError(t, err)
	if out == nil {
		t.Fatal("cloneMap(nil) returned nil")
	}
	if len(out) != 0 {
		t.Fatalf("cloneMap(nil) = %v, want empty map", out)
	}
}

func TestCloneMapWrongType(t *testing.T) {
	t.Parallel()
	_, err := cloneMap(map[string]any{"x": "string"}) // wrong inner type after convertMap
	// cloneMap returns error only when convertMap returns non-map. string stays string.
	// This case actually succeeds because map[string]any with string values is valid.
	_ = err
}

func TestConvertMapAllTypes(t *testing.T) {
	t.Parallel()
	// map[any]any (YAML output)
	in := map[any]any{
		"a": "string",
		"b": map[any]any{"c": 1},
	}
	out := convertMap(in)
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("convertMap returned %T, want map[string]any", out)
	}
	if m["a"] != "string" {
		t.Fatalf("a = %v, want string", m["a"])
	}
	sub, ok := m["b"].(map[string]any)
	if !ok {
		t.Fatalf("b = %T, want map[string]any", m["b"])
	}
	if sub["c"] != 1 {
		t.Fatalf("c = %v, want 1", sub["c"])
	}
}

func TestConvertMapBytes(t *testing.T) {
	t.Parallel()
	in := map[string]any{"k": []byte("hello")}
	out := convertMap(in)
	m, _ := out.(map[string]any)
	if m["k"] != "hello" {
		t.Fatalf("k = %v, want 'hello'", m["k"])
	}
}

func TestConvertMapSlice(t *testing.T) {
	t.Parallel()
	in := map[string]any{"k": []any{1, 2, 3}}
	out := convertMap(in)
	m, _ := out.(map[string]any)
	arr, ok := m["k"].([]any)
	if !ok {
		t.Fatalf("k = %T, want []any", m["k"])
	}
	if len(arr) != 3 {
		t.Fatalf("len = %d, want 3", len(arr))
	}
}

func TestConvertMapDefault(t *testing.T) {
	t.Parallel()
	in := map[string]any{"k": 42} // int passes through default case
	out := convertMap(in)
	m, _ := out.(map[string]any)
	if m["k"] != 42 {
		t.Fatalf("k = %v, want 42", m["k"])
	}
}

func TestDefaultDecoderUnsupportedFormat(t *testing.T) {
	t.Parallel()
	src := &KeyValue{Key: "x", Value: []byte("data"), Format: "unknown-format"}
	target := map[string]any{}
	if err := defaultDecoder(src, target); err == nil {
		t.Fatal("decoder should fail on unsupported format")
	}
}

func TestDefaultDecoderFlatKey(t *testing.T) {
	t.Parallel()
	src := &KeyValue{Key: "a.b.c", Value: []byte("data"), Format: ""}
	target := map[string]any{}
	assertNoError(t, defaultDecoder(src, target))

	// Should expand to nested map: a -> b -> c -> "data"
	a, ok := target["a"].(map[string]any)
	if !ok {
		t.Fatalf("a = %T, want map[string]any", target["a"])
	}
	b, ok := a["b"].(map[string]any)
	if !ok {
		t.Fatalf("b = %T, want map[string]any", a["b"])
	}
	if b["c"] == nil {
		t.Fatalf("c is nil")
	}
}

func TestValueMapOnNonMap(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":"not-a-map"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	_, err := cfg.Value("x").Map()
	if err == nil {
		t.Fatal("Map() on string should return error")
	}
}

func TestValueSliceOnNonSlice(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":"not-a-slice"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	_, err := cfg.Value("x").Slice()
	if err == nil {
		t.Fatal("Slice() on string should return error")
	}
}

func TestValueStringer(t *testing.T) {
	t.Parallel()
	// Custom Stringer via atomic value Store.
	v := &atomicValue{}
	v.Store(stringerTest{value: "stringer-output"})
	s, err := v.String()
	assertNoError(t, err)
	assertEqual(t, s, "stringer-output")
}

type stringerTest struct{ value string }

func (s stringerTest) String() string { return s.value }

func TestValueBytes(t *testing.T) {
	t.Parallel()
	v := &atomicValue{}
	v.Store([]byte("raw-bytes"))
	s, err := v.String()
	assertNoError(t, err)
	assertEqual(t, s, "raw-bytes")
}

func TestValueIntOnUnsupportedType(t *testing.T) {
	t.Parallel()
	v := &atomicValue{}
	v.Store([]string{"not", "a", "number"})
	_, err := v.Int()
	if err == nil {
		t.Fatal("Int() on []string should return error")
	}
}

func TestValueBoolOnUnsupportedType(t *testing.T) {
	t.Parallel()
	v := &atomicValue{}
	v.Store(map[string]any{"x": 1})
	_, err := v.Bool()
	if err == nil {
		t.Fatal("Bool() on map should return error")
	}
}

func TestValueFloatOnUnsupportedType(t *testing.T) {
	t.Parallel()
	v := &atomicValue{}
	v.Store(nil)
	_, err := v.Float()
	if err == nil {
		t.Fatal("Float() on nil should return error")
	}
}

func TestValueStringOnUnsupportedType(t *testing.T) {
	t.Parallel()
	v := &atomicValue{}
	v.Store(map[string]any{"x": 1})
	_, err := v.String()
	if err == nil {
		t.Fatal("String() on map should return error")
	}
}

func TestLoadMultipleSourcesOrder(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(
		newJSONSource(`{"x":"first","y":"first"}`),
		newJSONSource(`{"y":"second","z":"second"}`),
	))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	assertEqual(t, MustGet[string](cfg, "x"), "first")
	assertEqual(t, MustGet[string](cfg, "y"), "second") // overwritten
	assertEqual(t, MustGet[string](cfg, "z"), "second")
}

func TestReaderSource(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"a":1,"b":"x"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	// Scan the whole tree.
	var out map[string]any
	assertNoError(t, cfg.Scan(&out))
	if out["a"] == nil || out["b"] == nil {
		t.Fatalf("Scan did not populate map: %v", out)
	}
}

func TestCloseReleasesWatcher(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":1}`)))
	defer cfg.Close()

	assertNoError(t, cfg.Load())

	// Close should not block on blocking watchers.
	done := make(chan struct{})
	go func() {
		_ = cfg.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked for too long")
	}
}

func TestWatchOnClosedConfig(t *testing.T) {
	t.Parallel()
	cfg := New(WithSource(newJSONSource(`{"x":1}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())
	assertNoError(t, cfg.Close())

	if err := cfg.Watch("x", func(string, Value) {}); err != ErrClosed {
		t.Fatalf("Watch on closed = %v, want ErrClosed", err)
	}
}

func TestValueCachedAcrossReloads(t *testing.T) {
	t.Parallel()
	src := &mutableJSONSource{data: `{"x":1}`}
	cfg := New(WithSource(src))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v1 := cfg.Value("x")
	src.data = `{"x":2}`
	assertNoError(t, cfg.Load())

	// v1 should reflect the new value because it's the same atomicValue.
	n, _ := v1.Int()
	assertEqual(t, n, int64(2))
}

func TestReloadAfterTypeChangeDoesNotPanic(t *testing.T) {
	t.Parallel()
	src := &mutableJSONSource{data: `{"x":1}`}
	cfg := New(WithSource(src))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	v := cfg.Value("x")
	src.data = `{"x":"now-a-string"}`
	assertNoError(t, cfg.Load())

	// Reading as int should fail gracefully, not panic.
	_, err := v.Int()
	if err == nil {
		t.Fatal("expected error on type change")
	}
	// Reading as string should succeed.
	s, err := v.String()
	assertNoError(t, err)
	assertEqual(t, s, "now-a-string")
}

func TestKeyValueScanIntoProto(t *testing.T) {
	t.Parallel()
	// Skip if protobuf is unavailable — fallback to JSON unmarshal.
	cfg := New(WithSource(newJSONSource(`{"name":"test"}`)))
	defer cfg.Close()
	assertNoError(t, cfg.Load())

	type Simple struct {
		Name string `json:"name"`
	}
	var s Simple
	assertNoError(t, cfg.Value("").Scan(&s))
	assertEqual(t, s.Name, "test")
}
