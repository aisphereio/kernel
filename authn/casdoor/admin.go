package casdoor

import (
	"context"
	"slices"
	"strings"

	"github.com/aisphereio/kernel/authn"
	casdoorsdk "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

var _ authn.IdentityAdmin = (*Client)(nil)

func (c *Client) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return c.UpsertUser(ctx, req.User)
}

func (c *Client) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return c.UpsertUser(ctx, req.User)
}

func (c *Client) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	_ = ctx
	userID := firstNonEmpty(req.UserID, req.Metadata["username"])
	if userID == "" {
		return authn.ErrInvalidTokenRequest("user id is required")
	}
	if !req.Hard {
		return c.DisableUser(ctx, req.OrgID, userID)
	}
	ok, err := c.adminSDK().DeleteUser(&casdoorsdk.User{Owner: firstNonEmpty(req.OrgID, c.cfg.OrganizationName), Name: userID})
	if err != nil {
		return wrapBackend("casdoor delete user failed", err)
	}
	if !ok {
		return wrapBackend("casdoor delete user returned not affected", nil)
	}
	return nil
}

func (c *Client) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	_ = ctx
	user, err := c.adminSDK().GetUser(userID)
	if err != nil {
		return authn.User{}, wrapBackend("casdoor get user failed", err)
	}
	if user == nil {
		return authn.User{}, authn.ErrUnauthenticated("casdoor user not found")
	}
	return userFromSDK(user), nil
}

func (c *Client) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	_ = ctx
	sdk := c.adminSDK()
	switch {
	case filter.Email != "":
		user, err := sdk.GetUserByEmail(filter.Email)
		return singleUser(user, err)
	case filter.Phone != "":
		user, err := sdk.GetUserByPhone(filter.Phone)
		return singleUser(user, err)
	case filter.ID != "" || filter.Username != "":
		user, err := sdk.GetUser(firstNonEmpty(filter.Username, filter.ID))
		return singleUser(user, err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	page := filter.Offset/limit + 1
	query := map[string]string{}
	if filter.GroupID != "" {
		query["group"] = filter.GroupID
	}
	users, _, err := sdk.GetPaginationUsers(page, limit, query)
	if err != nil {
		return nil, wrapBackend("casdoor find users failed", err)
	}
	out := make([]authn.User, 0, len(users))
	for _, user := range users {
		if user != nil && matchesUserFilter(user, filter) {
			out = append(out, userFromSDK(user))
		}
	}
	return out, nil
}

func (c *Client) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	_ = ctx
	sdk := c.adminSDK()
	cu := userToSDK(user, c.cfg.OrganizationName)
	if cu.Name == "" {
		return authn.User{}, authn.ErrInvalidTokenRequest("user username or id is required")
	}
	existing, err := sdk.GetUser(cu.Name)
	if err == nil && existing != nil && existing.Name != "" {
		mergeUser(existing, cu)
		ok, err := sdk.UpdateUser(existing)
		if err != nil {
			return authn.User{}, wrapBackend("casdoor update user failed", err)
		}
		if !ok {
			return authn.User{}, wrapBackend("casdoor update user returned not affected", nil)
		}
		return userFromSDK(existing), nil
	}
	ok, err := sdk.AddUser(cu)
	if err != nil {
		return authn.User{}, wrapBackend("casdoor add user failed", err)
	}
	if !ok {
		return authn.User{}, wrapBackend("casdoor add user returned not affected", nil)
	}
	return userFromSDK(cu), nil
}

func (c *Client) DisableUser(ctx context.Context, orgID, userID string) error {
	_ = ctx
	sdk := c.adminSDK()
	user, err := sdk.GetUser(userID)
	if err != nil {
		return wrapBackend("casdoor get user before disable failed", err)
	}
	user.IsForbidden = true
	ok, err := sdk.UpdateUser(user)
	if err != nil {
		return wrapBackend("casdoor disable user failed", err)
	}
	if !ok {
		return wrapBackend("casdoor disable user returned not affected", nil)
	}
	return nil
}

