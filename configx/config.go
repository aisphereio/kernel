package configx

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	// init encoding
	_ "github.com/aisphereio/kernel/encodingx/json"
	_ "github.com/aisphereio/kernel/encodingx/proto"
	_ "github.com/aisphereio/kernel/encodingx/xml"
	_ "github.com/aisphereio/kernel/encodingx/yaml"
	"github.com/aisphereio/kernel/logx"
)

var _ Config = (*config)(nil)

var (
	// ErrNotFound is returned when a key does not exist in the loaded config tree.
	ErrNotFound = errors.New("configx: key not found")
	// ErrClosed is returned when a closed Config is used for a new load/watch operation.
	ErrClosed = errors.New("configx: config is closed")
	// ErrInvalidObserver is returned when Watch is called with a nil observer.
	ErrInvalidObserver = errors.New("configx: observer is nil")
	// ErrNilConfig is returned by helper functions when the Config argument is nil.
	ErrNilConfig = errors.New("configx: config is nil")
)

// Observer is invoked when a watched config key changes.
type Observer func(string, Value)

// Config is the runtime configuration interface used by kernel modules and apps.
type Config interface {
	Load() error
	Scan(v any) error
	Value(key string) Value
	Watch(key string, o Observer) error
	Close() error
}

type config struct {
	opts   options
	reader Reader

	cached sync.Map // map[string]Value

	loadMu       sync.Mutex
	watchStarted bool
	watchers     []Watcher
	closed       bool

	observerMu sync.RWMutex
	observers  map[string][]Observer
}

// New creates a Config from one or more sources.
func New(opts ...Option) Config {
	o := options{
		decoder:  defaultDecoder,
		resolver: defaultResolver,
		merge:    defaultMerge,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return &config{
		opts:      o,
		reader:    newReader(o),
		observers: make(map[string][]Observer),
	}
}

func (c *config) watch(w Watcher) {
	for {
		kvs, err := w.Next()
		if err != nil {
			if c.isClosed() || errors.Is(err, context.Canceled) {
				return
			}
			logx.Error("failed to watch next config", "error", err)
			time.Sleep(time.Second)
			continue
		}
		if len(kvs) == 0 {
			if c.isClosed() {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if err := c.mergeAndResolve(kvs...); err != nil {
			logx.Error("failed to apply watched config", "error", err)
			continue
		}
		c.refreshCached(true)
	}
}

func (c *config) mergeAndResolve(kvs ...*KeyValue) error {
	if err := c.reader.Merge(kvs...); err != nil {
		return err
	}
	return c.reader.Resolve()
}

func (c *config) Load() error {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	if c.closed {
		return ErrClosed
	}

	for _, src := range c.opts.sources {
		if src == nil {
			continue
		}
		kvs, err := src.Load()
		if err != nil {
			return err
		}
		for _, v := range kvs {
			if v != nil {
				logx.Debug("config loaded", "key", v.Key, "format", v.Format)
			}
		}
		if err = c.reader.Merge(kvs...); err != nil {
			logx.Error("failed to merge config source", "error", err)
			return err
		}

		if !c.watchStarted {
			w, err := src.Watch()
			if err != nil {
				logx.Error("failed to watch config source", "error", err)
				return err
			}
			if w != nil {
				c.watchers = append(c.watchers, w)
				go c.watch(w)
			}
		}
	}
	if err := c.reader.Resolve(); err != nil {
		logx.Error("failed to resolve config source", "error", err)
		return err
	}
	c.watchStarted = true
	c.refreshCached(true)
	return nil
}

func (c *config) Value(key string) Value {
	if v, ok := c.cached.Load(key); ok {
		return v.(Value)
	}
	if v, ok := c.reader.Value(key); ok {
		c.cached.Store(key, v)
		return v
	}
	return &errValue{err: ErrNotFound}
}

func (c *config) Scan(v any) error {
	data, err := c.reader.Source()
	if err != nil {
		return err
	}
	return unmarshalJSON(data, v)
}

func (c *config) Watch(key string, o Observer) error {
	if o == nil {
		return ErrInvalidObserver
	}
	if c.isClosed() {
		return ErrClosed
	}
	if v := c.Value(key); v.Load() == nil {
		return ErrNotFound
	}
	c.observerMu.Lock()
	if c.observers == nil {
		c.observers = make(map[string][]Observer)
	}
	c.observers[key] = append(c.observers[key], o)
	c.observerMu.Unlock()
	return nil
}

func (c *config) Close() error {
	c.loadMu.Lock()
	if c.closed {
		c.loadMu.Unlock()
		return nil
	}
	c.closed = true
	watchers := append([]Watcher(nil), c.watchers...)
	c.watchers = nil
	c.loadMu.Unlock()

	var firstErr error
	for _, w := range watchers {
		if w == nil {
			continue
		}
		if err := w.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *config) isClosed() bool {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	return c.closed
}

func (c *config) refreshCached(notify bool) {
	c.cached.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(Value)
		old := v.Load()
		if n, ok := c.reader.Value(k); ok {
			newValue := n.Load()
			if !reflect.DeepEqual(old, newValue) {
				v.Store(newValue)
				if notify {
					c.notify(k, v)
				}
			}
			return true
		}
		if old != nil {
			v.Store(nil)
			if notify {
				c.notify(k, v)
			}
		}
		return true
	})
}

func (c *config) notify(key string, v Value) {
	c.observerMu.RLock()
	observers := append([]Observer(nil), c.observers[key]...)
	c.observerMu.RUnlock()
	for _, observer := range observers {
		observer(key, v)
	}
}

// Get retrieves a config value by key and scans it into the target type.
func Get[T any](c Config, key string) (T, error) {
	var t T
	if c == nil {
		return t, ErrNilConfig
	}
	v := c.Value(key)

	if v.Load() == nil {
		return t, ErrNotFound
	}

	switch any(t).(type) {
	case bool:
		b, err := v.Bool()
		return any(b).(T), err
	case int64:
		i, err := v.Int()
		return any(i).(T), err
	case int:
		i, err := v.Int()
		return any(int(i)).(T), err
	case float64:
		f, err := v.Float()
		return any(f).(T), err
	case string:
		s, err := v.String()
		return any(s).(T), err
	}

	err := v.Scan(&t)
	return t, err
}

// MustGet returns a typed value or panics. Use it only during process startup.
func MustGet[T any](c Config, key string) T {
	v, err := Get[T](c, key)
	if err != nil {
		panic(err)
	}
	return v
}

// GetOrDefault returns a typed value if present; otherwise it returns fallback.
func GetOrDefault[T any](c Config, key string, fallback T) T {
	v, err := Get[T](c, key)
	if err != nil {
		return fallback
	}
	return v
}
