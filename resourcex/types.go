package resourcex

import (
	"strings"
	"time"

	"github.com/aisphereio/kernel/authz"
)

const (
	SubjectTypeUser           = authz.SubjectTypeUser
	SubjectTypeGroup          = authz.SubjectTypeGroup
	SubjectTypeServiceAccount = "service_account"
	SubjectTypeAgent          = "agent"

	SourceManual       = "manual"
	SourceSystem       = "system"
	SourceInherited    = "inherited"
	SourceExternalSync = "external_sync"
	SourceTemplate     = "template"

	StatusActive   = "active"
	StatusArchived = "archived"
	StatusDeleted  = "deleted"

	VisibilityPrivate = "private"
	VisibilityOrg     = "org"
	VisibilityPublic  = "public"
)

type ResourceRef = authz.ObjectRef
type SubjectRef = authz.SubjectRef

type AttributeSet map[string]any

// ResourceType describes a grantable resource kind and how it maps to the
// authorization graph. Instances can be dynamic, but resource types are a
// governed contract and normally require a matching SpiceDB/OpenFGA schema.
type ResourceType struct {
	Type         string
	Capability   string
	OwnerService string
	DisplayName  string
	Description  string

	ParentTypes []string
	Grantable   bool
	Auditable   bool
	SpiceDBType string

	Relations   []string
	Permissions []string
	Labels      map[string]string
	Metadata    AttributeSet

	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Resource is the IAM/control-plane projection of a business resource. It is
// intentionally not the resource body: owner services still own their domain
// tables, payloads and lifecycle-specific state.
type Resource struct {
	Ref ResourceRef

	OrgID     string
	ProjectID string
	Parent    ResourceRef

	OwnerService    string
	OwnerResourceID string

	Slug        string
	DisplayName string
	Path        string
	Status      string
	Visibility  string
	Grantable   bool

	Labels      map[string]string
	Annotations map[string]string
	Metadata    AttributeSet

	CreatedBy SubjectRef
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

func (r Resource) IsZero() bool { return r.Ref.IsZero() }

// ResourceBinding captures cross-domain resource relationships, such as a
// skill backed by a git repository, or an agent using a skill/tool.
type ResourceBinding struct {
	ID       string
	Source   ResourceRef
	Relation string
	Target   ResourceRef

	Status    string
	Metadata  AttributeSet
	CreatedBy SubjectRef
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ExternalResourceBinding maps an Aisphere resource projection to an external
// provider resource, for example a Forgejo repository or Kubernetes namespace.
type ExternalResourceBinding struct {
	ID       string
	Resource ResourceRef

	Provider     string
	ExternalType string
	ExternalID   string
	ExternalPath string
	ExternalURL  string

	SyncMode     string
	SyncStatus   string
	LastSyncedAt time.Time
	Metadata     AttributeSet
}

// RoleTemplate is the product-facing RBAC role. It maps to an authorization
// relation. The graph engine still evaluates ReBAC relationships.
type RoleTemplate struct {
	ID           string
	ResourceType string
	RoleKey      string
	DisplayName  string
	Description  string
	Relation     string
	BuiltIn      bool
	Enabled      bool
	SortOrder    int
	Metadata     AttributeSet
}

// Grant is the durable management record for an authorization relationship.
// The grant is the control-plane fact; SpiceDB/OpenFGA relationships are the
// query-optimized projection.
type Grant struct {
	ID       string
	Resource ResourceRef

	RoleKey  string
	Relation string
	Subject  SubjectRef

	Source    string
	Reason    string
	ExpiresAt time.Time

	CreatedBy SubjectRef
	CreatedAt time.Time
	RevokedAt time.Time

	ConsistencyToken string
	Metadata         AttributeSet
}

func NormalizeType(t string) string { return strings.TrimSpace(strings.ToLower(t)) }
