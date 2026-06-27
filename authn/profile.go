package authn

import "context"

// ProfileService enriches an authenticated Principal with identity-provider
// profile data. It is intentionally provider-neutral: callers should not have
// to know whether the backing system is Casdoor, OIDC, LDAP, SCIM, etc.
type ProfileService interface {
	GetIdentityProfile(ctx context.Context, req IdentityProfileRequest) (IdentityProfile, error)
}

// IdentityProfileRequest controls how much identity-provider data should be
// loaded after login or token verification.
//
// For request handling, pass Principal when you already have one from
// Authenticate/VerifyToken. For test tools and middleware, pass Token or
// Credential and the implementation will authenticate first.
type IdentityProfileRequest struct {
	Principal  Principal
	Token      string
	Credential Credential

	OrgID string
	AppID string

	IncludeUser               bool
	IncludeGroups             bool
	IncludeCurrentApplication bool

	// AllowPartial returns Principal plus whatever profile data can be read when
	// admin/read APIs fail. This is useful for normal request paths where token
	// authentication must not fail only because optional profile enrichment is
	// unavailable.
	AllowPartial bool

	Metadata map[string]string
}

// IdentityProfile is the normalized post-login identity snapshot used by
// handlers, middleware and Kernel IAM sync logic.
type IdentityProfile struct {
	Principal Principal

	// User is the canonical user projection from the identity provider when it
	// can be read. It may be zero when only token claims were available.
	User User

	// Groups contains expanded group objects. Principal.Groups and User.Groups
	// are usually only group IDs/names; this slice carries display names, parent
	// IDs and group type when available.
	Groups []Group

	// CurrentApplication is the login/OIDC application used by the current flow.
	// A user does not usually "belong" to an application in the same way they
	// belong to a group; the application here means the OAuth client that issued
	// the token or handled the login.
	CurrentApplication Application

	Warnings   []string
	Attributes AttributeSet
}

func (p IdentityProfile) HasFullUser() bool {
	return p.User.ID != "" || p.User.Username != ""
}

func (p IdentityProfile) GroupIDs() []string {
	out := make([]string, 0, len(p.Groups))
	seen := map[string]struct{}{}
	for _, group := range p.Groups {
		id := firstNonEmptyLocal(group.ID, group.Name)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (p IdentityProfile) IsPartial() bool {
	return len(p.Warnings) > 0
}

func firstNonEmptyLocal(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
