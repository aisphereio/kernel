package accessx

import (
	"testing"
)

func TestNewSkipPolicyResolver_Default(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{})
	policy := resolver("SomeOperation")
	if policy != SkipDefault {
		t.Fatalf("expected SkipDefault, got %q", policy)
	}
}

func TestNewSkipPolicyResolver_SkipOperations(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{
		SkipOperations: []string{"GetMe", "CreateOrganization"},
	})
	tests := []struct {
		operation string
		want      SkipPolicy
	}{
		{"GetMe", SkipAuthz},
		{"CreateOrganization", SkipAuthz},
		{"iam.v1.IAMAuthService/GetMe", SkipAuthz},
		{"/iam.v1.IAMAuthService/GetMe", SkipAuthz},
		{"SomeOtherOp", SkipDefault},
		{"", SkipDefault},
	}
	for _, tt := range tests {
		got := resolver(tt.operation)
		if got != tt.want {
			t.Errorf("resolver(%q) = %q, want %q", tt.operation, got, tt.want)
		}
	}
}

func TestNewSkipPolicyResolver_PublicOperations(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{
		PublicOperations: []string{"Login", "Exchange", "/healthz"},
	})
	tests := []struct {
		operation string
		want      SkipPolicy
	}{
		{"Login", SkipAll},
		{"Exchange", SkipAll},
		{"/healthz", SkipAll},
		{"SomeOtherOp", SkipDefault},
	}
	for _, tt := range tests {
		got := resolver(tt.operation)
		if got != tt.want {
			t.Errorf("resolver(%q) = %q, want %q", tt.operation, got, tt.want)
		}
	}
}

func TestNewSkipPolicyResolver_PublicOverridesSkip(t *testing.T) {
	// PublicOperations should take priority over SkipOperations.
	resolver := NewSkipPolicyResolver(AccessConfig{
		PublicOperations: []string{"Login"},
		SkipOperations:   []string{"Login", "GetMe"},
	})
	if policy := resolver("Login"); policy != SkipAll {
		t.Fatalf("expected SkipAll (public takes priority), got %q", policy)
	}
	if policy := resolver("GetMe"); policy != SkipAuthz {
		t.Fatalf("expected SkipAuthz, got %q", policy)
	}
}

func TestNewSkipPolicyResolver_WildcardPatterns(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{
		PublicOperations: []string{"/v1/authn/*"},
		SkipOperations:   []string{"/grpc.health.v1.Health/*"},
	})
	tests := []struct {
		operation string
		want      SkipPolicy
	}{
		{"/v1/authn/login", SkipAll},
		{"/v1/authn/logout", SkipAll},
		{"/v1/authn/exchange", SkipAll},
		{"/grpc.health.v1.Health/Check", SkipAuthz},
		{"/grpc.health.v1.Health/Watch", SkipAuthz},
		{"/v1/skills/list", SkipDefault},
	}
	for _, tt := range tests {
		got := resolver(tt.operation)
		if got != tt.want {
			t.Errorf("resolver(%q) = %q, want %q", tt.operation, got, tt.want)
		}
	}
}

func TestNewSkipPolicyResolver_LegacyAllowAll(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{
		AllowAllOperations: []string{"GetMe", "CreateOrganization"},
	})
	tests := []struct {
		operation string
		want      SkipPolicy
	}{
		{"GetMe", SkipAuthz},
		{"CreateOrganization", SkipAuthz},
		{"SomeOtherOp", SkipDefault},
	}
	for _, tt := range tests {
		got := resolver(tt.operation)
		if got != tt.want {
			t.Errorf("resolver(%q) = %q, want %q", tt.operation, got, tt.want)
		}
	}
}

func TestNewSkipPolicyResolver_EmptyConfig(t *testing.T) {
	resolver := NewSkipPolicyResolver(AccessConfig{
		SkipOperations:     []string{},
		PublicOperations:   []string{},
		AllowAllOperations: []string{},
	})
	if policy := resolver("anything"); policy != SkipDefault {
		t.Fatalf("expected SkipDefault for empty config, got %q", policy)
	}
}

func TestMatchOperation_Exact(t *testing.T) {
	set := map[string]bool{"GetMe": true, "CreateOrg": true}
	if !matchOperation(set, "GetMe") {
		t.Fatal("expected match for GetMe")
	}
	if !matchOperation(set, "CreateOrg") {
		t.Fatal("expected match for CreateOrg")
	}
	if matchOperation(set, "GetMe2") {
		t.Fatal("expected no match for GetMe2")
	}
	if matchOperation(nil, "GetMe") {
		t.Fatal("expected no match for nil set")
	}
	if matchOperation(set, "") {
		t.Fatal("expected no match for empty operation")
	}
}

func TestMatchOperation_Wildcard(t *testing.T) {
	set := make(map[string]bool)
	set["/v1/authn/*"] = true
	set["/grpc.health.*"] = true

	tests := []struct {
		op   string
		want bool
	}{
		{"/v1/authn/login", true},
		{"/v1/authn/logout", true},
		{"/grpc.health.v1.Health/Check", true},
		{"/v1/skills/list", false},
		{"", false},
	}
	for _, tt := range tests {
		got := matchOperation(set, tt.op)
		if got != tt.want {
			t.Errorf("matchOperation(%q) = %v, want %v", tt.op, got, tt.want)
		}
	}
}

func TestIsSkipPolicy(t *testing.T) {
	tests := []struct {
		p    SkipPolicy
		want bool
	}{
		{SkipDefault, false},
		{SkipAuthz, true},
		{SkipAll, true},
		{SkipPolicy("unknown"), false},
	}
	for _, tt := range tests {
		got := IsSkipPolicy(tt.p)
		if got != tt.want {
			t.Errorf("IsSkipPolicy(%q) = %v, want %v", tt.p, got, tt.want)
		}
	}
}