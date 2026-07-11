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

// RuntimeService is the complete authorization data-plane surface used by
// business services at request time. It intentionally excludes schema
// administration so services can delegate checks, lookups, and relationship
// projection to an IAM authorization service without receiving permission to
// publish or replace the shared authorization model.
type RuntimeService interface {
	Authorizer
	BatchAuthorizer
	ResourceLookup
	SubjectLookup
	RelationshipStore
}

// SchemaManager is implemented by engines like SpiceDB that own an explicit
// authorization schema.
type SchemaManager interface {
	ReadSchema(ctx context.Context) (Schema, error)
	WriteSchema(ctx context.Context, schema Schema) error
	ValidateSchema(ctx context.Context, schema Schema) error
}

// Service is the complete authorization control-plane surface. Business hot
// paths normally depend on RuntimeService; IAM/admin provisioning paths that
// own the authorization model may depend on Service.
type Service interface {
	RuntimeService
	SchemaManager
}

// RelationshipProjector converts domain events from Kernel IAM/Hub/etc. into
// ReBAC tuples. Pair it with a transactional outbox to avoid dual-write drift.
type RelationshipProjector interface {
	ProjectRelationships(ctx context.Context, event any) ([]Relationship, error)
}
