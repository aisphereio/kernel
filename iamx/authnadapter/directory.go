// Package authnadapter adapts authn.IdentityAdmin providers, such as Casdoor,
// into Kernel IAM directory contracts. It is the bridge that lets Kernel keep
// its own IAM domain model while Casdoor remains an OAuth/OIDC and user-center
// backend.
package authnadapter

import (
	"context"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/iamx"
)

type Directory struct{ provider authn.IdentityAdmin }

func NewDirectory(provider authn.IdentityAdmin) (*Directory, error) {
	if provider == nil {
		return nil, iamx.ErrInvalidArgument("identity admin provider is required")
	}
	return &Directory{provider: provider}, nil
}

func (d *Directory) GetUser(ctx context.Context, orgID, userID string) (iamx.User, error) {
	user, err := d.provider.GetUser(ctx, orgID, userID)
	if err != nil {
		return iamx.User{}, err
	}
	return FromAuthnUser(user), nil
}

func (d *Directory) ListUsers(ctx context.Context, q iamx.UserQuery) ([]iamx.User, error) {
	users, err := d.provider.FindUsers(ctx, authn.UserFilter{OrgID: q.OrgID, ID: q.ID, Username: q.Username, Email: q.Email, GroupID: q.GroupID, Limit: q.Limit, Offset: q.Offset})
	if err != nil {
		return nil, err
	}
	out := make([]iamx.User, 0, len(users))
	for _, user := range users {
		out = append(out, FromAuthnUser(user))
	}
	return out, nil
}

func (d *Directory) CreateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	return d.UpsertUser(ctx, user)
}

func (d *Directory) UpdateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	return d.UpsertUser(ctx, user)
}

func (d *Directory) UpsertUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	out, err := d.provider.UpsertUser(ctx, ToAuthnUser(user))
	if err != nil {
		return iamx.User{}, err
	}
	return FromAuthnUser(out), nil
}

func (d *Directory) DisableUser(ctx context.Context, orgID, userID string) error {
	return d.provider.DisableUser(ctx, orgID, userID)
}

// DeleteUser is a soft-delete boundary for provider adapters. Casdoor-like
// providers may not expose hard delete through Kernel authn.IdentityAdmin yet,
// so we disable the external user instead of leaking provider-specific delete
// semantics to business code.
func (d *Directory) DeleteUser(ctx context.Context, orgID, userID string) error {
	return d.provider.DisableUser(ctx, orgID, userID)
}

func (d *Directory) GetOrganization(ctx context.Context, orgID string) (iamx.Organization, error) {
	org, err := d.provider.GetOrganization(ctx, orgID)
	if err != nil {
		return iamx.Organization{}, err
	}
	return FromAuthnOrganization(org), nil
}

func (d *Directory) ListOrganizations(ctx context.Context, q iamx.OrganizationQuery) ([]iamx.Organization, error) {
	// authn.IdentityAdmin currently has no list organization method. Kernel IAM
	// keeps the list contract for DB-backed implementations; provider adapters
	// expose only single-org reads until the provider surface is expanded.
	if q.Name != "" {
		return []iamx.Organization{}, nil
	}
	return nil, iamx.ErrInvalidArgument("list organizations is not supported by this provider adapter")
}

func (d *Directory) CreateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	out, err := d.provider.CreateOrganization(ctx, authn.CreateOrganizationRequest{Organization: ToAuthnOrganization(org)})
	if err != nil {
		return iamx.Organization{}, err
	}
	return FromAuthnOrganization(out), nil
}

func (d *Directory) UpdateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	out, err := d.provider.UpdateOrganization(ctx, authn.UpdateOrganizationRequest{Organization: ToAuthnOrganization(org)})
	if err != nil {
		return iamx.Organization{}, err
	}
	return FromAuthnOrganization(out), nil
}

func (d *Directory) UpsertOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	org = org.Normalize()
	if _, err := d.provider.GetOrganization(ctx, org.ID); err == nil {
		out, err := d.provider.UpdateOrganization(ctx, authn.UpdateOrganizationRequest{Organization: ToAuthnOrganization(org)})
		if err != nil {
			return iamx.Organization{}, err
		}
		return FromAuthnOrganization(out), nil
	}
	out, err := d.provider.CreateOrganization(ctx, authn.CreateOrganizationRequest{Organization: ToAuthnOrganization(org)})
	if err != nil {
		return iamx.Organization{}, err
	}
	return FromAuthnOrganization(out), nil
}

