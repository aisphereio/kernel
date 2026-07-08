package contextx

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/aisphereio/kernel/authn"
)

func TestWithAuthnPrincipalMirrorsAllFields(t *testing.T) {
	issuedAt := time.Unix(100, 0)
	expiresAt := time.Unix(200, 0)
	source := authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
		Provider:    "casdoor",
		ExternalID:  "external-1",
		Issuer:      "https://casdoor.example.com",
		Audience:    []string{"aud-1", "aud-2"},
		TenantID:    "tenant-1",
		OrgID:       "org-1",
		AppID:       "app-1",
		ProjectID:   "project-1",
		Username:    "admin",
		Name:        "Administrator",
		Email:       "admin@example.com",
		Phone:       "123456789",
		Roles:       []string{"owner", "admin"},
		Groups:      []string{"platform", "agent-team"},
		Scopes:      []string{"openid", "profile", "email"},
		AuthMethod:  authn.AuthMethodOIDC,
		Attributes: authn.AttributeSet{
			"k": "v",
		},
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}

	ctx := WithAuthnPrincipal(context.Background(), source)
	got := PrincipalFromContext(ctx)
	if got == nil {
		t.Fatal("expected contextx principal")
	}

	if got.SubjectID != source.SubjectID || got.SubjectType != source.SubjectType {
		t.Fatalf("subject fields not mirrored: %#v", got)
	}
	if got.Provider != source.Provider || got.ExternalID != source.ExternalID || got.Issuer != source.Issuer {
		t.Fatalf("provider fields not mirrored: %#v", got)
	}
	if !reflect.DeepEqual(got.Audience, source.Audience) {
		t.Fatalf("audience not mirrored: %#v", got.Audience)
	}
	if got.TenantID != source.TenantID || got.OrgID != source.OrgID || got.AppID != source.AppID || got.ProjectID != source.ProjectID {
		t.Fatalf("tenant/resource fields not mirrored: %#v", got)
	}
	if got.Username != source.Username || got.Name != source.Name || got.Email != source.Email || got.Phone != source.Phone {
		t.Fatalf("profile fields not mirrored: %#v", got)
	}
	if !reflect.DeepEqual(got.Roles, source.Roles) || !reflect.DeepEqual(got.Groups, source.Groups) || !reflect.DeepEqual(got.Scopes, source.Scopes) {
		t.Fatalf("roles/groups/scopes not mirrored: %#v", got)
	}
	if got.AuthMethod != source.AuthMethod || got.Attribute("k") != "v" || !got.IssuedAt.Equal(issuedAt) || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("auth metadata not mirrored: %#v", got)
	}

	roundTrip := ToAuthnPrincipal(got)
	if roundTrip.OrgID != source.OrgID || roundTrip.ProjectID != source.ProjectID || roundTrip.Phone != source.Phone {
		t.Fatalf("round trip lost fields: %#v", roundTrip)
	}
}
