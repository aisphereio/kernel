package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aisphereio/kernel/iamx"
)

type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
)

type Option func(*Store)

func WithDialect(dialect Dialect) Option { return func(s *Store) { s.dialect = dialect } }
func WithNow(now func() time.Time) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

// Store is a database/sql backed IAM directory. It intentionally does not expose
// Casdoor object structs or provider SDKs to business code. Wire it from dbx by
// using dbx.DB.GORM(ctx).DB() in boot/server code, but keep business code on
// iamx.Service.
type Store struct {
	db      *sql.DB
	dialect Dialect
	now     func() time.Time
}

func New(database *sql.DB, opts ...Option) (*Store, error) {
	if database == nil {
		return nil, iamx.ErrInvalidArgument("iam sql db is required")
	}
	s := &Store{db: database, dialect: DialectPostgres, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

func (s *Store) AutoMigrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS kernel_iam_organizations (id VARCHAR(128) PRIMARY KEY, external_id VARCHAR(256), provider VARCHAR(64), parent_id VARCHAR(128), name VARCHAR(128), display_name VARCHAR(256), owner_id VARCHAR(128), enabled BOOLEAN, attributes TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS kernel_iam_users (org_id VARCHAR(128), id VARCHAR(128), external_id VARCHAR(256), provider VARCHAR(64), username VARCHAR(128), name VARCHAR(256), email VARCHAR(256), phone VARCHAR(64), enabled BOOLEAN, locked BOOLEAN, deleted BOOLEAN, attributes TEXT, created_at TIMESTAMP, updated_at TIMESTAMP, PRIMARY KEY (org_id, id))`,
		`CREATE TABLE IF NOT EXISTS kernel_iam_groups (org_id VARCHAR(128), id VARCHAR(128), external_id VARCHAR(256), provider VARCHAR(64), parent_id VARCHAR(128), name VARCHAR(128), display_name VARCHAR(256), type VARCHAR(64), path VARCHAR(1024), enabled BOOLEAN, attributes TEXT, created_at TIMESTAMP, updated_at TIMESTAMP, PRIMARY KEY (org_id, id))`,
		`CREATE TABLE IF NOT EXISTS kernel_iam_memberships (org_id VARCHAR(128), group_id VARCHAR(128), user_id VARCHAR(128), role_ids TEXT, source VARCHAR(64), created_at TIMESTAMP, PRIMARY KEY (org_id, group_id, user_id))`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	user = user.Normalize()
	if user.OrgID == "" || user.ID == "" {
		return iamx.User{}, iamx.ErrInvalidArgument("user org_id and id are required")
	}
	now := s.now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	m := toUserModel(user)
	_, err := s.db.ExecContext(ctx, s.bind(`INSERT INTO kernel_iam_users (org_id,id,external_id,provider,username,name,email,phone,enabled,locked,deleted,attributes,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), m.OrgID, m.ID, m.ExternalID, m.Provider, m.Username, m.Name, m.Email, m.Phone, m.Enabled, m.Locked, m.Deleted, m.Attributes, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return iamx.User{}, normalizeErr(err, "user")
	}
	return user, nil
}
func (s *Store) GetUser(ctx context.Context, orgID, userID string) (iamx.User, error) {
	var m userModel
	err := s.db.QueryRowContext(ctx, s.bind(`SELECT org_id,id,external_id,provider,username,name,email,phone,enabled,locked,deleted,attributes,created_at,updated_at FROM kernel_iam_users WHERE org_id=? AND id=? AND deleted=?`), orgID, userID, false).Scan(&m.OrgID, &m.ID, &m.ExternalID, &m.Provider, &m.Username, &m.Name, &m.Email, &m.Phone, &m.Enabled, &m.Locked, &m.Deleted, &m.Attributes, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return iamx.User{}, normalizeErr(err, "user")
	}
	return fromUserModel(m), nil
}
func (s *Store) ListUsers(ctx context.Context, q iamx.UserQuery) ([]iamx.User, error) {
	where, args := []string{"deleted=?"}, []any{false}
	if q.OrgID != "" {
		where = append(where, "org_id=?")
		args = append(args, q.OrgID)
	}
	if q.ID != "" {
		where = append(where, "id=?")
		args = append(args, q.ID)
	}
	if q.Username != "" {
		where = append(where, "username=?")
		args = append(args, q.Username)
	}
	if q.Email != "" {
		where = append(where, "email=?")
		args = append(args, q.Email)
	}
	join := ""
	if q.GroupID != "" {
		join = " JOIN kernel_iam_memberships m ON m.org_id=kernel_iam_users.org_id AND m.user_id=kernel_iam_users.id"
		where = append(where, "m.group_id=?")
		args = append(args, q.GroupID)
	}
	query := `SELECT org_id,id,external_id,provider,username,name,email,phone,enabled,locked,deleted,attributes,created_at,updated_at FROM kernel_iam_users` + join + ` WHERE ` + strings.Join(where, " AND ") + ` ORDER BY org_id,id` + pageSQL(q.Offset, q.Limit)
	rows, err := s.db.QueryContext(ctx, s.bind(query), args...)
	if err != nil {
		return nil, normalizeErr(err, "user")
	}
	defer rows.Close()
	out := []iamx.User{}
	for rows.Next() {
		var m userModel
		if err := rows.Scan(&m.OrgID, &m.ID, &m.ExternalID, &m.Provider, &m.Username, &m.Name, &m.Email, &m.Phone, &m.Enabled, &m.Locked, &m.Deleted, &m.Attributes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, fromUserModel(m))
	}
	return out, rows.Err()
}
func (s *Store) UpdateUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	old, err := s.GetUser(ctx, user.OrgID, user.ID)
	if err != nil {
		return iamx.User{}, err
	}
	user = user.Normalize()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = old.CreatedAt
	}
	user.UpdatedAt = s.now()
	m := toUserModel(user)
	res, err := s.db.ExecContext(ctx, s.bind(`UPDATE kernel_iam_users SET external_id=?,provider=?,username=?,name=?,email=?,phone=?,enabled=?,locked=?,deleted=?,attributes=?,created_at=?,updated_at=? WHERE org_id=? AND id=? AND deleted=?`), m.ExternalID, m.Provider, m.Username, m.Name, m.Email, m.Phone, m.Enabled, m.Locked, m.Deleted, m.Attributes, m.CreatedAt, m.UpdatedAt, m.OrgID, m.ID, false)
	if err != nil {
		return iamx.User{}, normalizeErr(err, "user")
	}
	return user, checkRows(res, "user")
}
func (s *Store) UpsertUser(ctx context.Context, user iamx.User) (iamx.User, error) {
	if _, err := s.GetUser(ctx, user.OrgID, user.ID); err == nil {
		return s.UpdateUser(ctx, user)
	}
	return s.CreateUser(ctx, user)
}
func (s *Store) DisableUser(ctx context.Context, orgID, userID string) error {
	res, err := s.db.ExecContext(ctx, s.bind(`UPDATE kernel_iam_users SET enabled=?,updated_at=? WHERE org_id=? AND id=? AND deleted=?`), false, s.now(), orgID, userID, false)
	if err != nil {
		return normalizeErr(err, "user")
	}
	return checkRows(res, "user")
}
func (s *Store) DeleteUser(ctx context.Context, orgID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, s.bind(`UPDATE kernel_iam_users SET enabled=?,deleted=?,updated_at=? WHERE org_id=? AND id=? AND deleted=?`), false, true, s.now(), orgID, userID, false)
	if err != nil {
		return normalizeErr(err, "user")
	}
	if err := checkRows(res, "user"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.bind(`DELETE FROM kernel_iam_memberships WHERE org_id=? AND user_id=?`), orgID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	org = org.Normalize()
	if org.ID == "" {
		return iamx.Organization{}, iamx.ErrInvalidArgument("organization id is required")
	}
	if org.ParentID != "" {
		if _, err := s.GetOrganization(ctx, org.ParentID); err != nil {
			return iamx.Organization{}, iamx.ErrInvalidArgument("parent organization not found")
		}
	}
	now := s.now()
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
	}
	org.UpdatedAt = now
	m := toOrgModel(org)
	_, err := s.db.ExecContext(ctx, s.bind(`INSERT INTO kernel_iam_organizations (id,external_id,provider,parent_id,name,display_name,owner_id,enabled,attributes,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`), m.ID, m.ExternalID, m.Provider, m.ParentID, m.Name, m.DisplayName, m.OwnerID, m.Enabled, m.Attributes, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return iamx.Organization{}, normalizeErr(err, "organization")
	}
	return org, nil
}
func (s *Store) GetOrganization(ctx context.Context, orgID string) (iamx.Organization, error) {
	var m orgModel
	err := s.db.QueryRowContext(ctx, s.bind(`SELECT id,external_id,provider,parent_id,name,display_name,owner_id,enabled,attributes,created_at,updated_at FROM kernel_iam_organizations WHERE id=?`), orgID).Scan(&m.ID, &m.ExternalID, &m.Provider, &m.ParentID, &m.Name, &m.DisplayName, &m.OwnerID, &m.Enabled, &m.Attributes, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return iamx.Organization{}, normalizeErr(err, "organization")
	}
	return fromOrgModel(m), nil
}
func (s *Store) ListOrganizations(ctx context.Context, q iamx.OrganizationQuery) ([]iamx.Organization, error) {
	where, args := []string{"1=1"}, []any{}
	if q.ParentID != "" {
		where = append(where, "parent_id=?")
		args = append(args, q.ParentID)
	}
	if q.Name != "" {
		where = append(where, "name=?")
		args = append(args, q.Name)
	}
	rows, err := s.db.QueryContext(ctx, s.bind(`SELECT id,external_id,provider,parent_id,name,display_name,owner_id,enabled,attributes,created_at,updated_at FROM kernel_iam_organizations WHERE `+strings.Join(where, " AND ")+` ORDER BY id`+pageSQL(q.Offset, q.Limit)), args...)
	if err != nil {
		return nil, normalizeErr(err, "organization")
	}
	defer rows.Close()
	out := []iamx.Organization{}
	for rows.Next() {
		var m orgModel
		if err := rows.Scan(&m.ID, &m.ExternalID, &m.Provider, &m.ParentID, &m.Name, &m.DisplayName, &m.OwnerID, &m.Enabled, &m.Attributes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, fromOrgModel(m))
	}
	return out, rows.Err()
}
func (s *Store) UpdateOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	old, err := s.GetOrganization(ctx, org.ID)
	if err != nil {
		return iamx.Organization{}, err
	}
	org = org.Normalize()
	if org.ParentID == org.ID {
		return iamx.Organization{}, iamx.ErrInvalidArgument("organization cannot be its own parent")
	}
	if org.ParentID != "" {
		if _, err := s.GetOrganization(ctx, org.ParentID); err != nil {
			return iamx.Organization{}, iamx.ErrInvalidArgument("parent organization not found")
		}
	}
	if org.CreatedAt.IsZero() {
		org.CreatedAt = old.CreatedAt
	}
	org.UpdatedAt = s.now()
	m := toOrgModel(org)
	res, err := s.db.ExecContext(ctx, s.bind(`UPDATE kernel_iam_organizations SET external_id=?,provider=?,parent_id=?,name=?,display_name=?,owner_id=?,enabled=?,attributes=?,created_at=?,updated_at=? WHERE id=?`), m.ExternalID, m.Provider, m.ParentID, m.Name, m.DisplayName, m.OwnerID, m.Enabled, m.Attributes, m.CreatedAt, m.UpdatedAt, m.ID)
	if err != nil {
		return iamx.Organization{}, normalizeErr(err, "organization")
	}
	return org, checkRows(res, "organization")
}
func (s *Store) UpsertOrganization(ctx context.Context, org iamx.Organization) (iamx.Organization, error) {
	if _, err := s.GetOrganization(ctx, org.ID); err == nil {
		return s.UpdateOrganization(ctx, org)
	}
	return s.CreateOrganization(ctx, org)
}
func (s *Store) DeleteOrganization(ctx context.Context, orgID string) error {
	if children, _ := s.ListOrganizations(ctx, iamx.OrganizationQuery{ParentID: orgID, Limit: 1}); len(children) > 0 {
		return iamx.ErrConflict("organization has children")
	}
	if users, _ := s.ListUsers(ctx, iamx.UserQuery{OrgID: orgID, Limit: 1}); len(users) > 0 {
		return iamx.ErrConflict("organization has users")
	}
	if groups, _ := s.ListGroups(ctx, iamx.GroupQuery{OrgID: orgID, Limit: 1}); len(groups) > 0 {
		return iamx.ErrConflict("organization has groups")
	}
	res, err := s.db.ExecContext(ctx, s.bind(`DELETE FROM kernel_iam_organizations WHERE id=?`), orgID)
	if err != nil {
		return normalizeErr(err, "organization")
	}
	return checkRows(res, "organization")
}

