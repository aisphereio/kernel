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
