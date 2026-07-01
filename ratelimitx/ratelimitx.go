// Package ratelimitx defines Kernel's rate limit provider contract.
//
// It separates policy semantics from middleware wiring. MEMORY is local to one
// process/pod and is safe for demos or per-instance self-protection. Cluster-wide
// quotas must use REDIS or EXTERNAL providers and are validated by bootx/serverx
// or clientpolicyx before the service starts.
package ratelimitx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aisphereio/kernel/errorx"
)

// Backend selects where limiter state lives.
type Backend string

const (
	BackendMemory   Backend = "memory"
	BackendRedis    Backend = "redis"
	BackendExternal Backend = "external"
)

// Scope defines whether a limit is local to this instance or shared cluster-wide.
type Scope string

const (
	ScopeLocalInstance Scope = "local_instance"
	ScopeGlobalCluster Scope = "global_cluster"
)

// FailMode controls behavior when a distributed provider is unavailable.
type FailMode string

const (
	FailOpen   FailMode = "open"
	FailClosed FailMode = "closed"
)

// KeyStrategy names how middleware should derive a limiter key.
type KeyStrategy string

const (
	KeyOperation KeyStrategy = "operation"
	KeyPrincipal KeyStrategy = "principal"
	KeyTenant    KeyStrategy = "tenant"
	KeyCaller    KeyStrategy = "caller"
	KeyCustom    KeyStrategy = "custom"
)

// Policy is the normalized rate-limit contract used by server and client autowire.
type Policy struct {
	Name     string
	Enabled  bool
	Backend  Backend
	Scope    Scope
	Key      KeyStrategy
	QPS      float64
	Burst    int
	FailMode FailMode
}

// Normalize fills production-safe defaults without weakening the declared scope.
func (p Policy) Normalize() Policy {
	if p.Backend == "" {
		p.Backend = BackendMemory
	}
	if p.Scope == "" {
		p.Scope = ScopeLocalInstance
	}
	if p.Key == "" {
		p.Key = KeyOperation
	}
	if p.FailMode == "" {
		p.FailMode = FailClosed
	}
	if p.Burst <= 0 && p.QPS > 0 {
		p.Burst = int(p.QPS)
		if p.Burst < 1 {
			p.Burst = 1
		}
	}
	return p
}

// Validate checks the policy independently from deployment topology.
func (p Policy) Validate() error {
	p = p.Normalize()
	if !p.Enabled {
		return nil
	}
	if p.QPS <= 0 {
		return fmt.Errorf("rate limit %q qps must be > 0", p.Name)
	}
	if p.Burst <= 0 {
		return fmt.Errorf("rate limit %q burst must be > 0", p.Name)
	}
	switch p.Backend {
	case BackendMemory, BackendRedis, BackendExternal:
	default:
		return fmt.Errorf("rate limit %q has unsupported backend %q", p.Name, p.Backend)
	}
	switch p.Scope {
	case ScopeLocalInstance, ScopeGlobalCluster:
	default:
		return fmt.Errorf("rate limit %q has unsupported scope %q", p.Name, p.Scope)
	}
	if p.Scope == ScopeGlobalCluster && p.Backend == BackendMemory {
		return fmt.Errorf("rate limit %q scope=global_cluster cannot use backend=memory; use redis or external", p.Name)
	}
	return nil
}

// ValidateForDeployment checks topology-sensitive constraints.
func (p Policy) ValidateForDeployment(replicas int) error {
	if err := p.Validate(); err != nil {
		return err
	}
	p = p.Normalize()
	if !p.Enabled {
		return nil
	}
	if replicas > 1 && p.Scope == ScopeGlobalCluster && p.Backend == BackendMemory {
		return fmt.Errorf("rate limit %q replicas=%d requires backend=redis or external for global_cluster scope", p.Name, replicas)
	}
	return nil
}

// Decision is returned by a limiter provider.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Reason     string
}

// Limiter is the runtime interface used by middleware.
type Limiter interface {
	Allow(ctx context.Context, key string, cost int) (Decision, error)
}

// Provider creates limiters from policies.
type Provider interface {
	NewLimiter(policy Policy) (Limiter, error)
}

// ErrLimited is the canonical cross-protocol error returned by rate-limit middleware.
var ErrLimited = errorx.TooManyRequests("RATE_LIMIT_EXCEEDED", "request rejected by rate limit")

// MemoryProvider implements per-process token buckets. It is intentionally local
// and must not be used for cluster-wide quotas.
type MemoryProvider struct{}

func NewMemoryProvider() *MemoryProvider { return &MemoryProvider{} }

func (p *MemoryProvider) NewLimiter(policy Policy) (Limiter, error) {
	policy = policy.Normalize()
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.Backend != BackendMemory {
		return nil, fmt.Errorf("memory provider cannot serve backend %q", policy.Backend)
	}
	return NewMemoryLimiter(policy), nil
}

// NewMemoryLimiter creates a local token bucket limiter.
func NewMemoryLimiter(policy Policy) Limiter {
	policy = policy.Normalize()
	return &memoryLimiter{policy: policy, buckets: map[string]*bucket{}}
}

type memoryLimiter struct {
	mu      sync.Mutex
	policy  Policy
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func (l *memoryLimiter) Allow(_ context.Context, key string, cost int) (Decision, error) {
	if cost <= 0 {
		cost = 1
	}
	if strings.TrimSpace(key) == "" {
		key = "_default"
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: float64(l.policy.Burst), last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.policy.QPS
		if max := float64(l.policy.Burst); b.tokens > max {
			b.tokens = max
		}
		b.last = now
	}
	need := float64(cost)
	if b.tokens >= need {
		b.tokens -= need
		return Decision{Allowed: true}, nil
	}
	missing := need - b.tokens
	retry := time.Duration((missing / l.policy.QPS) * float64(time.Second))
	if retry < time.Millisecond {
		retry = time.Millisecond
	}
	return Decision{Allowed: false, RetryAfter: retry, Reason: "token_bucket_exhausted"}, nil
}
