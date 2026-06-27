package authn

import (
	"context"
	"testing"
)

func TestBearerCredential(t *testing.T) {
	cred, ok := BearerCredential("Bearer token-123")
	if !ok {
		t.Fatal("expected bearer credential")
	}
	if cred.Scheme != CredentialBearer || cred.Token != "token-123" {
		t.Fatalf("unexpected credential: %#v", cred)
	}
}

func TestContextPrincipal(t *testing.T) {
	ctx := ContextWithPrincipal(context.Background(), Principal{SubjectID: "u_1"})
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("missing principal")
	}
	if principal.SubjectType != SubjectTypeUser || principal.SubjectID != "u_1" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
}
