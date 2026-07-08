package authn

import "testing"

func TestPrincipalFromGatewayClaimHeaders(t *testing.T) {
	p, ok := PrincipalFromTrustedHeaders(map[string]string{
		"x-aisphere-external-sub":      "admin/123",
		"x-aisphere-external-issuer":   "https://casdoor.example.com",
		"x-aisphere-external-email":    "admin@example.com",
		"x-aisphere-external-name":     "Admin User",
		"x-aisphere-external-username": "admin",
		"x-aisphere-org-id":            "aisphere",
		"x-aisphere-project-id":        "default",
		"x-aisphere-roles":             "owner,admin",
		"x-aisphere-groups":            "platform,agent-team",
	})
	if !ok {
		t.Fatal("expected principal from gateway claim headers")
	}
	if p.SubjectID != "admin/123" || p.ExternalID != "admin/123" {
		t.Fatalf("unexpected subject/external id: %#v", p)
	}
	if p.Issuer != "https://casdoor.example.com" {
		t.Fatalf("unexpected issuer: %q", p.Issuer)
	}
	if p.Email != "admin@example.com" || p.Username != "admin" {
		t.Fatalf("unexpected profile fields: %#v", p)
	}
	if p.OrgID != "aisphere" || p.ProjectID != "default" {
		t.Fatalf("unexpected org/project: %#v", p)
	}
	if !p.HasRole("owner") || !p.HasRole("admin") {
		t.Fatalf("roles not parsed: %#v", p.Roles)
	}
	if !p.HasGroup("platform") || !p.HasGroup("agent-team") {
		t.Fatalf("groups not parsed: %#v", p.Groups)
	}
	if p.AuthMethod != AuthMethodOIDC {
		t.Fatalf("unexpected auth method: %q", p.AuthMethod)
	}
}

func TestPrincipalFromGatewayClaimHeadersCasdoorProfileClaims(t *testing.T) {
	p, ok := PrincipalFromTrustedHeaders(map[string]string{
		"x-aisphere-external-sub":          "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-external-id":           "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-external-issuer":       "https://casdoor.weagent.cc:30723",
		"x-aisphere-external-email":        "l2ov7s@example.com",
		"x-aisphere-external-name":         "admin",
		"x-aisphere-external-display-name": "管理员",
		"x-aisphere-external-phone":        "46192823473",
		"x-aisphere-external-owner":        "aisphere",
		"x-aisphere-external-scope":        "openid profile email",
	})
	if !ok {
		t.Fatal("expected principal from casdoor gateway claim headers")
	}
	if p.SubjectID != "496333c7-7acc-4717-8596-056544fc0a68" {
		t.Fatalf("unexpected subject id: %q", p.SubjectID)
	}
	if p.ExternalID != "496333c7-7acc-4717-8596-056544fc0a68" {
		t.Fatalf("unexpected external id: %q", p.ExternalID)
	}
	if p.TenantID != "aisphere" || p.OrgID != "aisphere" {
		t.Fatalf("owner should populate tenant/org: %#v", p)
	}
	if p.Username != "admin" || p.Name != "管理员" || p.Phone != "46192823473" {
		t.Fatalf("profile claims not preserved: %#v", p)
	}
	if !p.HasScope("openid") || !p.HasScope("profile") || !p.HasScope("email") {
		t.Fatalf("space-separated oauth scopes not parsed: %#v", p.Scopes)
	}
}

func TestPrincipalFromGatewayClaimHeadersPrefersInternalProjection(t *testing.T) {
	p, ok := PrincipalFromTrustedHeaders(map[string]string{
		"x-aisphere-external-sub": "casdoor/admin",
		"x-aisphere-user-id":      "user-internal-1",
	})
	if !ok {
		t.Fatal("expected principal from projected user id")
	}
	if p.SubjectID != "user-internal-1" {
		t.Fatalf("expected internal user id as subject, got %q", p.SubjectID)
	}
	if p.ExternalID != "casdoor/admin" {
		t.Fatalf("expected external sub preserved, got %q", p.ExternalID)
	}
}

func TestPrincipalFromGatewayClaimHeadersRequiresSubject(t *testing.T) {
	if _, ok := PrincipalFromTrustedHeaders(map[string]string{
		"x-aisphere-external-email": "admin@example.com",
	}); ok {
		t.Fatal("expected empty subject to be rejected")
	}
}
