package iam_test

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/iamx"
	"github.com/aisphereio/kernel/iamx/memory"
)

func TestIAMDomainSupportsCasdoorStyleOrgAndGroupTree(t *testing.T) {
	ctx := context.Background()
	dir := memory.New()
	svc, err := iamx.NewService(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = svc.EnsureOrganization(ctx, iamx.Organization{ID: "aisphere", Name: "Aisphere", Enabled: true, Provider: iamx.ProviderCasdoor, ExternalID: "built-in/aisphere"})
	_, _ = svc.EnsureGroup(ctx, iamx.Group{OrgID: "aisphere", ID: "engineering", Name: "研发部", Type: iamx.GroupTypePhysical, Enabled: true, Provider: iamx.ProviderCasdoor, ExternalID: "built-in/engineering"})
	_, _ = svc.EnsureGroup(ctx, iamx.Group{OrgID: "aisphere", ID: "platform", ParentID: "engineering", Name: "平台组", Type: iamx.GroupTypeVirtual, Enabled: true, Provider: iamx.ProviderCasdoor, ExternalID: "built-in/platform"})
	_, _ = svc.EnsureUser(ctx, iamx.User{OrgID: "aisphere", ID: "u-admin", Username: "admin", Name: "平台管理员", Enabled: true, Provider: iamx.ProviderCasdoor, ExternalID: "built-in/admin"})
	if err := dir.AddMembership(ctx, iamx.Membership{OrgID: "aisphere", GroupID: "platform", UserID: "u-admin"}); err != nil {
		t.Fatal(err)
	}

	tree, err := svc.BuildGroupTree(ctx, "aisphere")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Group.ID != "engineering" || len(tree[0].Children) != 1 || tree[0].Children[0].Group.ID != "platform" {
		t.Fatalf("unexpected tree: %#v", tree)
	}

	memberships, err := dir.ListMemberships(ctx, "aisphere", "u-admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 || memberships[0].GroupID != "platform" {
		t.Fatalf("unexpected memberships: %#v", memberships)
	}
}
