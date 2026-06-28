package authz

import "context"

// DenyAll returns an authz.Service that denies every permission check and
// reports all non-check capabilities as unsupported. It is useful as the safe
// default when authz is disabled or not configured.
func DenyAll() Service { return denyAllService{} }

type denyAllService struct{}

func (denyAllService) Check(context.Context, CheckRequest) (Decision, error) {
	return Deny("deny_all"), nil
}

func (denyAllService) BatchCheck(_ context.Context, req BatchCheckRequest) (BatchCheckResult, error) {
	out := BatchCheckResult{Decisions: make([]Decision, len(req.Checks))}
	for i := range out.Decisions {
		out.Decisions[i] = Deny("deny_all")
	}
	return out, nil
}

func (denyAllService) WriteRelationships(context.Context, ...Relationship) (WriteResult, error) {
	return WriteResult{}, ErrUnsupportedCapability("deny_all authorizer does not support relationship writes")
}

func (denyAllService) DeleteRelationships(context.Context, RelationshipFilter) (WriteResult, error) {
	return WriteResult{}, ErrUnsupportedCapability("deny_all authorizer does not support relationship deletes")
}

func (denyAllService) ReadRelationships(context.Context, RelationshipFilter) ([]Relationship, error) {
	return nil, ErrUnsupportedCapability("deny_all authorizer does not support relationship reads")
}

func (denyAllService) LookupResources(context.Context, LookupResourcesRequest) (LookupResourcesResult, error) {
	return LookupResourcesResult{}, ErrUnsupportedCapability("deny_all authorizer does not support resource lookup")
}

func (denyAllService) LookupSubjects(context.Context, LookupSubjectsRequest) (LookupSubjectsResult, error) {
	return LookupSubjectsResult{}, ErrUnsupportedCapability("deny_all authorizer does not support subject lookup")
}

func (denyAllService) ReadSchema(context.Context) (Schema, error) {
	return Schema{}, ErrUnsupportedCapability("deny_all authorizer does not support schema reads")
}

func (denyAllService) WriteSchema(context.Context, Schema) error {
	return ErrUnsupportedCapability("deny_all authorizer does not support schema writes")
}

func (denyAllService) ValidateSchema(context.Context, Schema) error {
	return ErrUnsupportedCapability("deny_all authorizer does not support schema validation")
}

// AllowAllForDevOnly returns an authz.Service that allows every permission
// check and reports all administration/query capabilities as unsupported. This
// is intentionally explicit: dev-mode permission checks may pass, but relation
// management APIs still require a real backend such as SpiceDB.
func AllowAllForDevOnly() Service { return allowAllService{} }

type allowAllService struct{}

func (allowAllService) Check(context.Context, CheckRequest) (Decision, error) {
	return Allow("allow_all_dev_only"), nil
}

func (allowAllService) BatchCheck(_ context.Context, req BatchCheckRequest) (BatchCheckResult, error) {
	out := BatchCheckResult{Decisions: make([]Decision, len(req.Checks))}
	for i := range out.Decisions {
		out.Decisions[i] = Allow("allow_all_dev_only")
	}
	return out, nil
}

func (allowAllService) WriteRelationships(context.Context, ...Relationship) (WriteResult, error) {
	return WriteResult{}, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support relationship writes")
}

func (allowAllService) DeleteRelationships(context.Context, RelationshipFilter) (WriteResult, error) {
	return WriteResult{}, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support relationship deletes")
}

func (allowAllService) ReadRelationships(context.Context, RelationshipFilter) ([]Relationship, error) {
	return nil, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support relationship reads")
}

func (allowAllService) LookupResources(context.Context, LookupResourcesRequest) (LookupResourcesResult, error) {
	return LookupResourcesResult{}, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support resource lookup")
}

func (allowAllService) LookupSubjects(context.Context, LookupSubjectsRequest) (LookupSubjectsResult, error) {
	return LookupSubjectsResult{}, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support subject lookup")
}

func (allowAllService) ReadSchema(context.Context) (Schema, error) {
	return Schema{}, ErrUnsupportedCapability("allow_all_dev_only authorizer does not support schema reads")
}

func (allowAllService) WriteSchema(context.Context, Schema) error {
	return ErrUnsupportedCapability("allow_all_dev_only authorizer does not support schema writes")
}

func (allowAllService) ValidateSchema(context.Context, Schema) error {
	return ErrUnsupportedCapability("allow_all_dev_only authorizer does not support schema validation")
}
