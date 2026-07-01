package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/aisphereio/kernel/iamx"
)

// Store is an in-memory IAM directory for tests, demos and local development.
// It supports multi-level organizations/groups and inherited group membership
// projection. Production IAM should use iamx/db.Store and optionally sync from
// Casdoor through iamx/authnadapter.
type Store struct {
	mu          sync.RWMutex
	users       map[string]iamx.User
	orgs        map[string]iamx.Organization
	groups      map[string]iamx.Group
	memberships map[string]iamx.Membership
	now         func() time.Time
}

func New() *Store {
	return &Store{users: map[string]iamx.User{}, orgs: map[string]iamx.Organization{}, groups: map[string]iamx.Group{}, memberships: map[string]iamx.Membership{}, now: time.Now}
}

func userKey(orgID, userID string) string                { return orgID + "/" + userID }
func groupKey(orgID, groupID string) string              { return orgID + "/" + groupID }
func membershipKey(orgID, groupID, userID string) string { return orgID + "/" + groupID + "/" + userID }

func (s *Store) CreateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	_ = ctx
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return iamx.User{}, iamx.ErrInvalidArgument("user org_id and id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userKey(user.OrgID, user.ID)
	if _, ok := s.users[key]; ok {
		return iamx.User{}, iamx.ErrConflict("user already exists")
	}
	now := s.now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	s.users[key] = user
	return user, nil
}

func (s *Store) GetUser(ctx context.Context, orgID, userID string) (iamx.User, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userKey(orgID, userID)]
	if !ok || user.Deleted {
		return iamx.User{}, iamx.ErrNotFound("user not found")
	}
	return user, nil
}

