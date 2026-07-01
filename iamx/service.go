package iamx

import "context"

type UserQuery struct {
	OrgID    string
	ID       string
	Username string
	Email    string
	GroupID  string
	Limit    int
	Offset   int
}

type OrganizationQuery struct {
	ParentID string
	Name     string
	Limit    int
	Offset   int
}

type GroupQuery struct {
	OrgID    string
	ParentID string
	Type     string
	UserID   string
	Limit    int
	Offset   int
}

// UserGroupQuery controls user -> group projection queries.
type UserGroupQuery struct {
	OrgID            string
	UserID           string
	IncludeInherited bool
	Limit            int
	Offset           int
}

// Directory is the IAM read/write surface for users, orgs, groups and memberships.
// It is intentionally provider-neutral. Casdoor, DB-backed IAM and hybrid sync
// implementations must adapt to this interface.
type Directory interface {
	CreateUser(ctx context.Context, user User) (User, error)
	GetUser(ctx context.Context, orgID, userID string) (User, error)
	ListUsers(ctx context.Context, query UserQuery) ([]User, error)
	UpdateUser(ctx context.Context, user User) (User, error)
	UpsertUser(ctx context.Context, user User) (User, error)
	DisableUser(ctx context.Context, orgID, userID string) error
	DeleteUser(ctx context.Context, orgID, userID string) error

	CreateOrganization(ctx context.Context, org Organization) (Organization, error)
	GetOrganization(ctx context.Context, orgID string) (Organization, error)
	ListOrganizations(ctx context.Context, query OrganizationQuery) ([]Organization, error)
	UpdateOrganization(ctx context.Context, org Organization) (Organization, error)
	UpsertOrganization(ctx context.Context, org Organization) (Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error

	CreateGroup(ctx context.Context, group Group) (Group, error)
	GetGroup(ctx context.Context, orgID, groupID string) (Group, error)
	ListGroups(ctx context.Context, query GroupQuery) ([]Group, error)
	UpdateGroup(ctx context.Context, group Group) (Group, error)
	UpsertGroup(ctx context.Context, group Group) (Group, error)
	DeleteGroup(ctx context.Context, orgID, groupID string) error
	ListGroupAncestors(ctx context.Context, orgID, groupID string) ([]Group, error)
	ListGroupDescendants(ctx context.Context, orgID, groupID string) ([]Group, error)

	AddMembership(ctx context.Context, membership Membership) error
	RemoveMembership(ctx context.Context, orgID, groupID, userID string) error
	ListMemberships(ctx context.Context, orgID, userID string) ([]Membership, error)
	ListEffectiveMemberships(ctx context.Context, orgID, userID string) ([]Membership, error)
}

// Service is Kernel IAM's application-facing facade. Business components should
// depend on this facade rather than calling authn/casdoor or provider SDKs.
type Service struct{ directory Directory }

func NewService(directory Directory) (*Service, error) {
	if directory == nil {
		return nil, ErrInvalidArgument("iam directory is required")
	}
	return &Service{directory: directory}, nil
}

func (s *Service) Directory() Directory { return s.directory }

func (s *Service) CreateUser(ctx context.Context, user User) (User, error) {
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return User{}, ErrInvalidArgument("user org_id and id are required")
	}
	return s.directory.CreateUser(ctx, user)
}

func (s *Service) GetUser(ctx context.Context, orgID, userID string) (User, error) {
	return s.directory.GetUser(ctx, orgID, userID)
}

func (s *Service) ListUsers(ctx context.Context, query UserQuery) ([]User, error) {
	return s.directory.ListUsers(ctx, query)
}

func (s *Service) UpdateUser(ctx context.Context, user User) (User, error) {
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return User{}, ErrInvalidArgument("user org_id and id are required")
	}
	return s.directory.UpdateUser(ctx, user)
}

func (s *Service) EnsureUser(ctx context.Context, user User) (User, error) {
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return User{}, ErrInvalidArgument("user org_id and id are required")
	}
	return s.directory.UpsertUser(ctx, user)
}

func (s *Service) DisableUser(ctx context.Context, orgID, userID string) error {
	return s.directory.DisableUser(ctx, orgID, userID)
}

func (s *Service) DeleteUser(ctx context.Context, orgID, userID string) error {
	return s.directory.DeleteUser(ctx, orgID, userID)
}

func (s *Service) CreateOrganization(ctx context.Context, org Organization) (Organization, error) {
	org = org.Normalize()
	if org.ID == "" {
		return Organization{}, ErrInvalidArgument("organization id is required")
	}
	return s.directory.CreateOrganization(ctx, org)
}

func (s *Service) GetOrganization(ctx context.Context, orgID string) (Organization, error) {
	return s.directory.GetOrganization(ctx, orgID)
}

func (s *Service) ListOrganizations(ctx context.Context, query OrganizationQuery) ([]Organization, error) {
	return s.directory.ListOrganizations(ctx, query)
}