func (d *Directory) DeleteOrganization(ctx context.Context, orgID string) error {
	return d.provider.DeleteOrganization(ctx, authn.DeleteOrganizationRequest{OrgID: orgID})
}

func (d *Directory) GetGroup(ctx context.Context, orgID, groupID string) (iamx.Group, error) {
	group, err := d.provider.GetGroup(ctx, orgID, groupID)
	if err != nil {
		return iamx.Group{}, err
	}
	return FromAuthnGroup(group), nil
}

func (d *Directory) ListGroups(ctx context.Context, q iamx.GroupQuery) ([]iamx.Group, error) {
	groups, err := d.provider.ListGroups(ctx, authn.GroupFilter{OrgID: q.OrgID, ParentID: q.ParentID, Type: q.Type, UserID: q.UserID, Limit: q.Limit, Offset: q.Offset})
	if err != nil {
		return nil, err
	}
	out := make([]iamx.Group, 0, len(groups))
	for _, group := range groups {
		out = append(out, FromAuthnGroup(group))
	}
	return out, nil
}

func (d *Directory) CreateGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	out, err := d.provider.CreateGroup(ctx, authn.CreateGroupRequest{Group: ToAuthnGroup(group)})
	if err != nil {
		return iamx.Group{}, err
	}
	return FromAuthnGroup(out), nil
}

func (d *Directory) UpdateGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	out, err := d.provider.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: ToAuthnGroup(group)})
	if err != nil {
		return iamx.Group{}, err
	}
	return FromAuthnGroup(out), nil
}

func (d *Directory) UpsertGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	group = group.Normalize()
	if _, err := d.provider.GetGroup(ctx, group.OrgID, group.ID); err == nil {
		out, err := d.provider.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: ToAuthnGroup(group)})
		if err != nil {
			return iamx.Group{}, err
		}
		return FromAuthnGroup(out), nil
	}
	out, err := d.provider.CreateGroup(ctx, authn.CreateGroupRequest{Group: ToAuthnGroup(group)})
	if err != nil {
		return iamx.Group{}, err
	}
	return FromAuthnGroup(out), nil
}

func (d *Directory) DeleteGroup(ctx context.Context, orgID, groupID string) error {
	return d.provider.DeleteGroup(ctx, authn.DeleteGroupRequest{OrgID: orgID, GroupID: groupID})
}

func (d *Directory) ListGroupAncestors(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	groups, err := d.provider.ListGroups(ctx, authn.GroupFilter{OrgID: orgID})
	if err != nil {
		return nil, err
	}
	byID := map[string]iamx.Group{}
	for _, group := range groups {
		mapped := FromAuthnGroup(group)
		byID[mapped.ID] = mapped
	}
	group, ok := byID[groupID]
	if !ok {
		return nil, iamx.ErrNotFound("group not found")
	}
	out := []iamx.Group{}
	seen := map[string]bool{}
	for group.ParentID != "" {
		if seen[group.ParentID] {
			return nil, iamx.ErrInvalidArgument("group cycle detected")
		}
		seen[group.ParentID] = true
		parent, ok := byID[group.ParentID]
		if !ok {
			return nil, iamx.ErrInvalidArgument("parent group not found")
		}
		out = append(out, parent)
		group = parent
	}
	return out, nil
}

func (d *Directory) ListGroupDescendants(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	groups, err := d.provider.ListGroups(ctx, authn.GroupFilter{OrgID: orgID})
	if err != nil {
		return nil, err
	}
	children := map[string][]iamx.Group{}
	found := false
	for _, group := range groups {
		mapped := FromAuthnGroup(group)
		if mapped.ID == groupID {
			found = true
		}
		children[mapped.ParentID] = append(children[mapped.ParentID], mapped)
	}
	if !found {
		return nil, iamx.ErrNotFound("group not found")
	}
	out := []iamx.Group{}
	var walk func(parent string)
	walk = func(parent string) {
		for _, child := range children[parent] {
			out = append(out, child)
			walk(child.ID)
		}
	}
	walk(groupID)
	return out, nil
}

