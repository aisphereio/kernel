// Command check_gateway_principal tests the Gateway claim headers → Principal path.
//
// It simulates Envoy Gateway claimToHeaders injection and verifies that
// PrincipalFromTrustedHeaders correctly reconstructs all Principal fields
// including the newly supported external-id, external-owner, external-display-name,
// external-phone, external-scope headers.
//
// Usage:
//
//	go run ./examples/authn-casdoor/cmd/check_gateway_principal/
package main

import (
	"fmt"
	"os"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
)

func main() {
	// Simulate Envoy Gateway claimToHeaders injection
	// This matches the Envoy config after our fix:
	//   sub  → x-aisphere-external-sub
	//   iss  → x-aisphere-external-issuer
	//   email → x-aisphere-external-email
	//   name → x-aisphere-external-name
	//   displayName → x-aisphere-external-display-name
	//   phone → x-aisphere-external-phone
	//   owner → x-aisphere-external-owner
	//   id   → x-aisphere-external-id
	//   scope → x-aisphere-external-scope
	//   sub  → x-aisphere-principal
	//   id   → x-aisphere-user-id
	//   owner → x-aisphere-org-id
	headers := map[string]string{
		"x-aisphere-external-sub":           "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-external-issuer":       "https://casdoor.weagent.cc:30723",
		"x-aisphere-external-email":        "admin@example.com",
		"x-aisphere-external-name":         "admin",
		"x-aisphere-external-display-name": "管理员",
		"x-aisphere-external-phone":        "46192823473",
		"x-aisphere-external-owner":       "aisphere",
		"x-aisphere-external-id":          "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-external-scope":       "openid profile email",
		"x-aisphere-principal":            "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-user-id":              "496333c7-7acc-4717-8596-056544fc0a68",
		"x-aisphere-org-id":               "aisphere",
	}

	fmt.Println("=== Test: principalFromGatewayClaimHeaders ===")
	p, ok := authn.PrincipalFromTrustedHeaders(headers)
	if !ok {
		fmt.Println("FAIL: PrincipalFromTrustedHeaders returned false")
		os.Exit(1)
	}

	fmt.Printf("  SubjectID:   %q\n", p.SubjectID)
	fmt.Printf("  SubjectType: %q\n", p.SubjectType)
	fmt.Printf("  Provider:    %q\n", p.Provider)
	fmt.Printf("  ExternalID:  %q\n", p.ExternalID)
	fmt.Printf("  Issuer:      %q\n", p.Issuer)
	fmt.Printf("  TenantID:    %q\n", p.TenantID)
	fmt.Printf("  OrgID:       %q\n", p.OrgID)
	fmt.Printf("  ProjectID:   %q\n", p.ProjectID)
	fmt.Printf("  Username:    %q\n", p.Username)
	fmt.Printf("  Name:        %q\n", p.Name)
	fmt.Printf("  Email:       %q\n", p.Email)
	fmt.Printf("  Phone:       %q\n", p.Phone)
	fmt.Printf("  Roles:       %v\n", p.Roles)
	fmt.Printf("  Groups:      %v\n", p.Groups)
	fmt.Printf("  Scopes:      %v\n", p.Scopes)
	fmt.Printf("  AuthMethod:  %q\n", p.AuthMethod)
	fmt.Printf("  Attributes:  %v\n", p.Attributes)
	fmt.Printf("  IssuedAt:    %v\n", p.IssuedAt)
	fmt.Printf("  ExpiresAt:   %v\n", p.ExpiresAt)

	// Verify key fields
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"SubjectID", p.SubjectID, "496333c7-7acc-4717-8596-056544fc0a68"},
		{"ExternalID", p.ExternalID, "496333c7-7acc-4717-8596-056544fc0a68"},
		{"Issuer", p.Issuer, "https://casdoor.weagent.cc:30723"},
		{"OrgID", p.OrgID, "aisphere"},
		{"TenantID", p.TenantID, "aisphere"},
		{"Username", p.Username, "admin"},
		{"Name", p.Name, "管理员"},
		{"Email", p.Email, "admin@example.com"},
		{"Phone", p.Phone, "46192823473"},
		{"AuthMethod", p.AuthMethod, "oidc"},
	}
	allOK := true
	for _, c := range checks {
		if c.got != c.want {
			fmt.Printf("  FAIL: %s = %q, want %q\n", c.name, c.got, c.want)
			allOK = false
		}
	}
	if allOK {
		fmt.Println("  All field checks PASSED")
	}

	// Verify scopes parsing (space-separated)
	if len(p.Scopes) != 3 || p.Scopes[0] != "openid" || p.Scopes[1] != "profile" || p.Scopes[2] != "email" {
		fmt.Printf("  FAIL: Scopes = %v, want [openid profile email]\n", p.Scopes)
		allOK = false
	} else {
		fmt.Println("  Scopes parsing PASSED (space-separated)")
	}

	// Verify Attributes
	if p.Attributes["gateway.external_sub"] != "496333c7-7acc-4717-8596-056544fc0a68" {
		fmt.Printf("  FAIL: gateway.external_sub = %v\n", p.Attributes["gateway.external_sub"])
		allOK = false
	}
	if p.Attributes["gateway.external_owner"] != "aisphere" {
		fmt.Printf("  FAIL: gateway.external_owner = %v\n", p.Attributes["gateway.external_owner"])
		allOK = false
	}
	if p.Attributes["gateway.external_displayName"] != "管理员" {
		fmt.Printf("  FAIL: gateway.external_displayName = %v\n", p.Attributes["gateway.external_displayName"])
		allOK = false
	}

	fmt.Println()
	fmt.Println("=== Test 2: contextx.Principal mirror (lossless) ===")
	cp := contextx.FromAuthnPrincipal(p)
	if cp == nil {
		fmt.Println("FAIL: FromAuthnPrincipal returned nil")
		os.Exit(1)
	}
	fmt.Printf("  SubjectID:   %q\n", cp.SubjectID)
	fmt.Printf("  OrgID:       %q\n", cp.OrgID)
	fmt.Printf("  Username:    %q\n", cp.Username)
	fmt.Printf("  Name:        %q\n", cp.Name)
	fmt.Printf("  Email:       %q\n", cp.Email)
	fmt.Printf("  Phone:       %q\n", cp.Phone)
	fmt.Printf("  ExternalID:  %q\n", cp.ExternalID)
	fmt.Printf("  Issuer:      %q\n", cp.Issuer)
	fmt.Printf("  Scopes:      %v\n", cp.Scopes)
	fmt.Printf("  Attributes:  %v\n", cp.Attributes)

	// Verify round-trip: authn → contextx → authn
	p2 := contextx.ToAuthnPrincipal(cp)
	if p2.SubjectID != p.SubjectID || p2.OrgID != p.OrgID || p2.Username != p.Username || p2.Email != p.Email || p2.Phone != p.Phone {
		fmt.Printf("FAIL: round-trip mismatch\n")
		fmt.Printf("  original: SubjectID=%q OrgID=%q Username=%q Email=%q Phone=%q\n",
			p.SubjectID, p.OrgID, p.Username, p.Email, p.Phone)
		fmt.Printf("  restored: SubjectID=%q OrgID=%q Username=%q Email=%q Phone=%q\n",
			p2.SubjectID, p2.OrgID, p2.Username, p2.Email, p2.Phone)
		allOK = false
	} else {
		fmt.Println("  Round-trip authn → contextx → authn PASSED")
	}

	fmt.Println()
	if allOK {
		fmt.Println("=== ALL TESTS PASSED ===")
	} else {
		fmt.Println("=== SOME TESTS FAILED ===")
		os.Exit(1)
	}
}