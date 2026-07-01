package bootx

import (
	"strings"
	"testing"
	"time"

	"github.com/aisphereio/kernel/clientpolicyx"
	"github.com/aisphereio/kernel/ratelimitx"
)

func TestValidateGovernanceRejectsGlobalMemoryLimit(t *testing.T) {
	err := ValidateGovernance(GovernanceConfig{
		Deployment:   Deployment{Replicas: 3},
		ServerLimits: []ratelimitx.Policy{{Name: "api", Enabled: true, Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeGlobalCluster, QPS: 10, Burst: 20}},
	})
	if err == nil || !strings.Contains(err.Error(), "global_cluster") {
		t.Fatalf("expected global memory rate limit rejection, got %v", err)
	}
}

func TestValidateGovernanceChecksDownstreamAndProviders(t *testing.T) {
	err := ValidateGovernance(GovernanceConfig{
		Deployment: Deployment{Replicas: 1},
		Downstreams: []clientpolicyx.DownstreamPolicy{{
			Name: "iam", Protocol: clientpolicyx.ProtocolGRPC, Target: "discovery:///iam", Timeout: time.Second,
			RateLimit:   ratelimitx.Policy{Name: "iam", Enabled: true, Backend: ratelimitx.BackendMemory, Scope: ratelimitx.ScopeLocalInstance, QPS: 100, Burst: 200},
			ServiceAuth: clientpolicyx.ServiceAuthPolicy{Enabled: true, Mode: "jwt"},
		}},
		AuthnRequired: true, AuthnConfigured: true,
		AuthzRequired: true, AuthzConfigured: true,
		AuditRequired: true, AuditConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ValidateGovernance(GovernanceConfig{AuthzRequired: true})
	if err == nil || !strings.Contains(err.Error(), "authz provider") {
		t.Fatalf("expected missing authz provider error, got %v", err)
	}
}
