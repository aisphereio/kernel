package configx

import (
	"context"
	"testing"
)

type mutableJSONSource struct {
	data string
}

func (s *mutableJSONSource) Load() ([]*KeyValue, error) {
	return []*KeyValue{{Key: "config.json", Value: []byte(s.data), Format: "json"}}, nil
}

func (s *mutableJSONSource) Watch() (Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &blockingWatcher{ctx: ctx, cancel: cancel}, nil
}

type blockingWatcher struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (w *blockingWatcher) Next() ([]*KeyValue, error) {
	<-w.ctx.Done()
	return nil, w.ctx.Err()
}

func (w *blockingWatcher) Stop() error {
	w.cancel()
	return nil
}

func TestLoadRefreshesCachedValueAfterTypeChange(t *testing.T) {
	src := &mutableJSONSource{data: `{"feature":{"enabled":"true"}}`}
	c := New(WithSource(src))
	defer c.Close()

	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	v := c.Value("feature.enabled")
	if got, err := v.Bool(); err != nil || !got {
		t.Fatalf("Bool() = %v, %v; want true, nil", got, err)
	}

	src.data = `{"feature":{"enabled":false}}`
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Bool(); err != nil || got {
		t.Fatalf("cached Bool() after reload = %v, %v; want false, nil", got, err)
	}
}

func TestWatchAllowsMultipleObservers(t *testing.T) {
	src := &mutableJSONSource{data: `{"server":{"port":8080}}`}
	c := New(WithSource(src))
	defer c.Close()

	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := c.Watch("server.port", func(string, Value) { calls++ }); err != nil {
		t.Fatal(err)
	}
	if err := c.Watch("server.port", func(string, Value) { calls++ }); err != nil {
		t.Fatal(err)
	}

	src.data = `{"server":{"port":9090}}`
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("observer calls = %d, want 2", calls)
	}
	if got := GetOrDefault[int](c, "server.port", 0); got != 9090 {
		t.Fatalf("GetOrDefault() = %d, want 9090", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	c := New(WithSource(&mutableJSONSource{data: `{}`}))
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Load(); err != ErrClosed {
		t.Fatalf("Load() after Close = %v, want ErrClosed", err)
	}
}

func TestGetNilConfig(t *testing.T) {
	if _, err := Get[string](nil, "app.name"); err != ErrNilConfig {
		t.Fatalf("Get(nil) = %v, want ErrNilConfig", err)
	}
}