func (c *Client) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	_ = ctx
	org := orgToSDK(req.Organization)
	ok, err := c.adminSDK().AddOrganization(org)
	if err != nil {
		return authn.Organization{}, wrapBackend("casdoor add organization failed", err)
	}
	if !ok {
		return authn.Organization{}, wrapBackend("casdoor add organization returned not affected", nil)
	}
	return orgFromSDK(org), nil
}

func (c *Client) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	_ = ctx
	org, err := c.adminSDK().GetOrganization(orgID)
	if err != nil {
		return authn.Organization{}, wrapBackend("casdoor get organization failed", err)
	}
	return orgFromSDK(org), nil
}

func (c *Client) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	_ = ctx
	org := orgToSDK(req.Organization)
	ok, err := c.adminSDK().UpdateOrganization(org)
	if err != nil {
		return authn.Organization{}, wrapBackend("casdoor update organization failed", err)
	}
	if !ok {
		return authn.Organization{}, wrapBackend("casdoor update organization returned not affected", nil)
	}
	return orgFromSDK(org), nil
}

func (c *Client) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	_ = ctx
	ok, err := c.adminSDK().DeleteOrganization(&casdoorsdk.Organization{Owner: "admin", Name: req.OrgID})
	if err != nil {
		return wrapBackend("casdoor delete organization failed", err)
	}
	if !ok {
		return wrapBackend("casdoor delete organization returned not affected", nil)
	}
	return nil
}

func (c *Client) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	_ = ctx
	app := appToSDK(req.Application)
	ok, err := c.adminSDK().AddApplication(app)
	if err != nil {
		return authn.Application{}, wrapBackend("casdoor add application failed", err)
	}
	if !ok {
		return authn.Application{}, wrapBackend("casdoor add application returned not affected", nil)
	}
	return appFromSDK(app), nil
}

func (c *Client) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	_ = ctx
	app, err := c.adminSDK().GetApplication(appID)
	if err != nil {
		return authn.Application{}, wrapBackend("casdoor get application failed", err)
	}
	return appFromSDK(app), nil
}

func (c *Client) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	_ = ctx
	app := appToSDK(req.Application)
	ok, err := c.adminSDK().UpdateApplication(app)
	if err != nil {
		return authn.Application{}, wrapBackend("casdoor update application failed", err)
	}
	if !ok {
		return authn.Application{}, wrapBackend("casdoor update application returned not affected", nil)
	}
	return appFromSDK(app), nil
}

func (c *Client) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	_ = ctx
	ok, err := c.adminSDK().DeleteApplication(&casdoorsdk.Application{Owner: "admin", Name: req.AppID})
	if err != nil {
		return wrapBackend("casdoor delete application failed", err)
	}
	if !ok {
		return wrapBackend("casdoor delete application returned not affected", nil)
	}
	return nil
}

func (c *Client) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	_ = ctx
	group := groupToSDK(req.Group)
	ok, err := c.adminSDK().AddGroup(group)
	if err != nil {
		return authn.Group{}, wrapBackend("casdoor add group failed", err)
	}
	if !ok {
		return authn.Group{}, wrapBackend("casdoor add group returned not affected", nil)
	}
	return groupFromSDK(group), nil
}

func (c *Client) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	_ = ctx
	group, err := c.adminSDK().GetGroup(groupID)
	if err != nil {
		return authn.Group{}, wrapBackend("casdoor get group failed", err)
	}
	return groupFromSDK(group), nil
}

func (c *Client) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	_ = ctx
	groups, err := c.adminSDK().GetGroups()
	if err != nil {
		return nil, wrapBackend("casdoor list groups failed", err)
	}
	out := make([]authn.Group, 0, len(groups))
	for _, group := range groups {
		if group == nil || !matchesGroupFilter(group, filter) {
			continue
		}
		out = append(out, groupFromSDK(group))
	}
	return out, nil
}

func (c *Client) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	_ = ctx
	group := groupToSDK(req.Group)
	ok, err := c.adminSDK().UpdateGroup(group)
	if err != nil {
		return authn.Group{}, wrapBackend("casdoor update group failed", err)
	}
	if !ok {
		return authn.Group{}, wrapBackend("casdoor update group returned not affected", nil)
	}
	return groupFromSDK(group), nil
}

