package clientpolicyx

import (
	"testing"
	"time"

	"github.com/aisphereio/kernel/ratelimitx"
)

func TestValidateRejectsMissingTarget(t *testing.T) {
	p := DownstreamPolicy{Name: "iam", Timeout: time.Second}
	if err := p.Validate(Deployment{Replicas: 1}); err == nil {
		t.Fatal("expected missing target error")
	}
}

func TestValidateRejectsGlobalMemory(t *testing.T) {
	p := DownstreamPolicy{
		Name: "iam", Target: "discovery:///iam", Timeout: time.Second,
		RateLimit: ratelimitx.Policy{Enabled: true, Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeGlobalCluster, QPS: 10, Burst: 10},
	}
	if err := p.Validate(Deployment{Replicas: 3}); err == nil {
		t.Fatal("expected global memory validation error")
	}
}

func TestValidateAcceptsLocalMemory(t *testing.T) {
	p := DownstreamPolicy{
		Name: "iam", Target: "discovery:///iam", Timeout: time.Second,
		RateLimit:   ratelimitx.Policy{Enabled: true, Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 10, Burst: 10},
		Retry:       RetryPolicy{Enabled: true, MaxAttempts: 2},
		ServiceAuth: ServiceAuthPolicy{Enabled: true, Mode: "jwt"},
	}
	if err := p.Validate(Deployment{Replicas: 3}); err != nil {
		t.Fatal(err)
	}
}
