package resourcex

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/authz"
)

func TestMemoryStoreResourceAndGrant(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	_, err := store.RegisterResourceType(ctx, ResourceType{
		Type: "skill", Capability: "hub", OwnerService: "aisphere-hub", SpiceDBType: "skill", Grantable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.UpsertResource(ctx, Resource{
		Ref: authz.ObjectRef{Type: "skill", ID: "s1"}, OrgID: "org1", ProjectID: "p1", OwnerService: "aisphere-hub", OwnerResourceID: "s1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetResource(ctx, authz.ObjectRef{Type: "skill", ID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrgID != "org1" || got.ProjectID != "p1" {
		t.Fatalf("unexpected resource: %#v", got)
	}

	grant, err := store.GrantAccess(ctx, Grant{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"}, Relation: "editor", Subject: authz.SubjectRef{Type: "user", ID: "u1"}, RoleKey: "editor",
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListGrants(ctx, GrantQuery{Resource: authz.ObjectRef{Type: "skill", ID: "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != grant.ID {
		t.Fatalf("unexpected grants: %#v", list.Items)
	}
}
