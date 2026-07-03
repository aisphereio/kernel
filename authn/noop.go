package authn

import "context"

// NoopAdmin returns an IdentityAdmin that reports unsupported operations. Use it
// as a safe default when identity provisioning is explicitly disabled.
func NoopAdmin() IdentityAdmin { return noopAdmin{} }

type noopAdmin struct{}

func (noopAdmin) ExchangeCode(context.Context, AuthCodeExchangeRequest) (TokenSet, Principal, error) {
	return TokenSet{}, Principal{}, ErrIdentityBackendFailed("authn code exchange is disabled", nil)
}
func (noopAdmin) RefreshToken(context.Context, RefreshTokenRequest) (TokenSet, error) {
	return TokenSet{}, ErrIdentityBackendFailed("authn token refresh is disabled", nil)
}
func (noopAdmin) VerifyToken(context.Context, VerifyTokenRequest) (Principal, error) {
	return Principal{}, ErrInvalidCredential("authn token verification is disabled")
}
func (noopAdmin) RevokeToken(context.Context, RevokeTokenRequest) error {
	return ErrIdentityBackendFailed("authn token revoke is disabled", nil)
}
func (noopAdmin) BuildLogoutURL(context.Context, LogoutURLRequest) (LogoutURL, error) {
	return LogoutURL{}, ErrIdentityBackendFailed("authn logout is disabled", nil)
}
func (noopAdmin) GetUser(context.Context, string, string) (User, error) {
	return User{}, ErrIdentityBackendFailed("authn user directory is disabled", nil)
}
func (noopAdmin) FindUsers(context.Context, UserFilter) ([]User, error) {
	return nil, ErrIdentityBackendFailed("authn user directory is disabled", nil)
}
func (noopAdmin) UpsertUser(context.Context, User) (User, error) {
	return User{}, ErrIdentityBackendFailed("authn user provisioning is disabled", nil)
}
func (noopAdmin) DisableUser(context.Context, string, string) error {
	return ErrIdentityBackendFailed("authn user provisioning is disabled", nil)
}
func (noopAdmin) CreateOrganization(context.Context, CreateOrganizationRequest) (Organization, error) {
	return Organization{}, ErrIdentityBackendFailed("authn organization provisioning is disabled", nil)
}
func (noopAdmin) GetOrganization(context.Context, string) (Organization, error) {
	return Organization{}, ErrIdentityBackendFailed("authn organization provisioning is disabled", nil)
}
func (noopAdmin) UpdateOrganization(context.Context, UpdateOrganizationRequest) (Organization, error) {
	return Organization{}, ErrIdentityBackendFailed("authn organization provisioning is disabled", nil)
}
func (noopAdmin) DeleteOrganization(context.Context, DeleteOrganizationRequest) error {
	return ErrIdentityBackendFailed("authn organization provisioning is disabled", nil)
}
func (noopAdmin) CreateApplication(context.Context, CreateApplicationRequest) (Application, error) {
	return Application{}, ErrIdentityBackendFailed("authn application provisioning is disabled", nil)
}
func (noopAdmin) GetApplication(context.Context, string, string) (Application, error) {
	return Application{}, ErrIdentityBackendFailed("authn application provisioning is disabled", nil)
}
func (noopAdmin) UpdateApplication(context.Context, UpdateApplicationRequest) (Application, error) {
	return Application{}, ErrIdentityBackendFailed("authn application provisioning is disabled", nil)
}
func (noopAdmin) DeleteApplication(context.Context, DeleteApplicationRequest) error {
	return ErrIdentityBackendFailed("authn application provisioning is disabled", nil)
}
func (noopAdmin) CreateGroup(context.Context, CreateGroupRequest) (Group, error) {
	return Group{}, ErrIdentityBackendFailed("authn group provisioning is disabled", nil)
}
func (noopAdmin) GetGroup(context.Context, string, string) (Group, error) {
	return Group{}, ErrIdentityBackendFailed("authn group provisioning is disabled", nil)
}
func (noopAdmin) ListGroups(context.Context, GroupFilter) ([]Group, error) {
	return nil, ErrIdentityBackendFailed("authn group provisioning is disabled", nil)
}
func (noopAdmin) UpdateGroup(context.Context, UpdateGroupRequest) (Group, error) {
	return Group{}, ErrIdentityBackendFailed("authn group provisioning is disabled", nil)
}
func (noopAdmin) DeleteGroup(context.Context, DeleteGroupRequest) error {
	return ErrIdentityBackendFailed("authn group provisioning is disabled", nil)
}
func (noopAdmin) AssignUserToGroup(context.Context, AssignUserToGroupRequest) error {
	return ErrIdentityBackendFailed("authn group membership provisioning is disabled", nil)
}
func (noopAdmin) RemoveUserFromGroup(context.Context, AssignUserToGroupRequest) error {
	return ErrIdentityBackendFailed("authn group membership provisioning is disabled", nil)
}
