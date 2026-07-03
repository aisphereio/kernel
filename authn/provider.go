package authn

// Provider is Kernel's default runtime authentication surface.
//
// Business code must depend on this interface (or one of its smaller
// sub-interfaces), not on concrete identity providers such as Casdoor.
// Casdoor/OIDC/JWT implementations live behind this contract.
type Provider interface {
	Authenticator
	TokenService
	LoginService
	LogoutService
}

// ManagementProvider extends Provider with user/organization/application/group
// administration. It is intended for IAM control-plane code and provisioning,
// not normal business request handlers.
//
// Kernel's default implementation is authn/casdoor.Client. Business packages
// should not import the Casdoor SDK directly.
type ManagementProvider interface {
	Provider
	IdentityAdmin
}

// ProviderSet names the Kernel authn capabilities that boot/server wiring can
// expose without leaking a concrete provider implementation.
type ProviderSet struct {
	Runtime    Provider
	Management ManagementProvider
}

func (p ProviderSet) Authenticator() Authenticator {
	if p.Runtime != nil {
		return p.Runtime
	}
	if p.Management != nil {
		return p.Management
	}
	return nil
}

func (p ProviderSet) Tokens() TokenService {
	if p.Runtime != nil {
		return p.Runtime
	}
	if p.Management != nil {
		return p.Management
	}
	return nil
}

func (p ProviderSet) Login() LoginService {
	if p.Runtime != nil {
		return p.Runtime
	}
	if p.Management != nil {
		return p.Management
	}
	return nil
}

func (p ProviderSet) Logout() LogoutService {
	if p.Runtime != nil {
		return p.Runtime
	}
	if p.Management != nil {
		return p.Management
	}
	return nil
}

func (p ProviderSet) IdentityAdmin() IdentityAdmin {
	if p.Management != nil {
		return p.Management
	}
	return nil
}
