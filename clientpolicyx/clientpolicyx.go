// Package clientpolicyx defines outbound microservice call governance.
//
// Server access policy answers: "who may call this RPC?" Downstream policy
// answers: "how does this service call that downstream safely?" Business code
// must use Kernel client factories/autowire so timeout, identity propagation,
// rate limiting, retry, circuit breaker and errorx decoding stay consistent.
package clientpolicyx

import (
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/ratelimitx"
)

// Protocol is the downstream transport.
type Protocol string

const (
	ProtocolGRPC Protocol = "grpc"
	ProtocolHTTP Protocol = "http"
)

// Deployment describes topology used for validation.
type Deployment struct {
	Replicas int
}

// RetryPolicy controls client retries. Only retryable errorx errors or explicit
// transient transport failures should be retried by runtime middleware.
type RetryPolicy struct {
	Enabled     bool
	MaxAttempts int
	Backoff     time.Duration
}

func (p RetryPolicy) Normalize() RetryPolicy {
	if p.Enabled && p.MaxAttempts <= 0 {
		p.MaxAttempts = 2
	}
	if p.Enabled && p.Backoff <= 0 {
		p.Backoff = 25 * time.Millisecond
	}
	return p
}

// CircuitBreakerPolicy controls outbound breaker behavior.
type CircuitBreakerPolicy struct {
	Enabled bool
	Name    string
}

// ServiceAuthPolicy controls service-to-service identity.
type ServiceAuthPolicy struct {
	Enabled bool
	Mode    string // jwt, mtls, spiffe, mesh
}

// DownstreamPolicy is the caller-side contract for one downstream dependency.
type DownstreamPolicy struct {
	Name           string
	Protocol       Protocol
	Target         string
	Timeout        time.Duration
	RateLimit      ratelimitx.Policy
	CircuitBreaker CircuitBreakerPolicy
	Retry          RetryPolicy
	ServiceAuth    ServiceAuthPolicy
}

// Normalize fills safe defaults.
func (p DownstreamPolicy) Normalize() DownstreamPolicy {
	if p.Protocol == "" {
		p.Protocol = ProtocolGRPC
	}
	if p.Timeout <= 0 {
		p.Timeout = time.Second
	}
	p.Retry = p.Retry.Normalize()
	if p.RateLimit.Enabled && p.RateLimit.Name == "" {
		p.RateLimit.Name = p.Name + ".client"
	}
	return p
}

// Validate checks policy semantics and topology-sensitive constraints.
func (p DownstreamPolicy) Validate(deploy Deployment) error {
	p = p.Normalize()
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("downstream policy name is required")
	}
	switch p.Protocol {
	case ProtocolGRPC, ProtocolHTTP:
	default:
		return fmt.Errorf("downstream %q has unsupported protocol %q", p.Name, p.Protocol)
	}
	if strings.TrimSpace(p.Target) == "" {
		return fmt.Errorf("downstream %q target is required", p.Name)
	}
	if p.Timeout <= 0 {
		return fmt.Errorf("downstream %q timeout must be > 0", p.Name)
	}
	if p.Retry.Enabled && p.Retry.MaxAttempts < 2 {
		return fmt.Errorf("downstream %q retry max_attempts must be >= 2", p.Name)
	}
	if p.Retry.Enabled && p.Timeout <= 0 {
		return fmt.Errorf("downstream %q retry requires timeout", p.Name)
	}
	if p.RateLimit.Enabled {
		if err := p.RateLimit.ValidateForDeployment(deploy.Replicas); err != nil {
			return fmt.Errorf("downstream %q: %w", p.Name, err)
		}
	}
	if p.ServiceAuth.Enabled && strings.TrimSpace(p.ServiceAuth.Mode) == "" {
		return fmt.Errorf("downstream %q service_auth mode is required", p.Name)
	}
	return nil
}
