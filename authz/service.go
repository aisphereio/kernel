package authz

import "context"

type Authorizer interface {
	Check(ctx context.Context, req CheckRequest) (Decision, error)
}

type BatchAuthorizer interface {
	BatchCheck(ctx context.Context, req BatchCheckRequest) (BatchCheckResult, error)
}

type ResourceLookup interface {
	LookupResources(ctx context.Context, req LookupResourcesRequest) (LookupResourcesResult, error)
}

type SubjectLookup interface {
	LookupSubjects(ctx context.Context, req LookupSubjectsRequest) (LookupSubjectsResult, error)
}

type RelationshipWriter interface {
	WriteRelationships(ctx context.Context, relationships ...Relationship) (WriteResult, error)
	DeleteRelationships(ctx context.Context, filter RelationshipFilter) (WriteResult, error)
}

type RelationshipReader interface {
	ReadRelationships(ctx context.Context, filter RelationshipFilter) ([]Relationship, error)
}

type RelationshipStore interface {
	RelationshipWriter
	RelationshipReader
}

// SchemaManager is implemented by engines like SpiceDB that own an explicit
// authorization schema.
type SchemaManager interface {
	ReadSchema(ctx context.Context) (Schema, error)
	WriteSchema(ctx context.Context, schema Schema) error
	ValidateSchema(ctx context.Context, schema Schema) error
}

// RuntimeService is the authorization surface exposed by IAM to business
// services. It intentionally excludes schema management: only IAM owns and
// publishes the backing authorization engine schema.
type RuntimeService interface {
	Authorizer
	BatchAuthorizer
	ResourceLookup
	SubjectLookup
	RelationshipStore
}

// Service is the complete authorization-engine surface. It is implemented by
// providers such as SpiceDB and must only be constructed inside IAM.
type Service interface {
	RuntimeService
	SchemaManager
}

// RelationshipProjector converts domain events from Kernel IAM/Hub/etc. into
// ReBAC tuples. Pair it with a transactional outbox to avoid dual-write drift.
type RelationshipProjector interface {
	ProjectRelationships(ctx context.Context, event any) ([]Relationship, error)
}