func (c *Client) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	_ = ctx
	ok, err := c.adminSDK().DeleteGroup(&casdoorsdk.Group{Owner: req.OrgID, Name: req.GroupID})
	if err != nil {
		return wrapBackend("casdoor delete group failed", err)
	}
	if !ok {
		return wrapBackend("casdoor delete group returned not affected", nil)
	}
	return nil
}

func (c *Client) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	_ = ctx
	return c.updateGroupMembership(req, true)
}

func (c *Client) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	_ = ctx
	return c.updateGroupMembership(req, false)
}

func (c *Client) updateGroupMembership(req authn.AssignUserToGroupRequest, add bool) error {
	sdk := c.adminSDK()
	group, err := sdk.GetGroup(req.GroupID)
	if err != nil {
		return wrapBackend("casdoor get group before membership update failed", err)
	}
	if group == nil {
		return wrapBackend("casdoor group not found", nil)
	}
	if add && !slices.Contains(group.Users, req.UserID) {
		group.Users = append(group.Users, req.UserID)
	}
	if !add {
		group.Users = removeString(group.Users, req.UserID)
	}
	ok, err := sdk.UpdateGroup(group)
	if err != nil {
		return wrapBackend("casdoor update group membership failed", err)
	}
	if !ok {
		return wrapBackend("casdoor update group membership returned not affected", nil)
	}

	user, err := sdk.GetUser(req.UserID)
	if err != nil {
		return wrapBackend("casdoor get user before membership update failed", err)
	}
	if user == nil {
		return wrapBackend("casdoor user not found", nil)
	}
	if add && !slices.Contains(user.Groups, req.GroupID) {
		user.Groups = append(user.Groups, req.GroupID)
	}
	if !add {
		user.Groups = removeString(user.Groups, req.GroupID)
	}
	ok, err = sdk.UpdateUser(user)
	if err != nil {
		return wrapBackend("casdoor update user membership failed", err)
	}
	if !ok {
		return wrapBackend("casdoor update user membership returned not affected", nil)
	}
	return nil
}

func singleUser(user *casdoorsdk.User, err error) ([]authn.User, error) {
	if err != nil {
		return nil, wrapBackend("casdoor find user failed", err)
	}
	if user == nil || user.Name == "" {
		return nil, nil
	}
	return []authn.User{userFromSDK(user)}, nil
}

func matchesUserFilter(user *casdoorsdk.User, filter authn.UserFilter) bool {
	if filter.Role != "" && !hasRole(user.Roles, filter.Role) {
		return false
	}
	if filter.GroupID != "" && !slices.Contains(user.Groups, filter.GroupID) {
		return false
	}
	return true
}

func hasRole(roles []*casdoorsdk.Role, name string) bool {
	for _, role := range roles {
		if role != nil && role.Name == name {
			return true
		}
	}
	return false
}

