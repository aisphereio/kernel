package authz

import (
	"context"
	"testing"
)

type runtimeOnlyService struct{}

func (runtimeOnlyService) Check(context.Context, CheckRequest) (Decision, error) {
	return Decision{}, nil
}

func (runtimeOnlyService) BatchCheck(context.Context, BatchCheckRequest) (BatchCheckResult, error) {
	return BatchCheckResult{}, nil
}

func (runtimeOnlyService) LookupResources(context.Context, LookupResourcesRequest) (LookupResourcesResult, error) {
	return LookupResourcesResult{}, nil
}

func (runtimeOnlyService) LookupSubjects(context.Context, LookupSubjectsRequest) (LookupSubjectsResult, error) {
	return LookupSubjectsResult{}, nil
}

func (runtimeOnlyService) WriteRelationships(context.Context, ...Relationship) (WriteResult, error) {
	return WriteResult{}, nil
}

func (runtimeOnlyService) DeleteRelationships(context.Context, RelationshipFilter) (WriteResult, error) {
	return WriteResult{}, nil
}

func (runtimeOnlyService) ReadRelationships(context.Context, RelationshipFilter) ([]Relationship, error) {
	return nil, nil
}

type fullService struct{ runtimeOnlyService }

func (fullService) ReadSchema(context.Context) (Schema, error) {
	return Schema{}, nil
}

func (fullService) WriteSchema(context.Context, Schema) error {
	return nil
}

func (fullService) ValidateSchema(context.Context, Schema) error {
	return nil
}

var _ RuntimeService = runtimeOnlyService{}
var _ Service = fullService{}

func TestRuntimeServiceDoesNotRequireSchemaManager(t *testing.T) {
	var runtime RuntimeService = runtimeOnlyService{}
	if _, ok := runtime.(SchemaManager); ok {
		t.Fatal("runtime authorization service must not imply schema administration")
	}
}
