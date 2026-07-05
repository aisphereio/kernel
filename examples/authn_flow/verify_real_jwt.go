//go:build ignore

// This program verifies a real Casdoor JWT using the kernel authn/oidcx verifier.
// It connects to the real Casdoor OIDC discovery + JWKS endpoints.
//
// Usage:
//
//	go run ./examples/authn_flow/verify_real_jwt.go <access_token>
//
// The token can be obtained via:
//
//	curl -s -X POST "http://36.137.200.194:30082/api/login/oauth/access_token" \
//	  -H "Content-Type: application/x-www-form-urlencoded" \
//	  -d "grant_type=password" \
//	  -d "client_id=bbdcfc272e2b990cb923" \
//	  -d "client_secret=c4d351406d40251b267624328e50e8a1f7352a65" \
//	  -d "username=user" \
//	  -d "password=password" \
//	  -d "scope=openid profile email"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/oidcx"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./examples/authn_flow/verify_real_jwt.go <access_token>")
		os.Exit(1)
	}
	token := os.Args[1]

	cfg := oidcx.Config{
		Provider:      "casdoor",
		Issuer:        "http://36.137.200.194:30082",
		DiscoveryURL:  "http://36.137.200.194:30082/.well-known/openid-configuration",
		JWKSURL:       "http://36.137.200.194:30082/.well-known/jwks",
		Audience:      []string{"bbdcfc272e2b990cb923"},
		AllowedOwners: []string{"aisphere"},
		AllowedAlgs:   []string{"RS256", "RS512", "ES256", "ES512"},
	}

	verifier, err := oidcx.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to create verifier: %v\n", err)
		os.Exit(1)
	}

	// Step 1: Verify via Authenticate interface (Gateway path)
	fmt.Println("=== Step 1: Gateway-style verification (oidcx.Verifier.Authenticate) ===")
	principal, err := verifier.Authenticate(context.Background(), authn.Credential{
		Scheme: authn.CredentialBearer,
		Token:  token,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: Authenticate failed: %v\n", err)
		os.Exit(1)
	}
	printPrincipal("Gateway Principal", principal)

	// Step 2: Verify via VerifyToken interface (backend re-verify)
	fmt.Println("\n=== Step 2: Backend re-verify (oidcx.Verifier.VerifyToken) ===")
	principal2, err := verifier.VerifyToken(context.Background(), authn.VerifyTokenRequest{
		Token: token,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: VerifyToken failed: %v\n", err)
		os.Exit(1)
	}
	printPrincipal("Backend Principal", principal2)

	// Step 3: Verify trusted headers injection/roundtrip
	fmt.Println("\n=== Step 3: Trusted headers injection + roundtrip ===")
	headers := map[string]string{}
	authn.InjectTrustedHeaders(headers, principal)
	fmt.Println("Injected headers:")
	for k, v := range headers {
		fmt.Printf("  %s: %s\n", k, v)
	}

	reconstructed, ok := authn.PrincipalFromTrustedHeaders(headers)
	if !ok {
		fmt.Fprintf(os.Stderr, "FAIL: PrincipalFromTrustedHeaders failed\n")
		os.Exit(1)
	}
	printJSON("Reconstructed Principal from headers", reconstructed)

	// Step 4: Verify spoofed headers are stripped
	fmt.Println("\n=== Step 4: Spoofed header stripping ===")
	spoofed := map[string]string{
		"X-Aisphere-Subject":  "attacker",
		"X-Aisphere-Username": "mallory",
		"Authorization":       "Bearer " + token,
	}
	authn.StripTrustedHeaders(spoofed)
	if v, ok := spoofed["X-Aisphere-Subject"]; ok {
		fmt.Fprintf(os.Stderr, "FAIL: X-Aisphere-Subject was not stripped: %s\n", v)
		os.Exit(1)
	}
	fmt.Println("  ✅ Spoofed X-Aisphere-Subject stripped")
	if v, ok := spoofed["X-Aisphere-Username"]; ok {
		fmt.Fprintf(os.Stderr, "FAIL: X-Aisphere-Username was not stripped: %s\n", v)
		os.Exit(1)
	}
	fmt.Println("  ✅ Spoofed X-Aisphere-Username stripped")
	if _, ok := spoofed["Authorization"]; !ok {
		fmt.Fprintf(os.Stderr, "FAIL: Authorization header was incorrectly stripped\n")
		os.Exit(1)
	}
	fmt.Println("  ✅ Authorization header preserved")

	fmt.Println("\n🎉 All checks passed! Real Casdoor JWT verified successfully via oidcx.")
}

func printJSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("%s:\n%s\n", label, string(b))
}

func printPrincipal(label string, p authn.Principal) {
	fmt.Printf("%s:\n", label)
	fmt.Printf("  SubjectID:   %s\n", p.SubjectID)
	fmt.Printf("  SubjectType: %s\n", p.SubjectType)
	fmt.Printf("  Provider:    %s\n", p.Provider)
	fmt.Printf("  ExternalID:  %s\n", p.ExternalID)
	fmt.Printf("  Issuer:      %s\n", p.Issuer)
	fmt.Printf("  Audience:    %v\n", p.Audience)
	fmt.Printf("  OrgID:       %s\n", p.OrgID)
	fmt.Printf("  TenantID:    %s\n", p.TenantID)
	fmt.Printf("  Username:    %s\n", p.Username)
	fmt.Printf("  Name:        %s\n", p.Name)
	fmt.Printf("  Email:       %s\n", p.Email)
	fmt.Printf("  Groups:      %v\n", p.Groups)
	fmt.Printf("  Roles:       %v\n", p.Roles)
	fmt.Printf("  Scopes:      %v\n", p.Scopes)
	fmt.Printf("  AuthMethod:  %s\n", p.AuthMethod)
	fmt.Printf("  IsAuthenticated: %v\n", p.IsAuthenticated())
}
