// Package iamx defines Kernel's provider-neutral IAM domain model.
//
// IAM owns users, organizations, groups and memberships inside Kernel. External
// systems such as Casdoor are adapters or sync sources; business packages must
// depend on iamx contracts, not on Casdoor object structs or SDK calls.
package iamx

import (
	"strings"
	"time"
)

const (
	ProviderCasdoor = "casdoor"
	ProviderKernel  = "kernel"
)

const (
	GroupTypePhysical = "physical"
	GroupTypeVirtual  = "virtual"
	GroupTypeTeam     = "team"
)

const (
	MembershipDirect    = "direct"
	MembershipInherited = "inherited"
)

type Attributes map[string]any

// User is Kernel's canonical user projection. It intentionally keeps fewer
// fields than Casdoor's object.User and stores provider-specific data under
// Attributes so the domain model stays stable.
type User struct {
	ID         string
	ExternalID string
	Provider   string

	OrgID    string
	Username string
	Name     string
	Email    string
	Phone    string

	Enabled    bool
	Locked     bool
	Deleted    bool
	Attributes Attributes

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u User) Normalize() User {
	u.ID = strings.TrimSpace(u.ID)
	u.ExternalID = strings.TrimSpace(u.ExternalID)
	u.Provider = strings.TrimSpace(u.Provider)
	if u.Provider == "" {
		u.Provider = ProviderKernel
	}
	u.OrgID = strings.TrimSpace(u.OrgID)
	u.Username = strings.TrimSpace(u.Username)
	u.Email = strings.TrimSpace(strings.ToLower(u.Email))
	return u
}

func (u User) Active() bool { return u.Normalize().Enabled && !u.Locked && !u.Deleted }

// Organization models a tenant/org boundary. ParentID allows multi-level orgs.
type Organization struct {
	ID          string
	ExternalID  string
	Provider    string
	ParentID    string
	Name        string
	DisplayName string
	OwnerID     string
	Enabled     bool
	Attributes  Attributes
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (o Organization) Normalize() Organization {
	o.ID = strings.TrimSpace(o.ID)
	o.ExternalID = strings.TrimSpace(o.ExternalID)
	o.Provider = strings.TrimSpace(o.Provider)
	if o.Provider == "" {
		o.Provider = ProviderKernel
	}
	o.ParentID = strings.TrimSpace(o.ParentID)
	o.Name = strings.TrimSpace(o.Name)
	return o
}

// Group models organizational groups/teams. ParentID creates a group tree.
type Group struct {
	ID          string
	ExternalID  string
	Provider    string
	OrgID       string
	ParentID    string
	Name        string
	DisplayName string
	Type        string
	Path        string
	Enabled     bool
	Attributes  Attributes
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (g Group) Normalize() Group {
	g.ID = strings.TrimSpace(g.ID)
	g.ExternalID = strings.TrimSpace(g.ExternalID)
	g.Provider = strings.TrimSpace(g.Provider)
	if g.Provider == "" {
		g.Provider = ProviderKernel
	}
	g.OrgID = strings.TrimSpace(g.OrgID)
	g.ParentID = strings.TrimSpace(g.ParentID)
	g.Name = strings.TrimSpace(g.Name)
	g.Type = strings.TrimSpace(strings.ToLower(g.Type))
	if g.Type == "" {
		g.Type = GroupTypeVirtual
	}
	return g
}

// Membership binds a user to a group. RoleIDs are optional role projections;
// resource authorization still belongs to authz/accessx.
type Membership struct {
	OrgID     string
	UserID    string
	GroupID   string
	RoleIDs   []string
	Source    string
	CreatedAt time.Time
}

func (m Membership) Normalize() Membership {
	m.OrgID = strings.TrimSpace(m.OrgID)
	m.UserID = strings.TrimSpace(m.UserID)
	m.GroupID = strings.TrimSpace(m.GroupID)
	if m.Source == "" {
		m.Source = MembershipDirect
	}
	return m
}

// GroupNode is an API-friendly tree projection for org/group management UI.
type GroupNode struct {
	Group    Group
	Children []GroupNode
}
