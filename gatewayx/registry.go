package gatewayx

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

// KVStore is the small etcd-like contract Gateway needs. Production should
// adapt this to clientv3 under gatewayx/etcd or an etcdx.Store; tests can use
// MemoryKVStore. Kernel business packages must not use raw etcd clients.
type KVStore interface {
	Put(ctx context.Context, key string, value []byte) error
	DeletePrefix(ctx context.Context, prefix string) error
	ListPrefix(ctx context.Context, prefix string) (map[string][]byte, error)
}

// EtcdRegistry stores manifests in an etcd-shaped KV store. The type does not
// import clientv3 directly so offline builds and unit tests stay lightweight.
type EtcdRegistry struct {
	Store  KVStore
	Prefix string
}

func NewEtcdRegistry(store KVStore, prefix string) *EtcdRegistry {
	if prefix == "" {
		prefix = "/aisphere/kernel/routes/dev"
	}
	return &EtcdRegistry{Store: store, Prefix: strings.TrimRight(prefix, "/")}
}

func (r *EtcdRegistry) RegisterManifest(manifest Manifest) error {
	if r == nil || r.Store == nil {
		return fmt.Errorf("gatewayx: etcd registry store is not configured")
	}
	manifest = normalizeManifest(manifest)
	ctx := context.Background()
	servicePrefix := path.Join(r.Prefix, manifest.Namespace, manifest.Service)
	if err := r.Store.DeletePrefix(ctx, servicePrefix+"/"); err != nil {
		return err
	}
	for _, rt := range manifest.Routes {
		b, err := json.Marshal(rt)
		if err != nil {
			return err
		}
		if err := r.Store.Put(ctx, path.Join(servicePrefix, rt.ID), b); err != nil {
			return err
		}
	}
	return nil
}

func (r *EtcdRegistry) ListRoutes() []GatewayRoute {
	if r == nil || r.Store == nil {
		return nil
	}
	m, err := r.Store.ListPrefix(context.Background(), strings.TrimRight(r.Prefix, "/")+"/")
	if err != nil {
		return nil
	}
	out := make([]GatewayRoute, 0, len(m))
	for _, b := range m {
		var rt GatewayRoute
		if json.Unmarshal(b, &rt) == nil && rt.ID != "" {
			out = append(out, rt)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normalizeManifest(manifest Manifest) Manifest {
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	for i := range manifest.Routes {
		rt := &manifest.Routes[i]
		if rt.ID == "" {
			rt.ID = manifest.Service + ":" + strings.ToUpper(rt.Method) + ":" + rt.Path
		}
		if rt.Upstream.Service == "" {
			rt.Upstream.Service = manifest.Service
		}
		if rt.Upstream.Namespace == "" {
			rt.Upstream.Namespace = manifest.Namespace
		}
	}
	return manifest
}

// MemoryKVStore is an in-memory KVStore for tests. It mirrors etcd prefix
// semantics closely enough for Gateway route registry validation.
type MemoryKVStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMemoryKVStore() *MemoryKVStore { return &MemoryKVStore{data: map[string][]byte{}} }

func (s *MemoryKVStore) Put(ctx context.Context, key string, value []byte) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[key] = append([]byte(nil), value...)
	return nil
}

func (s *MemoryKVStore) DeletePrefix(ctx context.Context, prefix string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			delete(s.data, k)
		}
	}
	return nil
}

func (s *MemoryKVStore) ListPrefix(ctx context.Context, prefix string) (map[string][]byte, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string][]byte{}
	for k, v := range s.data {
		if strings.HasPrefix(k, prefix) {
			out[k] = append([]byte(nil), v...)
		}
	}
	return out, nil
}
