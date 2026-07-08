package contextx

import (
	"context"

	"github.com/aisphereio/kernel/authn"
)

// WithAuthnPrincipal mirrors an authn.Principal into contextx.Principal. This
// keeps authn-aware middleware and contextx-aware business/framework code aligned
// without dropping identity fields.
func WithAuthnPrincipal(ctx context.Context, p authn.Principal) context.Context {
	return WithPrincipal(ctx, FromAuthnPrincipal(p))
}

// FromAuthnPrincipal converts authn.Principal to the contextx mirror type.
func FromAuthnPrincipal(p authn.Principal) *Principal {
	p = p.Normalize()
	return &Principal{
		SubjectID:   p.SubjectID,
		SubjectType: p.SubjectType,
		Provider:    p.Provider,
		ExternalID:  p.ExternalID,
		Issuer:      p.Issuer,
		Audience:    cloneStrings(p.Audience),
		TenantID:    p.TenantID,
		OrgID:       p.OrgID,
		AppID:       p.AppID,
		ProjectID:   p.ProjectID,
		Username:    p.Username,
		Name:        p.Name,
		Email:       p.Email,
		Phone:       p.Phone,
		Roles:       cloneStrings(p.Roles),
		Groups:      cloneStrings(p.Groups),
		Scopes:      cloneStrings(p.Scopes),
		AuthMethod:  p.AuthMethod,
		Attributes:  cloneAuthnAttributes(p.Attributes),
		IssuedAt:    p.IssuedAt,
		ExpiresAt:   p.ExpiresAt,
	}
}

// ToAuthnPrincipal converts a contextx principal mirror back to authn.Principal.
// This is mainly useful for framework adapters that accept a generic contextx
// request context but need to call authn/authz APIs.
func ToAuthnPrincipal(p *Principal) authn.Principal {
	if p == nil {
		return authn.Principal{}
	}
	n := p.Normalize()
	return authn.Principal{
		SubjectID:   n.SubjectID,
		SubjectType: n.SubjectType,
		Provider:    n.Provider,
		ExternalID:  n.ExternalID,
		Issuer:      n.Issuer,
		Audience:    cloneStrings(n.Audience),
		TenantID:    n.TenantID,
		OrgID:       n.OrgID,
		AppID:       n.AppID,
		ProjectID:   n.ProjectID,
		Username:    n.Username,
		Name:        n.Name,
		Email:       n.Email,
		Phone:       n.Phone,
		Roles:       cloneStrings(n.Roles),
		Groups:      cloneStrings(n.Groups),
		Scopes:      cloneStrings(n.Scopes),
		AuthMethod:  n.AuthMethod,
		Attributes:  cloneContextAttributes(n.Attributes),
		IssuedAt:    n.IssuedAt,
		ExpiresAt:   n.ExpiresAt,
	}.Normalize()
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string(nil), in...)
}

func cloneAuthnAttributes(in authn.AttributeSet) AttributeSet {
	if in == nil {
		return nil
	}
	out := make(AttributeSet, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneContextAttributes(in AttributeSet) authn.AttributeSet {
	if in == nil {
		return nil
	}
	out := make(authn.AttributeSet, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
