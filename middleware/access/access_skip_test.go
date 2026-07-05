package access

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/contextx"
	transport "github.com/aisphereio/kernel/transportx"
)

func TestServerWithSkipPolicyResolver_SkipAll(t *testing.T) {
	// SkipAll should pass through without calling the resolver; Guard.Require still records audit.
	guard := accessx.New(nil, authz.DenyAll(), auditx.Noop())
	resolverCalled := false
	mw := Server(guard,
		WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
			resolverCalled = true
			return accessx.Check{Permission: "read", Resource: authz.ObjectRef{Type: "doc", ID: "1"}, AuditAction: "doc.access"}, true, nil
		}),
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			return accessx.SkipAll
		}),
	)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = transport.NewServerContext(ctx, fakeTransport{operation: "/doc.Service/Get"})
	handlerCalled := false
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
	if resolverCalled {
		t.Fatal("expected resolver NOT to be called for SkipAll")
	}
}

func TestServerSkipPolicyResolver_SkipAuthz(t *testing.T) {
	// SkipAuthz from config should authenticate and audit through Guard.Require,
	// but Guard.Require should skip the authz check.
	store := authz.NewMemoryRelationshipStore()
	audit := auditx.NewMemoryStore()
	guard := accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
	mw := Server(guard,
		WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
			return accessx.Check{Permission: "read", Resource: authz.ObjectRef{Type: "doc", ID: "1"}, AuditAction: "doc.access"}, true, nil
		}),
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			return accessx.SkipAuthz
		}),
	)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = authn.ContextWithPrincipal(ctx, authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser})
	ctx = transport.NewServerContext(ctx, fakeTransport{req: header{}, reply: header{}, operation: "/doc.Service/Get"})
	handlerCalled := false
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestServerSkipPolicyResolver_Default(t *testing.T) {
	// SkipDefault should fall through to normal resolver + guard check.
	store := authz.NewMemoryRelationshipStore()
	if _, err := store.WriteRelationships(context.Background(), authz.Relationship{
		Resource: authz.ObjectRef{Type: "doc", ID: "1"}, Relation: "read", Subject: authz.SubjectRef{Type: authn.SubjectTypeUser, ID: "u1"},
	}); err != nil {
		t.Fatal(err)
	}
	audit := auditx.NewMemoryStore()
	guard := accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
	mw := Server(guard,
		WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
			return accessx.Check{Permission: "read", Resource: authz.ObjectRef{Type: "doc", ID: "1"}, AuditAction: "doc.access"}, true, nil
		}),
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			return accessx.SkipDefault
		}),
	)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = authn.ContextWithPrincipal(ctx, authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser})
	ctx = transport.NewServerContext(ctx, fakeTransport{req: header{}, reply: header{}, operation: "/doc.Service/Get"})
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerSkipPolicyResolver_NoResolver(t *testing.T) {
	// When no resolver is configured, SkipPolicyResolver should still work.
	guard := accessx.New(nil, authz.DenyAll(), auditx.Noop())
	mw := Server(guard,
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			return accessx.SkipAll
		}),
	)
	ctx := context.Background()
	handlerCalled := false
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestServerSkipPolicyResolver_AppliesToCheck(t *testing.T) {
	// When the resolver returns a Check without SkipPolicy set, the
	// SkipPolicyResolver should apply the policy to the Check before
	// calling Guard.Require.
	store := authz.NewMemoryRelationshipStore()
	audit := auditx.NewMemoryStore()
	guard := accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
	mw := Server(guard,
		WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
			// Resolver does NOT set SkipPolicy — it should be applied by the middleware.
			return accessx.Check{Permission: "read", Resource: authz.ObjectRef{Type: "doc", ID: "1"}, AuditAction: "doc.access"}, true, nil
		}),
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			return accessx.SkipAuthz
		}),
	)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = authn.ContextWithPrincipal(ctx, authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser})
	ctx = transport.NewServerContext(ctx, fakeTransport{req: header{}, reply: header{}, operation: "/doc.Service/Get"})
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerSkipPolicyResolver_ResolverSkipTakesPriority(t *testing.T) {
	// When the resolver explicitly sets SkipPolicy on the Check, the
	// SkipPolicyResolver should NOT override it.
	store := authz.NewMemoryRelationshipStore()
	audit := auditx.NewMemoryStore()
	guard := accessx.New(nil, authz.NewMemoryAuthorizer(store), audit)
	mw := Server(guard,
		WithResolver(func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
			// Resolver explicitly sets SkipPolicy=SkipAuthz.
			return accessx.Check{
				SkipPolicy:  accessx.SkipAuthz,
				Permission:  "read",
				Resource:    authz.ObjectRef{Type: "doc", ID: "1"},
				AuditAction: "doc.access",
			}, true, nil
		}),
		WithSkipPolicyResolver(func(operation string) accessx.SkipPolicy {
			// This would return SkipDefault, but the resolver already set SkipAuthz.
			return accessx.SkipDefault
		}),
	)
	ctx := context.Background()
	ctx = contextx.WithRequestID(ctx, "req-1")
	ctx = authn.ContextWithPrincipal(ctx, authn.Principal{SubjectID: "u1", SubjectType: authn.SubjectTypeUser})
	ctx = transport.NewServerContext(ctx, fakeTransport{req: header{}, reply: header{}, operation: "/doc.Service/Get"})
	_, err := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
