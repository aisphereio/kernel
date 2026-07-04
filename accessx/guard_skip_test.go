package accessx

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

// fakeAuthenticator implements authn.Authenticator for testing.
type fakeAuthenticator struct {
	principal authn.Principal
	err       error
}

func (f fakeAuthenticator) Authenticate(_ context.Context, _ authn.Credential) (authn.Principal, error) {
	return f.principal, f.err
}

// fakeAuthorizer implements authz.Authorizer for testing.
type fakeAuthorizer struct {
	decision authz.Decision
	err      error
}

func (f fakeAuthorizer) Check(_ context.Context, _ authz.CheckRequest) (authz.Decision, error) {
	return f.decision, f.err
}

// fakeAuditRecorder records audit events for verification.
type fakeAuditRecorder struct {
	records []auditx.Record
}

func (f *fakeAuditRecorder) Record(_ context.Context, record auditx.Record) error {
	f.records = append(f.records, record)
	return nil
}

func TestGuardRequire_SkipAll(t *testing.T) {
	// SkipAll should skip both authn and authz — no authenticator needed.
	guard := New(nil, authz.DenyAll(), auditx.Noop())
	principal, err := guard.Require(context.Background(), Check{
		SkipPolicy: SkipAll,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !principal.IsAnonymous() {
		t.Fatal("expected anonymous principal for SkipAll")
	}
}

func TestGuardRequire_SkipAuthz(t *testing.T) {
	// SkipAuthz should still authenticate but skip the authz check.
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	// Use DenyAll authorizer — if SkipAuthz works, it should still pass.
	guard := New(authn, authz.DenyAll(), auditx.Noop())
	principal, err := guard.Require(context.Background(), Check{
		SkipPolicy: SkipAuthz,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.SubjectID != "u1" {
		t.Fatalf("expected principal u1, got %q", principal.SubjectID)
	}
}

func TestGuardRequire_SkipAuthz_WithAuthnFailure(t *testing.T) {
	// SkipAuthz should still fail if authentication fails.
	authn := fakeAuthenticator{
		err: authn.ErrUnauthenticated("test error"),
	}
	guard := New(authn, authz.AllowAllForDevOnly(), auditx.Noop())
	_, err := guard.Require(context.Background(), Check{
		SkipPolicy: SkipAuthz,
	})
	if err == nil {
		t.Fatal("expected error for failed authentication with SkipAuthz")
	}
}

func TestGuardRequire_SkipDefault_PerformsAuthz(t *testing.T) {
	// SkipDefault (or empty) should perform the full authz check.
	fakeAuthn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	fakeAuthz := fakeAuthorizer{
		decision: authz.Deny("test deny"),
	}
	guard := New(fakeAuthn, fakeAuthz, auditx.Noop())
	_, err := guard.Require(context.Background(), Check{
		Permission: "read",
		Resource:   authz.ObjectRef{Type: "doc", ID: "1"},
	})
	if err == nil {
		t.Fatal("expected denied error for SkipDefault with Deny authorizer")
	}
}

func TestGuardRequire_SkipDefault_Allows(t *testing.T) {
	fakeAuthn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	fakeAuthz := fakeAuthorizer{
		decision: authz.Allow("allowed"),
	}
	guard := New(fakeAuthn, fakeAuthz, auditx.Noop())
	_, err := guard.Require(context.Background(), Check{
		Permission: "read",
		Resource:   authz.ObjectRef{Type: "doc", ID: "1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGuardRequire_BackwardCompat_AllowAll(t *testing.T) {
	// Deprecated AllowAll should still work.
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), auditx.Noop())
	principal, err := guard.Require(context.Background(), Check{
		AllowAll: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.SubjectID != "u1" {
		t.Fatalf("expected principal u1, got %q", principal.SubjectID)
	}
}

func TestGuardRequire_BackwardCompat_AuthzModeNone(t *testing.T) {
	// Deprecated AuthzModeNone should still work.
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), auditx.Noop())
	principal, err := guard.Require(context.Background(), Check{
		AuthzMode: AuthzModeNone,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.SubjectID != "u1" {
		t.Fatalf("expected principal u1, got %q", principal.SubjectID)
	}
}

func TestGuardRequire_SkipPolicyTakesPriority(t *testing.T) {
	// When SkipPolicy is set, it should take priority over deprecated fields.
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), auditx.Noop())
	// SkipPolicy=SkipAuthz should skip authz even though AuthzMode is default.
	principal, err := guard.Require(context.Background(), Check{
		SkipPolicy: SkipAuthz,
		AuthzMode:  AuthzModeDefault,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if principal.SubjectID != "u1" {
		t.Fatalf("expected principal u1, got %q", principal.SubjectID)
	}
}

func TestGuardCan_SkipAll(t *testing.T) {
	guard := New(nil, authz.DenyAll(), auditx.Noop())
	allowed, err := guard.Can(context.Background(), Check{
		SkipPolicy: SkipAll,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed for SkipAll")
	}
}

func TestGuardCan_SkipAuthz(t *testing.T) {
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), nil)
	allowed, err := guard.Can(context.Background(), Check{
		SkipPolicy: SkipAuthz,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed for SkipAuthz")
	}
}

func TestGuardCan_BackwardCompat(t *testing.T) {
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), nil)
	// AllowAll
	allowed, err := guard.Can(context.Background(), Check{AllowAll: true})
	if err != nil || !allowed {
		t.Fatal("expected allowed for AllowAll")
	}
	// AuthzModeNone
	allowed, err = guard.Can(context.Background(), Check{AuthzMode: AuthzModeNone})
	if err != nil || !allowed {
		t.Fatal("expected allowed for AuthzModeNone")
	}
}

func TestGuardRequire_RecordsAuditOnSkip(t *testing.T) {
	audit := &fakeAuditRecorder{}
	authn := fakeAuthenticator{
		principal: authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser},
	}
	guard := New(authn, authz.DenyAll(), audit)
	_, err := guard.Require(context.Background(), Check{
		SkipPolicy:  SkipAuthz,
		AuditAction: "test.action",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audit.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audit.records))
	}
	if audit.records[0].Action != "test.action" {
		t.Fatalf("expected action 'test.action', got %q", audit.records[0].Action)
	}
	if audit.records[0].Result != auditx.ResultSuccess {
		t.Fatalf("expected result 'success', got %q", audit.records[0].Result)
	}
}

func TestGuardRequire_SkipAllNoAudit(t *testing.T) {
	// SkipAll should not record audit (no principal, no action).
	audit := &fakeAuditRecorder{}
	guard := New(nil, authz.DenyAll(), audit)
	_, err := guard.Require(context.Background(), Check{
		SkipPolicy:  SkipAll,
		AuditAction: "test.action",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SkipAll still records audit (with anonymous principal).
	if len(audit.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(audit.records))
	}
}