package db

import (
	"encoding/json"
	"time"

	"github.com/aisphereio/kernel/iamx"
)

type userModel struct {
	OrgID, ID, ExternalID, Provider, Username, Name, Email, Phone string
	Enabled, Locked, Deleted                                      bool
	Attributes                                                    string
	CreatedAt, UpdatedAt                                          time.Time
}

type orgModel struct {
	ID, ExternalID, Provider, ParentID, Name, DisplayName, OwnerID string
	Enabled                                                        bool
	Attributes                                                     string
	CreatedAt, UpdatedAt                                           time.Time
}

type groupModel struct {
	OrgID, ID, ExternalID, Provider, ParentID, Name, DisplayName, Type, Path string
	Enabled                                                                  bool
	Attributes                                                               string
	CreatedAt, UpdatedAt                                                     time.Time
}

type membershipModel struct {
	OrgID, GroupID, UserID, RoleIDs, Source string
	CreatedAt                               time.Time
}

func attrsToString(attrs iamx.Attributes) string {
	if len(attrs) == 0 {
		return "{}"
	}
	b, err := json.Marshal(attrs)
	if err != nil {
		return "{}"
	}
	return string(b)
}
func attrsFromString(raw string) iamx.Attributes {
	if raw == "" {
		return iamx.Attributes{}
	}
	out := iamx.Attributes{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return iamx.Attributes{}
	}
	return out
}
func stringsToJSON(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}
func stringsFromJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func toUserModel(user iamx.User) userModel {
	user = user.Normalize()
	return userModel{OrgID: user.OrgID, ID: user.ID, ExternalID: user.ExternalID, Provider: user.Provider, Username: user.Username, Name: user.Name, Email: user.Email, Phone: user.Phone, Enabled: user.Enabled, Locked: user.Locked, Deleted: user.Deleted, Attributes: attrsToString(user.Attributes), CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
func fromUserModel(m userModel) iamx.User {
	return iamx.User{OrgID: m.OrgID, ID: m.ID, ExternalID: m.ExternalID, Provider: m.Provider, Username: m.Username, Name: m.Name, Email: m.Email, Phone: m.Phone, Enabled: m.Enabled, Locked: m.Locked, Deleted: m.Deleted, Attributes: attrsFromString(m.Attributes), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}.Normalize()
}
func toOrgModel(org iamx.Organization) orgModel {
	org = org.Normalize()
	return orgModel{ID: org.ID, ExternalID: org.ExternalID, Provider: org.Provider, ParentID: org.ParentID, Name: org.Name, DisplayName: org.DisplayName, OwnerID: org.OwnerID, Enabled: org.Enabled, Attributes: attrsToString(org.Attributes), CreatedAt: org.CreatedAt, UpdatedAt: org.UpdatedAt}
}
func fromOrgModel(m orgModel) iamx.Organization {
	return iamx.Organization{ID: m.ID, ExternalID: m.ExternalID, Provider: m.Provider, ParentID: m.ParentID, Name: m.Name, DisplayName: m.DisplayName, OwnerID: m.OwnerID, Enabled: m.Enabled, Attributes: attrsFromString(m.Attributes), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}.Normalize()
}
func toGroupModel(group iamx.Group) groupModel {
	group = group.Normalize()
	return groupModel{OrgID: group.OrgID, ID: group.ID, ExternalID: group.ExternalID, Provider: group.Provider, ParentID: group.ParentID, Name: group.Name, DisplayName: group.DisplayName, Type: group.Type, Path: group.Path, Enabled: group.Enabled, Attributes: attrsToString(group.Attributes), CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt}
}
func fromGroupModel(m groupModel) iamx.Group {
	return iamx.Group{OrgID: m.OrgID, ID: m.ID, ExternalID: m.ExternalID, Provider: m.Provider, ParentID: m.ParentID, Name: m.Name, DisplayName: m.DisplayName, Type: m.Type, Path: m.Path, Enabled: m.Enabled, Attributes: attrsFromString(m.Attributes), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}.Normalize()
}
func toMembershipModel(m iamx.Membership) membershipModel {
	m = m.Normalize()
	return membershipModel{OrgID: m.OrgID, GroupID: m.GroupID, UserID: m.UserID, RoleIDs: stringsToJSON(m.RoleIDs), Source: m.Source, CreatedAt: m.CreatedAt}
}
func fromMembershipModel(m membershipModel) iamx.Membership {
	return iamx.Membership{OrgID: m.OrgID, GroupID: m.GroupID, UserID: m.UserID, RoleIDs: stringsFromJSON(m.RoleIDs), Source: m.Source, CreatedAt: m.CreatedAt}.Normalize()
}
