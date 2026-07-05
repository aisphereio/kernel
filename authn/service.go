package authn

import "context"

// Authenticator verifies a transport credential and returns a normalized Principal.
type Authenticator interface {
	Authenticate(ctx context.Context, credential Credential) (Principal, error)
}

// TokenService covers OAuth/OIDC token flows such as Casdoor code exchange,
// refresh, validation and revocation.
type TokenService interface {
	ExchangeCode(ctx context.Context, req AuthCodeExchangeRequest) (TokenSet, Principal, error)
	RefreshToken(ctx context.Context, req RefreshTokenRequest) (TokenSet, error)
	VerifyToken(ctx context.Context, req VerifyTokenRequest) (Principal, error)
	RevokeToken(ctx context.Context, req RevokeTokenRequest) error
}

// UserDirectory reads identity-provider users. Business modules should usually
// go through Kernel IAM service, not a provider SDK directly.
type UserDirectory interface {
	GetUser(ctx context.Context, orgID, userID string) (User, error)
	FindUsers(ctx context.Context, filter UserFilter) ([]User, error)
}

// UserAdmin manages identity-provider users. Passwords, MFA and session state are
// owned by the backing identity provider, such as Casdoor. Kernel only exposes
// the management surface; it must not store password hashes or local sessions.
type UserAdmin interface {
	CreateUser(ctx context.Context, req CreateUserRequest) (User, error)
	UpdateUser(ctx context.Context, req UpdateUserRequest) (User, error)
	DeleteUser(ctx context.Context, req DeleteUserRequest) error
	UpsertUser(ctx context.Context, user User) (User, error)
	DisableUser(ctx context.Context, orgID, userID string) error
}

// OrganizationAdmin manages identity-provider organizations.
type OrganizationAdmin interface {
	CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (Organization, error)
	GetOrganization(ctx context.Context, orgID string) (Organization, error)
	UpdateOrganization(ctx context.Context, req UpdateOrganizationRequest) (Organization, error)
	DeleteOrganization(ctx context.Context, req DeleteOrganizationRequest) error
}

// ApplicationAdmin manages identity-provider applications / OAuth clients.
type ApplicationAdmin interface {
	CreateApplication(ctx context.Context, req CreateApplicationRequest) (Application, error)
	GetApplication(ctx context.Context, orgID, appID string) (Application, error)
	UpdateApplication(ctx context.Context, req UpdateApplicationRequest) (Application, error)
	DeleteApplication(ctx context.Context, req DeleteApplicationRequest) error
}

// GroupAdmin manages app/org group trees. Casdoor maps this naturally
// to organization groups with ParentGroup.
type GroupAdmin interface {
	CreateGroup(ctx context.Context, req CreateGroupRequest) (Group, error)
	GetGroup(ctx context.Context, orgID, groupID string) (Group, error)
	ListGroups(ctx context.Context, filter GroupFilter) ([]Group, error)
	UpdateGroup(ctx context.Context, req UpdateGroupRequest) (Group, error)
	DeleteGroup(ctx context.Context, req DeleteGroupRequest) error
	AssignUserToGroup(ctx context.Context, req AssignUserToGroupRequest) error
	RemoveUserFromGroup(ctx context.Context, req AssignUserToGroupRequest) error
}

// IdentityAdmin is the identity management surface expected from a Casdoor
// adapter. Keep it out of normal business services; use it in IAM provisioning
// after platform-level authorization has been checked.
type IdentityAdmin interface {
	TokenService
	UserDirectory
	UserAdmin
	OrganizationAdmin
	ApplicationAdmin
	GroupAdmin
}

// LogoutService builds identity-provider hosted logout (end-session) URLs.
//
// Implementations construct the provider-specific end-session endpoint URL
// with the required OIDC RP-Initiated Logout parameters (id_token_hint,
// post_logout_redirect_uri, state, client_id). The returned LogoutURL.URL
// is the full redirect target for a 302 response.
type LogoutService interface {
	BuildLogoutURL(ctx context.Context, req LogoutURLRequest) (LogoutURL, error)
}

// TokenVerifier adapts JWT/OIDC libraries into Authenticator.
type TokenVerifier func(ctx context.Context, req VerifyTokenRequest) (Principal, error)
