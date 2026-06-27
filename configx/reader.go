package configx

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/aisphereio/kernel/logx"
)

// Reader is the internal merged config tree reader.
type Reader interface {
	Merge(...*KeyValue) error
	Value(string) (Value, bool)
	Source() ([]byte, error)
	Resolve() error
}

type reader struct {
	opts   options
	values map[string]any
	lock   sync.RWMutex
}

func newReader(opts options) Reader {
	return &reader{
		opts:   opts,
		values: make(map[string]any),
	}
}

func (r *reader) Merge(kvs ...*KeyValue) error {
	merged, err := r.cloneMap()
	if err != nil {
		return err
	}
	for _, kv := range kvs {
		if kv == nil {
			continue
		}
		next := make(map[string]any)
		if err := r.opts.decoder(kv, next); err != nil {
			logx.Error("failed to decode config", "error", err, "key", kv.Key)
			return err
		}
		if err := r.opts.merge(&merged, convertMap(next)); err != nil {
			logx.Error("failed to merge config", "error", err, "key", kv.Key)
			return err
		}
	}
	r.lock.Lock()
	r.values = merged
	r.lock.Unlock()
	return nil
}

func (r *reader) Value(path string) (Value, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return readValue(r.values, path)
}

func (r *reader) Source() ([]byte, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return marshalJSON(convertMap(r.values))
}

func (r *reader) Resolve() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.opts.resolver(r.values)
}

func (r *reader) cloneMap() (map[string]any, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return cloneMap(r.values)
}

func cloneMap(src map[string]any) (map[string]any, error) {
	if src == nil {
		return map[string]any{}, nil
	}
	cloned, ok := cloneMergeValue(convertMap(src)).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configx: clone source must be map[string]any, got %T", src)
	}
	return cloned, nil
}

func convertMap(src any) any {
	switch m := src.(type) {
	case map[string]any:
		dst := make(map[string]any, len(m))
		for k, v := range m {
			dst[k] = convertMap(v)
		}
		return dst
	case map[any]any:
		dst := make(map[string]any, len(m))
		for k, v := range m {
			dst[fmt.Sprint(k)] = convertMap(v)
		}
		return dst
	case []any:
		dst := make([]any, len(m))
		for k, v := range m {
			dst[k] = convertMap(v)
		}
		return dst
	case []byte:
		// There is no binary data in config values; env/file raw values become strings.
		return string(m)
	default:
		return src
	}
}

// readValue reads Value in the given map by dot path, returning false if missing.
func readValue(values map[string]any, path string) (Value, bool) {
	if path == "" {
		av := &atomicValue{}
		av.Store(values)
		return av, true
	}
	var (
		next = values
		keys = strings.Split(path, ".")
		last = len(keys) - 1
	)
	for idx, key := range keys {
		value, ok := next[key]
		if !ok {
			return nil, false
		}
		if idx == last {
			av := &atomicValue{}
			av.Store(value)
			return av, true
		}
		switch vm := value.(type) {
		case map[string]any:
			next = vm
		default:
			return nil, false
		}
	}
	return nil, false
}

func marshalJSON(v any) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(m)
	}
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v any) error {
	if m, ok := v.(proto.Message); ok {
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, m)
	}
	return json.Unmarshal(data, v)
}
