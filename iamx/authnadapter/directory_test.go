package authnadapter_test

import (
	"testing"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/iamx"
	"github.com/aisphereio/kernel/iamx/authnadapter"
)

func TestMappingKeepsKernelIAMIndependentFromAuthnProvider(t *testing.T) {
	user := authn.User{ID: "u1", ExternalID: "casdoor/owner/alice", Provider: "casdoor", OrgID: "org1", Username: "alice", DisplayName: "Alice", Email: "Alice@Example.COM", Enabled: true, Attributes: authn.AttributeSet{"casdoor_owner": "built-in"}}
	got := authnadapter.FromAuthnUser(user)
	if got.ID != "u1" || got.Provider != iamx.ProviderCasdoor || got.Email != "alice@example.com" {
		t.Fatalf("unexpected mapped user: %#v", got)
	}

	back := authnadapter.ToAuthnUser(got)
	if back.ID != user.ID || back.DisplayName != user.DisplayName || back.Email != got.Email {
		t.Fatalf("unexpected reverse mapping: %#v", back)
	}
}
