package configx

import (
	"sync"
	"testing"
	"time"
)

// Test helpers shared across configx test files.
//
// These helpers exist so that contract_test.go, integration_test.go,
// coverage_edge_test.go, benchmark_test.go, and fuzz_test.go can build
// consistent fixtures without duplicating setup code.

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %v, got nil", target)
	}
	if !errorIs(err, target) {
		t.Fatalf("error %v does not match %v", err, target)
	}
}

func errorIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// newJSONSource returns an in-memory Source that emits a single JSON KeyValue.
// It is the standard fixture for example/contract/integration tests.
func newJSONSource(data string) Source {
	return newMemorySource("config.json", "json", data)
}

// newYAMLSource returns an in-memory Source that emits a single YAML KeyValue.
func newYAMLSource(data string) Source {
	return newMemorySource("config.yaml", "yaml", data)
}

// newMemorySource returns a Source that always returns the same KeyValue.
// Watch returns a blockingWatcher that never fires; suitable for tests that
// only exercise Load.
func newMemorySource(key, format, data string) Source {
	return &memorySource{key: key, format: format, data: data}
}

type memorySource struct {
	key    string
	format string
	data   string
}

func (s *memorySource) Load() ([]*KeyValue, error) {
	return []*KeyValue{{Key: s.key, Format: s.format, Value: []byte(s.data)}}, nil
}

func (s *memorySource) Watch() (Watcher, error) {
	return newBlockingWatcher(), nil
}

// newBlockingWatcher returns a Watcher that blocks forever until Stop.
func newBlockingWatcher() Watcher {
	return &blockingWatcherCtx{stop: make(chan struct{})}
}

type blockingWatcherCtx struct {
	stop chan struct{}
	once sync.Once
}

func (w *blockingWatcherCtx) Next() ([]*KeyValue, error) {
	<-w.stop
	return nil, nil
}

func (w *blockingWatcherCtx) Stop() error {
	w.once.Do(func() { close(w.stop) })
	return nil
}

// newNotifyWatcher returns a Watcher that emits the supplied KeyValue on demand.
// Used by integration tests that need to simulate runtime config changes.
func newNotifyWatcher(initial string) (*notifyWatcher, func(string)) {
	w := &notifyWatcher{
		ch: make(chan []*KeyValue, 8),
	}
	if initial != "" {
		w.ch <- []*KeyValue{{Key: "config.json", Format: "json", Value: []byte(initial)}}
	}
	return w, func(data string) {
		w.ch <- []*KeyValue{{Key: "config.json", Format: "json", Value: []byte(data)}}
	}
}

type notifyWatcher struct {
	ch   chan []*KeyValue
	stop chan struct{}
	once sync.Once
}

func (w *notifyWatcher) Next() ([]*KeyValue, error) {
	select {
	case kvs := <-w.ch:
		return kvs, nil
	case <-w.stop:
		return nil, nil
	}
}

func (w *notifyWatcher) Stop() error {
	w.once.Do(func() { close(w.stop) })
	return nil
}

// awaitTimeout waits for cond to return true or fails the test.
func awaitTimeout(t *testing.T, name string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s: condition not met within %v", name, timeout)
}

// assertPanic fails the test if fn does not panic.
func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	fn()
}
