package authn

import "testing"

func TestIdentityProfileGroupIDs(t *testing.T) {
	profile := IdentityProfile{Groups: []Group{{ID: "g1"}, {Name: "g2"}, {ID: "g1"}, {}}}
	ids := profile.GroupIDs()
	if len(ids) != 2 || ids[0] != "g1" || ids[1] != "g2" {
		t.Fatalf("unexpected group ids: %#v", ids)
	}
}

func TestIdentityProfilePartial(t *testing.T) {
	profile := IdentityProfile{Warnings: []string{"group lookup failed"}}
	if !profile.IsPartial() {
		t.Fatal("expected partial profile")
	}
}
