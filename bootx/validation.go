// Package bootx owns framework-level startup validation.
//
// It is intentionally small for now: Kernel services can use it before server
// startup to fail fast on topology/policy mismatches instead of letting each
// business component rediscover the same production rules.
package bootx

import (
	"fmt"

	"github.com/aisphereio/kernel/clientpolicyx"
	"github.com/aisphereio/kernel/ratelimitx"
)

// Deployment describes the runtime topology known at startup.
type Deployment struct {
	Replicas int
}

// GovernanceConfig contains the policy slices bootx can validate before serving traffic.
type GovernanceConfig struct {
	Deployment      Deployment
	ServerLimits    []ratelimitx.Policy
	Downstreams     []clientpolicyx.DownstreamPolicy
	AuthnRequired   bool
	AuthnConfigured bool
	AuthzRequired   bool
	AuthzConfigured bool
	AuditRequired   bool
	AuditConfigured bool
}

// ValidateGovernance fails fast when Kernel governance contracts are inconsistent
// with runtime topology or missing required providers.
func ValidateGovernance(cfg GovernanceConfig) error {
	replicas := cfg.Deployment.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	for _, p := range cfg.ServerLimits {
		if err := p.ValidateForDeployment(replicas); err != nil {
			return fmt.Errorf("server rate limit: %w", err)
		}
	}
	for _, ds := range cfg.Downstreams {
		if err := ds.Validate(clientpolicyx.Deployment{Replicas: replicas}); err != nil {
			return err
		}
	}
	if cfg.AuthnRequired && !cfg.AuthnConfigured {
		return fmt.Errorf("authn provider is required but not configured")
	}
	if cfg.AuthzRequired && !cfg.AuthzConfigured {
		return fmt.Errorf("authz provider is required but not configured")
	}
	if cfg.AuditRequired && !cfg.AuditConfigured {
		return fmt.Errorf("audit provider is required but not configured")
	}
	return nil
}