func (s *Store) CreateGroup(ctx context.Context, g iamx.Group) (iamx.Group, error) {
	g = g.Normalize()
	if g.OrgID == "" || g.ID == "" {
		return iamx.Group{}, iamx.ErrInvalidArgument("group org_id and id are required")
	}
	if _, err := s.GetOrganization(ctx, g.OrgID); err != nil {
		return iamx.Group{}, iamx.ErrInvalidArgument("organization not found")
	}
	if g.ParentID != "" {
		if _, err := s.GetGroup(ctx, g.OrgID, g.ParentID); err != nil {
			return iamx.Group{}, iamx.ErrInvalidArgument("parent group not found")
		}
	}
	now := s.now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	g.Path = s.buildGroupPath(ctx, g)
	m := toGroupModel(g)
	_, err := s.db.ExecContext(ctx, s.bind(`INSERT INTO kernel_iam_groups (org_id,id,external_id,provider,parent_id,name,display_name,type,path,enabled,attributes,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`), m.OrgID, m.ID, m.ExternalID, m.Provider, m.ParentID, m.Name, m.DisplayName, m.Type, m.Path, m.Enabled, m.Attributes, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return iamx.Group{}, normalizeErr(err, "group")
	}
	return g, nil
}
func (s *Store) GetGroup(ctx context.Context, orgID, groupID string) (iamx.Group, error) {
	var m groupModel
	err := s.db.QueryRowContext(ctx, s.bind(`SELECT org_id,id,external_id,provider,parent_id,name,display_name,type,path,enabled,attributes,created_at,updated_at FROM kernel_iam_groups WHERE org_id=? AND id=?`), orgID, groupID).Scan(&m.OrgID, &m.ID, &m.ExternalID, &m.Provider, &m.ParentID, &m.Name, &m.DisplayName, &m.Type, &m.Path, &m.Enabled, &m.Attributes, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return iamx.Group{}, normalizeErr(err, "group")
	}
	return fromGroupModel(m), nil
}
func (s *Store) ListGroups(ctx context.Context, q iamx.GroupQuery) ([]iamx.Group, error) {
	where, args := []string{"1=1"}, []any{}
	join := ""
	if q.OrgID != "" {
		where = append(where, "kernel_iam_groups.org_id=?")
		args = append(args, q.OrgID)
	}
	if q.ParentID != "" {
		where = append(where, "parent_id=?")
		args = append(args, q.ParentID)
	}
	if q.Type != "" {
		where = append(where, "type=?")
		args = append(args, q.Type)
	}
	if q.UserID != "" {
		join = " JOIN kernel_iam_memberships m ON m.org_id=kernel_iam_groups.org_id AND m.group_id=kernel_iam_groups.id"
		where = append(where, "m.user_id=?")
		args = append(args, q.UserID)
	}
	rows, err := s.db.QueryContext(ctx, s.bind(`SELECT kernel_iam_groups.org_id,kernel_iam_groups.id,external_id,provider,parent_id,name,display_name,type,path,enabled,attributes,created_at,updated_at FROM kernel_iam_groups`+join+` WHERE `+strings.Join(where, " AND ")+` ORDER BY path,id`+pageSQL(q.Offset, q.Limit)), args...)
	if err != nil {
		return nil, normalizeErr(err, "group")
	}
	defer rows.Close()
	out := []iamx.Group{}
	for rows.Next() {
		var m groupModel
		if err := rows.Scan(&m.OrgID, &m.ID, &m.ExternalID, &m.Provider, &m.ParentID, &m.Name, &m.DisplayName, &m.Type, &m.Path, &m.Enabled, &m.Attributes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, fromGroupModel(m))
	}
	return out, rows.Err()
}
func (s *Store) UpdateGroup(ctx context.Context, g iamx.Group) (iamx.Group, error) {
	old, err := s.GetGroup(ctx, g.OrgID, g.ID)
	if err != nil {
		return iamx.Group{}, err
	}
	g = g.Normalize()
	if g.ParentID == g.ID {
		return iamx.Group{}, iamx.ErrInvalidArgument("group cannot be its own parent")
	}
	if g.ParentID != "" {
		if _, err := s.GetGroup(ctx, g.OrgID, g.ParentID); err != nil {
			return iamx.Group{}, iamx.ErrInvalidArgument("parent group not found")
		}
	}
	if g.ParentID != old.ParentID {
		desc, err := s.ListGroupDescendants(ctx, g.OrgID, g.ID)
		if err != nil {
			return iamx.Group{}, err
		}
		for _, d := range desc {
			if d.ID == g.ParentID {
				return iamx.Group{}, iamx.ErrInvalidArgument("group parent would create a cycle")
			}
		}
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = old.CreatedAt
	}
	g.UpdatedAt = s.now()
	g.Path = s.buildGroupPath(ctx, g)
	m := toGroupModel(g)
	res, err := s.db.ExecContext(ctx, s.bind(`UPDATE kernel_iam_groups SET external_id=?,provider=?,parent_id=?,name=?,display_name=?,type=?,path=?,enabled=?,attributes=?,created_at=?,updated_at=? WHERE org_id=? AND id=?`), m.ExternalID, m.Provider, m.ParentID, m.Name, m.DisplayName, m.Type, m.Path, m.Enabled, m.Attributes, m.CreatedAt, m.UpdatedAt, m.OrgID, m.ID)
	if err != nil {
		return iamx.Group{}, normalizeErr(err, "group")
	}
	if err := checkRows(res, "group"); err != nil {
		return iamx.Group{}, err
	}
	_ = s.rebuildChildPaths(ctx, g.OrgID, g.ID)
	return g, nil
}
func (s *Store) UpsertGroup(ctx context.Context, g iamx.Group) (iamx.Group, error) {
	if _, err := s.GetGroup(ctx, g.OrgID, g.ID); err == nil {
		return s.UpdateGroup(ctx, g)
	}
	return s.CreateGroup(ctx, g)
}
func (s *Store) DeleteGroup(ctx context.Context, orgID, groupID string) error {
	desc, err := s.ListGroupDescendants(ctx, orgID, groupID)
	if err != nil {
		return err
	}
	if len(desc) > 0 {
		return iamx.ErrConflict("group has children")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, s.bind(`DELETE FROM kernel_iam_groups WHERE org_id=? AND id=?`), orgID, groupID)
	if err != nil {
		return normalizeErr(err, "group")
	}
	if err := checkRows(res, "group"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.bind(`DELETE FROM kernel_iam_memberships WHERE org_id=? AND group_id=?`), orgID, groupID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ListGroupAncestors(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	g, err := s.GetGroup(ctx, orgID, groupID)
	if err != nil {
		return nil, err
	}
	out := []iamx.Group{}
	seen := map[string]bool{}
	for g.ParentID != "" {
		if seen[g.ParentID] {
			return nil, iamx.ErrInvalidArgument("group cycle detected")
		}
		seen[g.ParentID] = true
		p, err := s.GetGroup(ctx, orgID, g.ParentID)
		if err != nil {
			return nil, iamx.ErrInvalidArgument("parent group not found")
		}
		out = append(out, p)
		g = p
	}
	return out, nil
}
func (s *Store) ListGroupDescendants(ctx context.Context, orgID, groupID string) ([]iamx.Group, error) {
	if _, err := s.GetGroup(ctx, orgID, groupID); err != nil {
		return nil, err
	}
	all, err := s.ListGroups(ctx, iamx.GroupQuery{OrgID: orgID})
	if err != nil {
		return nil, err
	}
	children := map[string][]iamx.Group{}
	for _, g := range all {
		children[g.ParentID] = append(children[g.ParentID], g)
	}
	out := []iamx.Group{}
	var walk func(string)
	walk = func(parent string) {
		for _, c := range children[parent] {
			out = append(out, c)
			walk(c.ID)
		}
	}
	walk(groupID)
	return out, nil
}

func (s *Store) AddMembership(ctx context.Context, m iamx.Membership) error {
	m = m.Normalize()
	if m.OrgID == "" || m.GroupID == "" || m.UserID == "" {
		return iamx.ErrInvalidArgument("membership org_id, group_id and user_id are required")
	}
	if _, err := s.GetUser(ctx, m.OrgID, m.UserID); err != nil {
		return err
	}
	if _, err := s.GetGroup(ctx, m.OrgID, m.GroupID); err != nil {
		return err
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = s.now()
	}
	model := toMembershipModel(m)
	_, err := s.db.ExecContext(ctx, s.bind(upsertMembershipSQL(s.dialect)), model.OrgID, model.GroupID, model.UserID, model.RoleIDs, model.Source, model.CreatedAt, model.RoleIDs, model.Source, model.CreatedAt)
	return normalizeErr(err, "membership")
}
func (s *Store) RemoveMembership(ctx context.Context, orgID, groupID, userID string) error {
	_, err := s.db.ExecContext(ctx, s.bind(`DELETE FROM kernel_iam_memberships WHERE org_id=? AND group_id=? AND user_id=?`), orgID, groupID, userID)
	return normalizeErr(err, "membership")
}
func (s *Store) ListMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	where, args := []string{"1=1"}, []any{}
	if orgID != "" {
		where = append(where, "org_id=?")
		args = append(args, orgID)
	}
	if userID != "" {
		where = append(where, "user_id=?")
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, s.bind(`SELECT org_id,group_id,user_id,role_ids,source,created_at FROM kernel_iam_memberships WHERE `+strings.Join(where, " AND ")+` ORDER BY group_id`), args...)
	if err != nil {
		return nil, normalizeErr(err, "membership")
	}
	defer rows.Close()
	out := []iamx.Membership{}
	for rows.Next() {
		var m membershipModel
		if err := rows.Scan(&m.OrgID, &m.GroupID, &m.UserID, &m.RoleIDs, &m.Source, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fromMembershipModel(m))
	}
	return out, rows.Err()
}
func (s *Store) ListEffectiveMemberships(ctx context.Context, orgID, userID string) ([]iamx.Membership, error) {
	direct, err := s.ListMemberships(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]iamx.Membership, 0, len(direct))
	seen := map[string]bool{}
	for _, m := range direct {
		out = append(out, m)
		seen[m.GroupID] = true
		ancestors, err := s.ListGroupAncestors(ctx, m.OrgID, m.GroupID)
		if err != nil {
			return nil, err
		}
		for _, g := range ancestors {
			if !seen[g.ID] {
				out = append(out, iamx.Membership{OrgID: m.OrgID, UserID: m.UserID, GroupID: g.ID, Source: iamx.MembershipInherited, CreatedAt: m.CreatedAt})
				seen[g.ID] = true
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func (s *Store) buildGroupPath(ctx context.Context, g iamx.Group) string {
	if g.ParentID == "" {
		return "/" + g.ID
	}
	p, err := s.GetGroup(ctx, g.OrgID, g.ParentID)
	if err != nil || p.Path == "" {
		return "/" + g.ParentID + "/" + g.ID
	}
	return p.Path + "/" + g.ID
}
func (s *Store) rebuildChildPaths(ctx context.Context, orgID, parentID string) error {
	children, err := s.ListGroups(ctx, iamx.GroupQuery{OrgID: orgID, ParentID: parentID})
	if err != nil {
		return err
	}
	for _, c := range children {
		c.Path = s.buildGroupPath(ctx, c)
		m := toGroupModel(c)
		if _, err := s.db.ExecContext(ctx, s.bind(`UPDATE kernel_iam_groups SET path=?,updated_at=? WHERE org_id=? AND id=?`), m.Path, s.now(), m.OrgID, m.ID); err != nil {
			return err
		}
		if err := s.rebuildChildPaths(ctx, orgID, c.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) bind(query string) string {
	if s.dialect != DialectPostgres {
		return query
	}
	n := 0
	var b strings.Builder
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func pageSQL(offset, limit int) string {
	out := ""
	if limit > 0 {
		out += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		out += fmt.Sprintf(" OFFSET %d", offset)
	}
	return out
}
func checkRows(res sql.Result, noun string) error {
	n, _ := res.RowsAffected()
	if n == 0 {
		return iamx.ErrNotFound(noun + " not found")
	}
	return nil
}
func normalizeErr(err error, noun string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return iamx.ErrNotFound(noun + " not found")
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed") {
		return iamx.ErrConflict(noun + " already exists")
	}
	return err
}
func upsertMembershipSQL(d Dialect) string {
	if d == DialectMySQL {
		return `INSERT INTO kernel_iam_memberships (org_id,group_id,user_id,role_ids,source,created_at) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE role_ids=?, source=?, created_at=?`
	}
	return `INSERT INTO kernel_iam_memberships (org_id,group_id,user_id,role_ids,source,created_at) VALUES (?,?,?,?,?,?) ON CONFLICT (org_id,group_id,user_id) DO UPDATE SET role_ids=?, source=?, created_at=?`
}