func (s *Store) ListUsers(ctx context.Context, q iamx.UserQuery) ([]iamx.User, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []iamx.User{}
	memberSet := map[string]bool{}
	if q.GroupID != "" {
		for _, m := range s.memberships {
			if m.OrgID == q.OrgID && m.GroupID == q.GroupID {
				memberSet[m.UserID] = true
			}
		}
	}
	for _, user := range s.users {
		if user.Deleted {
			continue
		}
		if q.OrgID != "" && user.OrgID != q.OrgID {
			continue
		}
		if q.ID != "" && user.ID != q.ID {
			continue
		}
		if q.Username != "" && user.Username != q.Username {
			continue
		}
		if q.Email != "" && user.Email != q.Email {
			continue
		}
		if q.GroupID != "" && !memberSet[user.ID] {
			continue
		}
		out = append(out, user)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return applyUserPage(out, q.Offset, q.Limit), nil
}

func (s *Store) UpdateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	_ = ctx
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return iamx.User{}, iamx.ErrInvalidArgument("user org_id and id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userKey(user.OrgID, user.ID)
	existing, ok := s.users[key]
	if !ok || existing.Deleted {
		return iamx.User{}, iamx.ErrNotFound("user not found")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = existing.CreatedAt
	}
	user.UpdatedAt = s.now()
	s.users[key] = user
	return user, nil
}

func (s *Store) UpsertUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	_ = ctx
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return iamx.User{}, iamx.ErrInvalidArgument("user org_id and id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	existing, ok := s.users[userKey(user.OrgID, user.ID)]
	if ok && user.CreatedAt.IsZero() {
		user.CreatedAt = existing.CreatedAt
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	s.users[userKey(user.OrgID, user.ID)] = user
	return user, nil
}

func (s *Store) DisableUser(ctx context.Context, orgID, userID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userKey(orgID, userID)
	user, ok := s.users[key]
	if !ok || user.Deleted {
		return iamx.ErrNotFound("user not found")
	}
	user.Enabled = false
	user.UpdatedAt = s.now()
	s.users[key] = user
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, orgID, userID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userKey(orgID, userID)
	user, ok := s.users[key]
	if !ok || user.Deleted {
		return iamx.ErrNotFound("user not found")
	}
	user.Deleted = true
	user.Enabled = false
	user.UpdatedAt = s.now()
	s.users[key] = user
	for mk, m := range s.memberships {
		if m.OrgID == orgID && m.UserID == userID {
			delete(s.memberships, mk)
		}
	}
	return nil
}

func (s *Store) CreateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	_ = ctx
	org = org.Normalize()
	if org.ID == "" {
		return iamx.Organization{}, iamx.ErrInvalidArgument("organization id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[org.ID]; ok {
		return iamx.Organization{}, iamx.ErrConflict("organization already exists")
	}
	if org.ParentID != "" {
		if _, ok := s.orgs[org.ParentID]; !ok {
			return iamx.Organization{}, iamx.ErrInvalidArgument("parent organization not found")
		}
	}
	now := s.now()
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
	}
	org.UpdatedAt = now
	s.orgs[org.ID] = org
	return org, nil
}

func (s *Store) GetOrganization(ctx context.Context, orgID string) (iamx.Organization, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	org, ok := s.orgs[orgID]
	if !ok {
		return iamx.Organization{}, iamx.ErrNotFound("organization not found")
	}
	return org, nil
}

func (s *Store) ListOrganizations(ctx context.Context, q iamx.OrganizationQuery) ([]iamx.Organization, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []iamx.Organization{}
	for _, org := range s.orgs {
		if q.ParentID != "" && org.ParentID != q.ParentID {
			continue
		}
		if q.Name != "" && org.Name != q.Name {
			continue
		}
		out = append(out, org)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return applyOrgPage(out, q.Offset, q.Limit), nil
}

func (s *Store) UpdateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	_ = ctx
	org = org.Normalize()
	if org.ID == "" {
		return iamx.Organization{}, iamx.ErrInvalidArgument("organization id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.orgs[org.ID]
	if !ok {
		return iamx.Organization{}, iamx.ErrNotFound("organization not found")
	}
	if org.ParentID != "" && org.ParentID == org.ID {
		return iamx.Organization{}, iamx.ErrInvalidArgument("organization cannot be its own parent")
	}
	if org.ParentID != "" {
		if _, ok := s.orgs[org.ParentID]; !ok {
			return iamx.Organization{}, iamx.ErrInvalidArgument("parent organization not found")
		}
	}
	if org.CreatedAt.IsZero() {
		org.CreatedAt = existing.CreatedAt
	}
	org.UpdatedAt = s.now()
	s.orgs[org.ID] = org
	return org, nil
}

func (s *Store) UpsertOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	if _, err := s.GetOrganization(ctx, org.ID); err == nil {
		return s.UpdateOrganization(ctx, org)
	}
	return s.CreateOrganization(ctx, org)
}

func (s *Store) DeleteOrganization(ctx context.Context, orgID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[orgID]; !ok {
		return iamx.ErrNotFound("organization not found")
	}
	for _, org := range s.orgs {
		if org.ParentID == orgID {
			return iamx.ErrConflict("organization has children")
		}
	}
	for _, user := range s.users {
		if user.OrgID == orgID && !user.Deleted {
			return iamx.ErrConflict("organization has users")
		}
	}
	for _, group := range s.groups {
		if group.OrgID == orgID {
			return iamx.ErrConflict("organization has groups")
		}
	}
	delete(s.orgs, orgID)
	return nil
}

func (s *Store) CreateGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	_ = ctx
	group = group.Normalize()
	if group.OrgID == "" || group.ID == "" {
		return iamx.Group{}, iamx.ErrInvalidArgument("group org_id and id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createGroupLocked(group)
}

func (s *Store) createGroupLocked(group iamx.Group) (iamx.Group, error) {
	if _, ok := s.orgs[group.OrgID]; !ok {
		return iamx.Group{}, iamx.ErrInvalidArgument("organization not found")
	}
	if _, ok := s.groups[groupKey(group.OrgID, group.ID)]; ok {
		return iamx.Group{}, iamx.ErrConflict("group already exists")
	}
	if group.ParentID != "" {
		if _, ok := s.groups[groupKey(group.OrgID, group.ParentID)]; !ok {
			return iamx.Group{}, iamx.ErrInvalidArgument("parent group not found")
		}
	}
	now := s.now()
	if group.CreatedAt.IsZero() {
		group.CreatedAt = now
	}
	group.UpdatedAt = now
	group.Path = s.buildGroupPathLocked(group)
	s.groups[groupKey(group.OrgID, group.ID)] = group
	return group, nil
}

func (s *Store) GetGroup(ctx context.Context, orgID, groupID string) (iamx.Group, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.groups[groupKey(orgID, groupID)]
	if !ok {
		return iamx.Group{}, iamx.ErrNotFound("group not found")
	}
	return group, nil
}

func (s *Store) ListGroups(ctx context.Context, q iamx.GroupQuery) ([]iamx.Group, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []iamx.Group{}
	memberGroups := map[string]bool{}
	if q.UserID != "" {
		for _, m := range s.memberships {
			if m.OrgID == q.OrgID && m.UserID == q.UserID {
				memberGroups[m.GroupID] = true
			}
		}
	}
	for _, group := range s.groups {
		if q.OrgID != "" && group.OrgID != q.OrgID {
			continue
		}
		if q.ParentID != "" && group.ParentID != q.ParentID {
			continue
		}
		if q.Type != "" && group.Type != q.Type {
			continue
		}
		if q.UserID != "" && !memberGroups[group.ID] {
			continue
		}
		out = append(out, group)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return applyGroupPage(out, q.Offset, q.Limit), nil
}

func (s *Store) UpdateGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	_ = ctx
	group = group.Normalize()
	if group.OrgID == "" || group.ID == "" {
		return iamx.Group{}, iamx.ErrInvalidArgument("group org_id and id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := groupKey(group.OrgID, group.ID)
	existing, ok := s.groups[key]
	if !ok {
		return iamx.Group{}, iamx.ErrNotFound("group not found")
	}
	if group.ParentID == group.ID {
		return iamx.Group{}, iamx.ErrInvalidArgument("group cannot be its own parent")
	}
	if group.ParentID != "" {
		if _, ok := s.groups[groupKey(group.OrgID, group.ParentID)]; !ok {
			return iamx.Group{}, iamx.ErrInvalidArgument("parent group not found")
		}
	}
	if group.ParentID != existing.ParentID && s.isDescendantLocked(group.OrgID, group.ParentID, group.ID) {
		return iamx.Group{}, iamx.ErrInvalidArgument("group parent would create a cycle")
	}
	if group.CreatedAt.IsZero() {
		group.CreatedAt = existing.CreatedAt
	}
	group.UpdatedAt = s.now()
	group.Path = s.buildGroupPathLocked(group)
	s.groups[key] = group
	s.rebuildChildPathsLocked(group.OrgID, group.ID)
	return group, nil
}

func (s *Store) UpsertGroup(ctx context.Context, group iamx.Group) (iamx.Group, error) {
	if _, err := s.GetGroup(ctx, group.OrgID, group.ID); err == nil {
		return s.UpdateGroup(ctx, group)
	}
	return s.CreateGroup(ctx, group)
}

func (s *Store) DeleteGroup(ctx context.Context, orgID, groupID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	key := groupKey(orgID, groupID)
	if _, ok := s.groups[key]; !ok {
		return iamx.ErrNotFound("group not found")
	}
	for _, group := range s.groups {
		if group.OrgID == orgID && group.ParentID == groupID {
			return iamx.ErrConflict("group has children")
		}
	}
	delete(s.groups, key)
	for key, m := range s.memberships {
		if m.OrgID == orgID && m.GroupID == groupID {
			delete(s.memberships, key)
		}
	}
	return nil
}

func (s *Store) ListGroupAncestors(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.groups[groupKey(orgID, groupID)]
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
		parent, ok := s.groups[groupKey(orgID, group.ParentID)]
		if !ok {
			return nil, iamx.ErrInvalidArgument("parent group not found")
		}
		out = append(out, parent)
		group = parent
	}
	return out, nil
}

func (s *Store) ListGroupDescendants(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.groups[groupKey(orgID, groupID)]; !ok {
		return nil, iamx.ErrNotFound("group not found")
	}
	out := []iamx.Group{}
	var walk func(parent string)
	walk = func(parent string) {
		children := []iamx.Group{}
		for _, group := range s.groups {
			if group.OrgID == orgID && group.ParentID == parent {
				children = append(children, group)
			}
		}
		sort.Slice(children, func(i, j int) bool { return children[i].ID < children[j].ID })
		for _, child := range children {
			out = append(out, child)
			walk(child.ID)
		}
	}
	walk(groupID)
	return out, nil
}

func (s *Store) AddMembership(ctx context.Context, m iamx.Membership) error {
	_ = ctx
	m = m.Normalize()
	if m.OrgID == "" || m.GroupID == "" || m.UserID == "" {
		return iamx.ErrInvalidArgument("membership org_id, group_id and user_id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userKey(m.OrgID, m.UserID)]; !ok {
		return iamx.ErrNotFound("user not found")
	}
	if _, ok := s.groups[groupKey(m.OrgID, m.GroupID)]; !ok {
		return iamx.ErrNotFound("group not found")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = s.now()
	}
	s.memberships[membershipKey(m.OrgID, m.GroupID, m.UserID)] = m
	return nil
}

func (s *Store) RemoveMembership(ctx context.Context, orgID, groupID, userID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memberships, membershipKey(orgID, groupID, userID))
	return nil
}

func (s *Store) ListMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []iamx.Membership{}
	for _, m := range s.memberships {
		if orgID != "" && m.OrgID != orgID {
			continue
		}
		if userID != "" && m.UserID != userID {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func (s *Store) ListEffectiveMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	direct, err := s.ListMemberships(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]iamx.Membership, 0, len(direct))
	seen := map[string]bool{}
	for _, m := range direct {
		out = append(out, m)
		seen[m.GroupID] = true
		group, ok := s.groups[groupKey(m.OrgID, m.GroupID)]
		if !ok {
			continue
		}
		for group.ParentID != "" {
			parent, ok := s.groups[groupKey(m.OrgID, group.ParentID)]
			if !ok {
				break
			}
			if !seen[parent.ID] {
				out = append(out, iamx.Membership{OrgID: m.OrgID, UserID: m.UserID, GroupID: parent.ID, Source: iamx.MembershipInherited, CreatedAt: m.CreatedAt})
				seen[parent.ID] = true
			}
			group = parent
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func (s *Store) buildGroupPathLocked(group iamx.Group) string {
	if group.ParentID == "" {
		return "/" + group.ID
	}
	parent, ok := s.groups[groupKey(group.OrgID, group.ParentID)]
	if !ok || parent.Path == "" {
		return "/" + group.ParentID + "/" + group.ID
	}
	return parent.Path + "/" + group.ID
}

func (s *Store) rebuildChildPathsLocked(orgID, parentID string) {
	for key, child := range s.groups {
		if child.OrgID == orgID && child.ParentID == parentID {
			child.Path = s.buildGroupPathLocked(child)
			s.groups[key] = child
			s.rebuildChildPathsLocked(orgID, child.ID)
		}
	}
}

func (s *Store) isDescendantLocked(orgID, maybeChildID, parentID string) bool {
	if maybeChildID == "" {
		return false
	}
	cur, ok := s.groups[groupKey(orgID, maybeChildID)]
	for ok {
		if cur.ParentID == parentID {
			return true
		}
		if cur.ParentID == "" {
			return false
		}
		cur, ok = s.groups[groupKey(orgID, cur.ParentID)]
	}
	return false
}

func applyUserPage(in []iamx.User, offset, limit int) []iamx.User {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []iamx.User{}
	}
	if limit <= 0 || offset+limit > len(in) {
		return in[offset:]
	}
	return in[offset : offset+limit]
}
func applyOrgPage(in []iamx.Organization, offset, limit int) []iamx.Organization {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []iamx.Organization{}
	}
	if limit <= 0 || offset+limit > len(in) {
		return in[offset:]
	}
	return in[offset : offset+limit]
}
func applyGroupPage(in []iamx.Group, offset, limit int) []iamx.Group {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []iamx.Group{}
	}
	if limit <= 0 || offset+limit > len(in) {
		return in[offset:]
	}
	return in[offset : offset+limit]
}
