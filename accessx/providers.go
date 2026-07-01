package accessx

import (
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

// Providers is the framework-level authn/authz/audit bundle consumed by
// serverx/autowire. It keeps concrete implementations such as Casdoor and
// SpiceDB outside business packages.
type Providers struct {
	Authn authn.ProviderSet
	Authz authz.ProviderSet
	Audit auditx.Recorder
}

// Guard builds the request-time access guard from provider-neutral Kernel
// interfaces. Missing authz defaults to deny-all at the Guard layer.
func (p Providers) Guard() Guard {
	return New(p.Authn.Authenticator(), p.Authz.Authorizer(), p.Audit)
}
