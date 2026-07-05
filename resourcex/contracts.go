package resourcex

import "context"

type ResourceTypeFilter struct {
	Capability   string
	OwnerService string
	Grantable    *bool
	Status       string
}

type ResourceQuery struct {
	Type       string
	OrgID      string
	ProjectID  string
	Parent     ResourceRef
	Owner      string
	Status     string
	Visibility string
	Labels     map[string]string
	Limit      int
	Cursor     string
}

type ResourceList struct {
	Items      []Resource
	NextCursor string
	Total      int64
}

type BindingQuery struct {
	Source   ResourceRef
	Target   ResourceRef
	Relation string
	Status   string
	Limit    int
	Cursor   string
}

type BindingList struct {
	Items      []ResourceBinding
	NextCursor string
	Total      int64
}

type RoleTemplateQuery struct {
	ResourceType string
	RoleKey      string
	Enabled      *bool
}

type GrantQuery struct {
	Resource ResourceRef
	Subject  SubjectRef
	Relation string
	RoleKey  string
	Source   string
	Active   *bool
	Limit    int
	Cursor   string
}

type GrantList struct {
	Items      []Grant
	NextCursor string
	Total      int64
}

// ResourceRegistry manages resource types, resource projections and resource
// bindings. Implementations usually live in a platform IAM/resource service.
type ResourceRegistry interface {
	RegisterResourceType(ctx context.Context, typ ResourceType) (ResourceType, error)
	GetResourceType(ctx context.Context, typ string) (ResourceType, error)
	ListResourceTypes(ctx context.Context, filter ResourceTypeFilter) ([]ResourceType, error)

	UpsertResource(ctx context.Context, resource Resource) (Resource, error)
	GetResource(ctx context.Context, ref ResourceRef) (Resource, error)
	ListResources(ctx context.Context, query ResourceQuery) (ResourceList, error)
	MoveResource(ctx context.Context, ref ResourceRef, parent ResourceRef) (Resource, error)
	ArchiveResource(ctx context.Context, ref ResourceRef) (Resource, error)
	DeleteResource(ctx context.Context, ref ResourceRef) error

	BindResource(ctx context.Context, binding ResourceBinding) (ResourceBinding, error)
	UnbindResource(ctx context.Context, bindingID string) error
	ListResourceBindings(ctx context.Context, query BindingQuery) (BindingList, error)
}

// GrantManager manages product-level RBAC grants while projecting them to the
// authorization graph through an implementation-specific backend.
type GrantManager interface {
	RegisterRoleTemplate(ctx context.Context, tpl RoleTemplate) (RoleTemplate, error)
	ListRoleTemplates(ctx context.Context, query RoleTemplateQuery) ([]RoleTemplate, error)

	GrantAccess(ctx context.Context, grant Grant) (Grant, error)
	RevokeAccess(ctx context.Context, grantID string) error
	ListGrants(ctx context.Context, query GrantQuery) (GrantList, error)
}
