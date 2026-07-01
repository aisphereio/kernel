package authz

// Provider is Kernel's runtime authorization surface.
//
// Business hot paths usually require only Check. Admin/provisioning code can
// use Service to manage schema and relationships. Concrete engines such as
// SpiceDB, Casbin or Casdoor-backed policy adapters must implement these
// Kernel contracts instead of being imported by business packages.
type Provider interface {
	Authorizer
}

// AdminProvider is the full authorization control-plane surface.
// Kernel's production ReBAC implementation is authz/spicedb.Client.
type AdminProvider interface {
	Service
}

// ProviderSet groups runtime and admin authorization providers for boot wiring.
type ProviderSet struct {
	Runtime Provider
	Admin   AdminProvider
}

func (p ProviderSet) Authorizer() Authorizer {
	if p.Runtime != nil {
		return p.Runtime
	}
	if p.Admin != nil {
		return p.Admin
	}
	return nil
}

func (p ProviderSet) Service() Service {
	if p.Admin != nil {
		return p.Admin
	}
	return nil
}
