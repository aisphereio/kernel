package gatewayx

import (
	"sort"
	"strings"
	"sync"
	"time"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
)

// AuthnMode describes how the gateway treats authentication at the edge.
// Resource-level authorization remains the downstream service's responsibility.
type AuthnMode string

const (
	AuthnModeNone       AuthnMode = "none"
	AuthnModePassive    AuthnMode = "passive"
	AuthnModeVerifyJWT  AuthnMode = "verify_jwt"
	AuthnModeIntrospect AuthnMode = "introspect"
)

// GatewayPolicy is the boundary-governance slice of a route policy. It must not
// contain business resource authorization details; those stay in the service
// access resolver and authz provider.
type GatewayPolicy struct {
	Exposure             accessv1.Exposure
	AuthnMode            AuthnMode
	ForwardAuthorization bool
	Timeout              time.Duration

	// Profiles is an optional publication profile list such as public, internal,
	// or ops. Empty means deployment-time filters derive behavior from Exposure.
	Profiles []string

	// Tags are optional governance labels such as skill, admin, debug, dtm.
	Tags []string
}

// EffectiveAuthnMode returns the explicit mode or derives a safe default from
// exposure. MVP gateway deployments should use passive token relay by default.
func (p GatewayPolicy) EffectiveAuthnMode() AuthnMode {
	if p.AuthnMode != "" {
		return p.AuthnMode
	}
	switch p.Exposure {
	case accessv1.Exposure_PUBLIC, accessv1.Exposure_SYSTEM:
		return AuthnModeNone
	case accessv1.Exposure_AUTHENTICATED, accessv1.Exposure_AUTHORIZED:
		return AuthnModePassive
	case accessv1.Exposure_INTERNAL:
		return AuthnModeNone
	default:
		return AuthnModePassive
	}
}

// UpstreamRef points to a Kubernetes Service in production. Validation tests can
// resolve it through StaticHosts, mirroring /etc/hosts or local docker-compose.
type UpstreamRef struct {
	Service   string
	Namespace string
	Port      int
	Protocol  string
	Operation string
}

func (u UpstreamRef) Key() string {
	ns := strings.TrimSpace(u.Namespace)
	if ns == "" {
		ns = "default"
	}
	return strings.TrimSpace(u.Service) + "." + ns
}

// GatewayRoute is generated from google.api.http + aisphere.access.v1.policy.
type GatewayRoute struct {
	ID       string
	Method   string
	Path     string
	Upstream UpstreamRef
	Gateway  GatewayPolicy
}

// Manifest is the route artifact a service publishes into the route registry.
type Manifest struct {
	Service   string
	Namespace string
	Routes    []GatewayRoute
}

// RouteRegistry is the provider-neutral gateway route registry contract.
// Production implementation should be etcd-backed; tests and local demos use
// MemoryRegistry.
type RouteRegistry interface {
	RegisterManifest(manifest Manifest) error
	ListRoutes() []GatewayRoute
}

// MemoryRegistry is a local/demo route registry. It mirrors the etcd registry
// shape without introducing an external dependency into unit tests.
type MemoryRegistry struct {
	mu     sync.RWMutex
	routes map[string]GatewayRoute
}

func NewMemoryRegistry() *MemoryRegistry { return &MemoryRegistry{routes: map[string]GatewayRoute{}} }

func (r *MemoryRegistry) RegisterManifest(manifest Manifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.routes == nil {
		r.routes = map[string]GatewayRoute{}
	}
	manifest = normalizeManifest(manifest)
	for _, rt := range manifest.Routes {
		r.routes[rt.ID] = rt
	}
	return nil
}

func (r *MemoryRegistry) ListRoutes() []GatewayRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]GatewayRoute, 0, len(r.routes))
	for _, rt := range r.routes {
		out = append(out, rt)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