func (d *Directory) AddMembership(ctx context.Context, m iamx.Membership) error {
	m = m.Normalize()
	return d.provider.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{OrgID: m.OrgID, GroupID: m.GroupID, UserID: m.UserID})
}

func (d *Directory) RemoveMembership(ctx context.Context, orgID, groupID, userID string) error {
	return d.provider.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{OrgID: orgID, GroupID: groupID, UserID: userID})
}

func (d *Directory) ListMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	groups, err := d.provider.ListGroups(ctx, authn.GroupFilter{OrgID: orgID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]iamx.Membership, 0, len(groups))
	for _, group := range groups {
		out = append(out, iamx.Membership{OrgID: orgID, UserID: userID, GroupID: group.ID, Source: iamx.MembershipDirect})
	}
	return out, nil
}

func (d *Directory) ListEffectiveMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	direct, err := d.ListMemberships(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]iamx.Membership, 0, len(direct))
	seen := map[string]bool{}
	for _, membership := range direct {
		out = append(out, membership)
		seen[membership.GroupID] = true
		ancestors, err := d.ListGroupAncestors(ctx, orgID, membership.GroupID)
		if err != nil {
			return nil, err
		}
		for _, group := range ancestors {
			if seen[group.ID] {
				continue
			}
			out = append(out, iamx.Membership{OrgID: orgID, UserID: userID, GroupID: group.ID, Source: iamx.MembershipInherited, CreatedAt: membership.CreatedAt})
			seen[group.ID] = true
		}
	}
	return out, nil
}

func FromAuthnUser(u authn.User) iamx.User {
	return iamx.User{ID: u.ID, ExternalID: u.ExternalID, Provider: u.Provider, OrgID: u.OrgID, Username: u.Username, Name: u.DisplayName, Email: u.Email, Phone: u.Phone, Enabled: u.Enabled, Attributes: iamx.Attributes(u.Attributes)}.Normalize()
}

func ToAuthnUser(u iamx.User) authn.User {
	u = u.Normalize()
	return authn.User{ID: u.ID, ExternalID: u.ExternalID, Provider: u.Provider, OrgID: u.OrgID, Username: u.Username, DisplayName: u.Name, Email: u.Email, Phone: u.Phone, Enabled: u.Enabled, Attributes: authn.AttributeSet(u.Attributes)}
}

func FromAuthnOrganization(o authn.Organization) iamx.Organization {
	return iamx.Organization{ID: o.ID, ExternalID: o.ExternalID, Provider: ProviderOrKernel(o.ExternalID), Name: o.Name, DisplayName: o.DisplayName, OwnerID: o.OwnerID, ParentID: o.ParentID, Enabled: o.Enabled, Attributes: iamx.Attributes(o.Attributes)}.Normalize()
}

func ToAuthnOrganization(o iamx.Organization) authn.Organization {
	o = o.Normalize()
	return authn.Organization{ID: o.ID, ExternalID: o.ExternalID, Name: o.Name, DisplayName: o.DisplayName, OwnerID: o.OwnerID, ParentID: o.ParentID, Enabled: o.Enabled, Attributes: authn.AttributeSet(o.Attributes)}
}

func FromAuthnGroup(g authn.Group) iamx.Group {
	return iamx.Group{ID: g.ID, ExternalID: g.ExternalID, Provider: iamx.ProviderCasdoor, OrgID: g.OrgID, ParentID: g.ParentID, Name: g.Name, DisplayName: g.DisplayName, Type: g.Type, Path: g.Path, Attributes: iamx.Attributes(g.Attributes)}.Normalize()
}

func ToAuthnGroup(g iamx.Group) authn.Group {
	g = g.Normalize()
	return authn.Group{ID: g.ID, ExternalID: g.ExternalID, OrgID: g.OrgID, ParentID: g.ParentID, Name: g.Name, DisplayName: g.DisplayName, Type: g.Type, Path: g.Path, Attributes: authn.AttributeSet(g.Attributes)}
}

func ProviderOrKernel(externalID string) string {
	if externalID != "" {
		return iamx.ProviderCasdoor
	}
	return iamx.ProviderKernel
}
