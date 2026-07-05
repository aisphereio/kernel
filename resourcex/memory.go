package resourcex

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aisphereio/kernel/authz"
)

type MemoryStore struct {
	mu       sync.RWMutex
	types    map[string]ResourceType
	items    map[string]Resource
	bindings map[string]ResourceBinding
	roles    map[string]RoleTemplate
	grants   map[string]Grant
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		types:    map[string]ResourceType{},
		items:    map[string]Resource{},
		bindings: map[string]ResourceBinding{},
		roles:    map[string]RoleTemplate{},
		grants:   map[string]Grant{},
	}
}

var _ ResourceRegistry = (*MemoryStore)(nil)
var _ GrantManager = (*MemoryStore)(nil)

func (s *MemoryStore) RegisterResourceType(ctx context.Context, typ ResourceType) (ResourceType, error) {
	if err := ctx.Err(); err != nil {
		return ResourceType{}, err
	}
	if err := ValidateResourceType(typ); err != nil {
		return ResourceType{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	typ.Type = NormalizeType(typ.Type)
	if typ.Status == "" {
		typ.Status = StatusActive
	}
	now := time.Now().UTC()
	if typ.CreatedAt.IsZero() {
		typ.CreatedAt = now
	}
	typ.UpdatedAt = now
	s.types[typ.Type] = typ
	return typ, nil
}

func (s *MemoryStore) GetResourceType(ctx context.Context, typ string) (ResourceType, error) {
	if err := ctx.Err(); err != nil {
		return ResourceType{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out, ok := s.types[NormalizeType(typ)]
	if !ok {
		return ResourceType{}, fmt.Errorf("resource type %q not found", typ)
	}
	return out, nil
}

func (s *MemoryStore) ListResourceTypes(ctx context.Context, filter ResourceTypeFilter) ([]ResourceType, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceType, 0, len(s.types))
	for _, typ := range s.types {
		if filter.Capability != "" && typ.Capability != filter.Capability {
			continue
		}
		if filter.OwnerService != "" && typ.OwnerService != filter.OwnerService {
			continue
		}
		if filter.Grantable != nil && typ.Grantable != *filter.Grantable {
			continue
		}
		if filter.Status != "" && typ.Status != filter.Status {
			continue
		}
		out = append(out, typ)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
}

func (s *MemoryStore) UpsertResource(ctx context.Context, resource Resource) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	if err := ValidateResource(resource); err != nil {
		return Resource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.types[NormalizeType(resource.Ref.Type)]; !ok {
		return Resource{}, fmt.Errorf("resource type %q not registered", resource.Ref.Type)
	}
	resource.Ref.Type = NormalizeType(resource.Ref.Type)
	if resource.Status == "" {
		resource.Status = StatusActive
	}
	if resource.Visibility == "" {
		resource.Visibility = VisibilityPrivate
	}
	now := time.Now().UTC()
	if resource.CreatedAt.IsZero() {
		resource.CreatedAt = now
	}
	resource.UpdatedAt = now
	s.items[objectKey(resource.Ref)] = resource
	return resource, nil
}

func (s *MemoryStore) GetResource(ctx context.Context, ref ResourceRef) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out, ok := s.items[objectKey(ref)]
	if !ok {
		return Resource{}, fmt.Errorf("resource %s not found", ref.String())
	}
	return out, nil
}

func (s *MemoryStore) ListResources(ctx context.Context, query ResourceQuery) (ResourceList, error) {
	if err := ctx.Err(); err != nil {
		return ResourceList{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Resource, 0)
	for _, item := range s.items {
		if query.Type != "" && item.Ref.Type != NormalizeType(query.Type) {
			continue
		}
		if query.OrgID != "" && item.OrgID != query.OrgID {
			continue
		}
		if query.ProjectID != "" && item.ProjectID != query.ProjectID {
			continue
		}
		if !query.Parent.IsZero() && item.Parent != query.Parent {
			continue
		}
		if query.Owner != "" && item.OwnerService != query.Owner {
			continue
		}
		if query.Status != "" && item.Status != query.Status {
			continue
		}
		if query.Visibility != "" && item.Visibility != query.Visibility {
			continue
		}
		if !labelsMatch(item.Labels, query.Labels) {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return ResourceList{Items: limitResources(out, query.Limit), Total: int64(len(out))}, nil
}

func (s *MemoryStore) MoveResource(ctx context.Context, ref ResourceRef, parent ResourceRef) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := objectKey(ref)
	item, ok := s.items[key]
	if !ok {
		return Resource{}, fmt.Errorf("resource %s not found", ref.String())
	}
	item.Parent = parent
	item.UpdatedAt = time.Now().UTC()
	s.items[key] = item
	return item, nil
}

func (s *MemoryStore) ArchiveResource(ctx context.Context, ref ResourceRef) (Resource, error) {
	return s.setResourceStatus(ctx, ref, StatusArchived)
}

func (s *MemoryStore) DeleteResource(ctx context.Context, ref ResourceRef) error {
	_, err := s.setResourceStatus(ctx, ref, StatusDeleted)
	return err
}

func (s *MemoryStore) setResourceStatus(ctx context.Context, ref ResourceRef, status string) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := objectKey(ref)
	item, ok := s.items[key]
	if !ok {
		return Resource{}, fmt.Errorf("resource %s not found", ref.String())
	}
	item.Status = status
	item.UpdatedAt = time.Now().UTC()
	if status == StatusDeleted {
		item.DeletedAt = item.UpdatedAt
	}
	s.items[key] = item
	return item, nil
}

func (s *MemoryStore) BindResource(ctx context.Context, binding ResourceBinding) (ResourceBinding, error) {
	if err := ctx.Err(); err != nil {
		return ResourceBinding{}, err
	}
	if err := ValidateBinding(binding); err != nil {
		return ResourceBinding{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if binding.ID == "" {
		binding.ID = bindingKey(binding.Source, binding.Relation, binding.Target)
	}
	if binding.Status == "" {
		binding.Status = StatusActive
	}
	now := time.Now().UTC()
	if binding.CreatedAt.IsZero() {
		binding.CreatedAt = now
	}
	binding.UpdatedAt = now
	s.bindings[binding.ID] = binding
	return binding, nil
}

func (s *MemoryStore) UnbindResource(ctx context.Context, bindingID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bindings, bindingID)
	return nil
}

func (s *MemoryStore) ListResourceBindings(ctx context.Context, query BindingQuery) (BindingList, error) {
	if err := ctx.Err(); err != nil {
		return BindingList{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceBinding, 0)
	for _, binding := range s.bindings {
		if !query.Source.IsZero() && binding.Source != query.Source {
			continue
		}
		if !query.Target.IsZero() && binding.Target != query.Target {
			continue
		}
		if query.Relation != "" && binding.Relation != query.Relation {
			continue
		}
		if query.Status != "" && binding.Status != query.Status {
			continue
		}
		out = append(out, binding)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return BindingList{Items: limitBindings(out, query.Limit), Total: int64(len(out))}, nil
}

func (s *MemoryStore) RegisterRoleTemplate(ctx context.Context, tpl RoleTemplate) (RoleTemplate, error) {
	if err := ctx.Err(); err != nil {
		return RoleTemplate{}, err
	}
	if err := ValidateRoleTemplate(tpl); err != nil {
		return RoleTemplate{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if tpl.ID == "" {
		tpl.ID = tpl.ResourceType + ":" + tpl.RoleKey
	}
	s.roles[tpl.ID] = tpl
	return tpl, nil
}

func (s *MemoryStore) ListRoleTemplates(ctx context.Context, query RoleTemplateQuery) ([]RoleTemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RoleTemplate, 0, len(s.roles))
	for _, role := range s.roles {
		if query.ResourceType != "" && role.ResourceType != query.ResourceType {
			continue
		}
		if query.RoleKey != "" && role.RoleKey != query.RoleKey {
			continue
		}
		if query.Enabled != nil && role.Enabled != *query.Enabled {
			continue
		}
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemoryStore) GrantAccess(ctx context.Context, grant Grant) (Grant, error) {
	if err := ctx.Err(); err != nil {
		return Grant{}, err
	}
	if err := ValidateGrant(grant); err != nil {
		return Grant{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if grant.ID == "" {
		grant.ID = grantKey(grant.Resource, grant.Relation, grant.Subject)
	}
	if grant.Source == "" {
		grant.Source = SourceManual
	}
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = time.Now().UTC()
	}
	s.grants[grant.ID] = grant
	return grant, nil
}

func (s *MemoryStore) RevokeAccess(ctx context.Context, grantID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return nil
	}
	grant.RevokedAt = time.Now().UTC()
	s.grants[grantID] = grant
	return nil
}

func (s *MemoryStore) ListGrants(ctx context.Context, query GrantQuery) (GrantList, error) {
	if err := ctx.Err(); err != nil {
		return GrantList{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Grant, 0)
	for _, grant := range s.grants {
		if !query.Resource.IsZero() && grant.Resource != query.Resource {
			continue
		}
		if !query.Subject.IsZero() && grant.Subject != query.Subject {
			continue
		}
		if query.Relation != "" && grant.Relation != query.Relation {
			continue
		}
		if query.RoleKey != "" && grant.RoleKey != query.RoleKey {
			continue
		}
		if query.Source != "" && grant.Source != query.Source {
			continue
		}
		if query.Active != nil {
			active := grant.RevokedAt.IsZero()
			if active != *query.Active {
				continue
			}
		}
		out = append(out, grant)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return GrantList{Items: limitGrants(out, query.Limit), Total: int64(len(out))}, nil
}

func objectKey(ref authz.ObjectRef) string {
	return NormalizeType(ref.Type) + ":" + strings.TrimSpace(ref.ID)
}
func bindingKey(source ResourceRef, relation string, target ResourceRef) string {
	return objectKey(source) + "#" + relation + "@" + objectKey(target)
}
func grantKey(resource ResourceRef, relation string, subject SubjectRef) string {
	return objectKey(resource) + "#" + relation + "@" + subject.String()
}

func labelsMatch(labels, want map[string]string) bool {
	for k, v := range want {
		if labels[k] != v {
			return false
		}
	}
	return true
}
func limitResources(in []Resource, limit int) []Resource {
	if limit > 0 && len(in) > limit {
		return in[:limit]
	}
	return in
}
func limitBindings(in []ResourceBinding, limit int) []ResourceBinding {
	if limit > 0 && len(in) > limit {
		return in[:limit]
	}
	return in
}
func limitGrants(in []Grant, limit int) []Grant {
	if limit > 0 && len(in) > limit {
		return in[:limit]
	}
	return in
}
