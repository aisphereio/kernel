package accessx

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestDenyAllGuardRequire(t *testing.T) {
	guard := DenyAllGuard()
	_, err := guard.Require(context.Background(), Check{
		Principal:  authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
		Permission: "read",
		Resource:   authz.ObjectRef{Type: "document", ID: "d1"},
	})
	if err == nil {
		t.Fatal("expected denied error")
	}
}

func TestAllowAllForDevOnlyCan(t *testing.T) {
	guard := AllowAllForDevOnly()
	allowed, err := guard.Can(context.Background(), Check{
		Principal:  authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
		Permission: "read",
		Resource:   authz.ObjectRef{Type: "document", ID: "d1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed")
	}
}