func (s *Service) UpdateOrganization(ctx context.Context, org Organization) (Organization, error) {
	org = org.Normalize()
	if org.ID == "" {
		return Organization{}, ErrInvalidArgument("organization id is required")
	}
	return s.directory.UpdateOrganization(ctx, org)
}

func (s *Service) EnsureOrganization(ctx context.Context, org Organization) (Organization, error) {
	org = org.Normalize()
	if org.ID == "" {
		return Organization{}, ErrInvalidArgument("organization id is required")
	}
	return s.directory.UpsertOrganization(ctx, org)
}

func (s *Service) DeleteOrganization(ctx context.Context, orgID string) error {
	return s.directory.DeleteOrganization(ctx, orgID)
}

func (s *Service) CreateGroup(ctx context.Context, group Group) (Group, error) {
	group = group.Normalize()
	if group.OrgID == "" || group.ID == "" {
		return Group{}, ErrInvalidArgument("group org_id and id are required")
	}
	return s.directory.CreateGroup(ctx, group)
}

func (s *Service) GetGroup(ctx context.Context, orgID, groupID string) (Group, error) {
	return s.directory.GetGroup(ctx, orgID, groupID)
}

func (s *Service) ListGroups(ctx context.Context, query GroupQuery) ([]Group, error) {
	return s.directory.ListGroups(ctx, query)
}

func (s *Service) UpdateGroup(ctx context.Context, group Group) (Group, error) {
	group = group.Normalize()
	if group.OrgID == "" || group.ID == "" {
		return Group{}, ErrInvalidArgument("group org_id and id are required")
	}
	return s.directory.UpdateGroup(ctx, group)
}

func (s *Service) EnsureGroup(ctx context.Context, group Group) (Group, error) {
	group = group.Normalize()
	if group.OrgID == "" || group.ID == "" {
		return Group{}, ErrInvalidArgument("group org_id and id are required")
	}
	return s.directory.UpsertGroup(ctx, group)
}

func (s *Service) DeleteGroup(ctx context.Context, orgID, groupID string) error {
	return s.directory.DeleteGroup(ctx, orgID, groupID)
}

func (s *Service) AddMembership(ctx context.Context, membership Membership) error {
	return s.directory.AddMembership(ctx, membership)
}

func (s *Service) RemoveMembership(ctx context.Context, orgID, groupID, userID string) error {
	return s.directory.RemoveMembership(ctx, orgID, groupID, userID)
}

func (s *Service) ListMemberships(ctx context.Context, orgID, userID string) ([]Membership, error) {
	return s.directory.ListMemberships(ctx, orgID, userID)
}

func (s *Service) ListEffectiveMemberships(ctx context.Context, orgID, userID string) ([]Membership, error) {
	return s.directory.ListEffectiveMemberships(ctx, orgID, userID)
}

func (s *Service) ListUserGroups(ctx context.Context, q UserGroupQuery) ([]Group, error) {
	memberships, err := s.directory.ListMemberships(ctx, q.OrgID, q.UserID)
	if err != nil {
		return nil, err
	}
	if q.IncludeInherited {
		memberships, err = s.directory.ListEffectiveMemberships(ctx, q.OrgID, q.UserID)
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	groups := make([]Group, 0, len(memberships))
	for _, m := range memberships {
		if seen[m.GroupID] {
			continue
		}
		seen[m.GroupID] = true
		group, err := s.directory.GetGroup(ctx, m.OrgID, m.GroupID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return applyGroupPage(groups, q.Offset, q.Limit), nil
}

func (s *Service) ListGroupAncestors(ctx context.Context, orgID, groupID string) ([]Group, error) {
	return s.directory.ListGroupAncestors(ctx, orgID, groupID)
}

func (s *Service) ListGroupDescendants(ctx context.Context, orgID, groupID string) ([]Group, error) {
	return s.directory.ListGroupDescendants(ctx, orgID, groupID)
}

func (s *Service) BuildGroupTree(ctx context.Context, orgID string) ([]GroupNode, error) {
	groups, err := s.directory.ListGroups(ctx, GroupQuery{OrgID: orgID})
	if err != nil {
		return nil, err
	}
	byParent := map[string][]Group{}
	for _, group := range groups {
		group = group.Normalize()
		byParent[group.ParentID] = append(byParent[group.ParentID], group)
	}
	var build func(parent string) []GroupNode
	build = func(parent string) []GroupNode {
		children := byParent[parent]
		nodes := make([]GroupNode, 0, len(children))
		for _, group := range children {
			nodes = append(nodes, GroupNode{Group: group, Children: build(group.ID)})
		}
		return nodes
	}
	return build(""), nil
}

func applyGroupPage(in []Group, offset, limit int) []Group {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []Group{}
	}
	if limit <= 0 || offset+limit > len(in) {
		return in[offset:]
	}
	return in[offset : offset+limit]
}
