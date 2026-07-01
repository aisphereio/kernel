package memory_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/iamx"
	"github.com/aisphereio/kernel/iamx/memory"
)

func TestMultiLevelGroupsAndEffectiveMemberships(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	_, _ = store.CreateOrganization(ctx, iamx.Organization{ID: "org1", Name: "Org", Enabled: true})
	_, _ = store.CreateGroup(ctx, iamx.Group{OrgID: "org1", ID: "root", Name: "Root", Enabled: true})
	_, _ = store.CreateGroup(ctx, iamx.Group{OrgID: "org1", ID: "dept", ParentID: "root", Name: "Dept", Enabled: true})
	team, err := store.CreateGroup(ctx, iamx.Group{OrgID: "org1", ID: "team", ParentID: "dept", Name: "Team", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if team.Path != "/root/dept/team" {
		t.Fatalf("unexpected path: %q", team.Path)
	}
	_, _ = store.CreateUser(ctx, iamx.User{OrgID: "org1", ID: "u1", Username: "alice", Enabled: true})
	if err := store.AddMembership(ctx, iamx.Membership{OrgID: "org1", GroupID: "team", UserID: "u1"}); err != nil {
		t.Fatal(err)
	}
	eff, err := store.ListEffectiveMemberships(ctx, "org1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range eff {
		got[m.GroupID] = m.Source
	}
	if got["team"] != iamx.MembershipDirect || got["dept"] != iamx.MembershipInherited || got["root"] != iamx.MembershipInherited {
		t.Fatalf("unexpected effective memberships: %#v", eff)
	}
	ancestors, err := store.ListGroupAncestors(ctx, "org1", "team")
	if err != nil {
		t.Fatal(err)
	}
	if len(ancestors) != 2 || ancestors[0].ID != "dept" || ancestors[1].ID != "root" {
		t.Fatalf("unexpected ancestors: %#v", ancestors)
	}
	desc, err := store.ListGroupDescendants(ctx, "org1", "root")
	if err != nil {
		t.Fatal(err)
	}
	if len(desc) != 2 || desc[0].ID != "dept" || desc[1].ID != "team" {
		t.Fatalf("unexpected descendants: %#v", desc)
	}
}

func TestCRUDSemantics(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	if _, err := store.CreateOrganization(ctx, iamx.Organization{ID: "org1", Name: "Org", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, iamx.User{OrgID: "org1", ID: "u1", Username: "alice", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, iamx.User{OrgID: "org1", ID: "u1", Username: "alice", Enabled: true}); err == nil {
		t.Fatal("expected duplicate user conflict")
	}
	updated, err := store.UpdateUser(ctx, iamx.User{OrgID: "org1", ID: "u1", Username: "alice2", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "alice2" {
		t.Fatalf("unexpected user: %#v", updated)
	}
	if err := store.DisableUser(ctx, "org1", "u1"); err != nil {
		t.Fatal(err)
	}
	disabled, err := store.GetUser(ctx, "org1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatal("expected disabled user")
	}
	if err := store.DeleteUser(ctx, "org1", "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUser(ctx, "org1", "u1"); err == nil {
		t.Fatal("expected deleted user to be hidden")
	}
}
