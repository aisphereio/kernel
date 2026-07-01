package ratelimitx

import (
	"context"
	"testing"
)

func TestPolicyRejectsGlobalMemory(t *testing.T) {
	p := Policy{Enabled: true, Name: "tenant", Scope: ScopeGlobalCluster, Backend: BackendMemory, QPS: 10, Burst: 10}
	if err := p.Validate(); err == nil {
		t.Fatal("expected global memory validation error")
	}
}

func TestMemoryLimiter(t *testing.T) {
	l := NewMemoryLimiter(Policy{Enabled: true, Name: "local", Backend: BackendMemory, Scope: ScopeLocalInstance, QPS: 1, Burst: 1})
	if d, err := l.Allow(context.Background(), "op", 1); err != nil || !d.Allowed {
		t.Fatalf("first allow = %+v err=%v", d, err)
	}
	if d, err := l.Allow(context.Background(), "op", 1); err != nil || d.Allowed {
		t.Fatalf("second allow = %+v err=%v", d, err)
	}
}
