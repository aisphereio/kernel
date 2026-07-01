package iamx_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/iamx"
	"github.com/aisphereio/kernel/iamx/memory"
)

func TestServiceBuildGroupTreeAndMemberships(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	svc, err := iamx.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.EnsureOrganization(ctx, iamx.Organization{ID: "org1", Name: "Org 1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureGroup(ctx, iamx.Group{OrgID: "org1", ID: "root", Name: "Root", Type: iamx.GroupTypePhysical, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureGroup(ctx, iamx.Group{OrgID: "org1", ID: "team-a", ParentID: "root", Name: "Team A", Type: iamx.GroupTypeVirtual, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureUser(ctx, iamx.User{OrgID: "org1", ID: "u1", Username: "alice", Email: "Alice@Example.COM", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddMembership(ctx, iamx.Membership{OrgID: "org1", GroupID: "team-a", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}

	tree, err := svc.BuildGroupTree(ctx, "org1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Group.ID != "root" || len(tree[0].Children) != 1 || tree[0].Children[0].Group.ID != "team-a" {
		t.Fatalf("unexpected tree: %#v", tree)
	}

	users, err := store.ListUsers(ctx, iamx.UserQuery{OrgID: "org1", GroupID: "team-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Email != "alice@example.com" {
		t.Fatalf("unexpected group users: %#v", users)
	}
}