func matchesGroupFilter(group *casdoorsdk.Group, filter authn.GroupFilter) bool {
	if filter.OrgID != "" && group.Owner != filter.OrgID {
		return false
	}
	if filter.ParentID != "" && group.ParentId != filter.ParentID {
		return false
	}
	if filter.Type != "" && group.Type != filter.Type {
		return false
	}
	if filter.UserID != "" && !slices.Contains(group.Users, filter.UserID) {
		return false
	}
	return true
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func mergeUser(dst, src *casdoorsdk.User) {
	dst.DisplayName = firstNonEmpty(src.DisplayName, dst.DisplayName)
	dst.Email = firstNonEmpty(src.Email, dst.Email)
	dst.Phone = firstNonEmpty(src.Phone, dst.Phone)
	if len(src.Groups) > 0 {
		dst.Groups = append([]string(nil), src.Groups...)
	}
}

func userFromSDK(user *casdoorsdk.User) authn.User {
	if user == nil {
		return authn.User{}
	}
	return authn.User{
		ID:          firstNonEmpty(user.Id, user.Name),
		ExternalID:  user.ExternalId,
		Provider:    ProviderName,
		OrgID:       user.Owner,
		Username:    user.Name,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Phone:       user.Phone,
		Roles:       roleNames(user.Roles),
		Groups:      append([]string(nil), user.Groups...),
		Enabled:     !user.IsForbidden && !user.IsDeleted,
		Attributes: authn.AttributeSet{
			"casdoor_id":         user.Id,
			"signup_application": user.SignupApplication,
		},
	}
}

func userToSDK(user authn.User, defaultOrg string) *casdoorsdk.User {
	return &casdoorsdk.User{
		Owner:       firstNonEmpty(user.OrgID, defaultOrg),
		Name:        firstNonEmpty(user.Username, user.ID),
		Id:          user.ID,
		ExternalId:  user.ExternalID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Phone:       user.Phone,
		Groups:      append([]string(nil), user.Groups...),
		IsForbidden: !user.Enabled,
	}
}

func orgFromSDK(org *casdoorsdk.Organization) authn.Organization {
	if org == nil {
		return authn.Organization{}
	}
	return authn.Organization{
		ID:          org.Name,
		ExternalID:  org.Name,
		Name:        org.Name,
		DisplayName: org.DisplayName,
		Tags:        append([]string(nil), org.Tags...),
		Enabled:     !org.DisableSignin,
	}
}

func orgToSDK(org authn.Organization) *casdoorsdk.Organization {
	return &casdoorsdk.Organization{
		Owner:         "admin",
		Name:          firstNonEmpty(org.Name, org.ID),
		DisplayName:   org.DisplayName,
		Tags:          append([]string(nil), org.Tags...),
		DisableSignin: !org.Enabled,
	}
}

func appFromSDK(app *casdoorsdk.Application) authn.Application {
	if app == nil {
		return authn.Application{}
	}
	return authn.Application{
		ID:             app.Name,
		ExternalID:     app.Name,
		OrgID:          app.Organization,
		Name:           app.Name,
		DisplayName:    app.DisplayName,
		ClientID:       app.ClientId,
		ClientSecret:   app.ClientSecret,
		RedirectURIs:   append([]string(nil), app.RedirectUris...),
		GrantTypes:     append([]string(nil), app.GrantTypes...),
		EnablePassword: app.EnablePassword,
		EnableSignup:   app.EnableSignUp,
		Attributes: authn.AttributeSet{
			"homepage_url": app.HomepageUrl,
			"cert":         app.Cert,
		},
	}
}

func appToSDK(app authn.Application) *casdoorsdk.Application {
	return &casdoorsdk.Application{
		Owner:          "admin",
		Name:           firstNonEmpty(app.Name, app.ID),
		DisplayName:    app.DisplayName,
		Organization:   app.OrgID,
		ClientId:       app.ClientID,
		ClientSecret:   app.ClientSecret,
		RedirectUris:   append([]string(nil), app.RedirectURIs...),
		GrantTypes:     append([]string(nil), app.GrantTypes...),
		EnablePassword: app.EnablePassword,
		EnableSignUp:   app.EnableSignup,
	}
}

func groupFromSDK(group *casdoorsdk.Group) authn.Group {
	if group == nil {
		return authn.Group{}
	}
	return authn.Group{
		ID:          group.Name,
		ExternalID:  group.Name,
		OrgID:       group.Owner,
		ParentID:    group.ParentId,
		Name:        group.Name,
		DisplayName: group.DisplayName,
		Type:        group.Type,
		Path:        strings.Trim(group.Key, "/"),
		Users:       append([]string(nil), group.Users...),
		Attributes: authn.AttributeSet{
			"manager":       group.Manager,
			"contact_email": group.ContactEmail,
			"is_top_group":  group.IsTopGroup,
			"is_enabled":    group.IsEnabled,
		},
	}
}

func groupToSDK(group authn.Group) *casdoorsdk.Group {
	return &casdoorsdk.Group{
		Owner:       group.OrgID,
		Name:        firstNonEmpty(group.Name, group.ID),
		DisplayName: group.DisplayName,
		ParentId:    group.ParentID,
		Type:        firstNonEmpty(group.Type, authn.GroupTypeVirtual),
		Users:       append([]string(nil), group.Users...),
		IsEnabled:   true,
	}
}
