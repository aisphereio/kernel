package authz

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/auditx"
)

func TestMemoryRelationshipStoreAndAuthorizer(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryRelationshipStore()
	_, err := store.WriteRelationships(ctx, Relationship{
		Resource: ObjectRef{Type: "document", ID: "doc_1"},
		Relation: "viewer",
		Subject:  SubjectRef{Type: "user", ID: "u_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := NewMemoryAuthorizer(store).Check(ctx, CheckRequest{
		Resource:   ObjectRef{Type: "document", ID: "doc_1"},
		Permission: "viewer",
		Subject:    SubjectRef{Type: "user", ID: "u_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allow, got %#v", decision)
	}
}

func TestAuditedAuthorizer(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryRelationshipStore()
	_, _ = store.WriteRelationships(ctx, Relationship{
		Resource: ObjectRef{Type: "document", ID: "doc_1"},
		Relation: "viewer",
		Subject:  SubjectRef{Type: "user", ID: "u_1"},
	})
	auditStore := auditx.NewMemoryStore()
	authorizer := NewAuditedAuthorizer(NewMemoryAuthorizer(store), auditStore)
	_, err := authorizer.Check(ctx, CheckRequest{
		Resource:   ObjectRef{Type: "document", ID: "doc_1"},
		Permission: "viewer",
		Subject:    SubjectRef{Type: "user", ID: "u_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	records, err := auditStore.Query(ctx, auditx.QueryFilter{ActorID: "u_1", Action: "authz.check"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Result != auditx.ResultSuccess {
		t.Fatalf("unexpected audit records: %#v", records)
	}
}

func TestRuleResolverNestedFieldPath(t *testing.T) {
	type Owner struct {
		ID string `json:"id" protobuf:"bytes,1,opt,name=id,json=id,proto3"`
	}
	type Request struct {
		SkillID string `json:"skill_id" protobuf:"bytes,1,opt,name=skill_id,json=skillId,proto3"`
		Owner   *Owner `json:"owner" protobuf:"bytes,2,opt,name=owner,json=owner,proto3"`
	}

	resolver := RuleResolver{}
	resource, err := resolver.ResolveResource(Rule{Resource: "skill:{owner.id}:{skill_id}"}, &Request{SkillID: "s_1", Owner: &Owner{ID: "u_1"}})
	if err != nil {
		t.Fatal(err)
	}
	if resource.Type != "skill" || resource.ID != "u_1:s_1" {
		t.Fatalf("unexpected resource: %#v", resource)
	}
}
