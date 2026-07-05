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
	// Authn is intentionally not copied into Guard on the main framework path.
	// serverx/autowire installs middleware/authn before middleware/access, so the
	// access guard consumes the already-injected Principal and performs only
	// authz/audit. This avoids a second, implicit authn flow inside accessx.
	return NewGuard(p.Authz.Authorizer(), p.Audit)
}
