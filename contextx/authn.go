package contextx

import (
	"context"

	"github.com/aisphereio/kernel/authn"
)

// WithAuthnPrincipal mirrors an authn.Principal into contextx.Principal. This
// keeps old authn-aware middleware and new contextx-aware business code aligned
// during the Gateway/BFF migration.
func WithAuthnPrincipal(ctx context.Context, p authn.Principal) context.Context {
	p = p.Normalize()
	return WithPrincipal(ctx, &Principal{
		SubjectID:  p.SubjectID,
		TenantID:   p.TenantID,
		Roles:      append([]string(nil), p.Roles...),
		Scopes:     append([]string(nil), p.Scopes...),
		AuthMethod: p.AuthMethod,
	})
}
