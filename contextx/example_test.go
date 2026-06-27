package contextx_test

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/contextx"
)

// ============================================================================
// RequestID / TraceID EXAMPLES
// ============================================================================

func ExampleWithRequestID() {
	ctx := contextx.WithRequestID(context.Background(), "req_abc")
	fmt.Println(contextx.RequestIDFromContext(ctx))
	// Output: req_abc
}

func ExampleRequestIDFromContext() {
	// With value
	ctx := contextx.WithRequestID(context.Background(), "req_abc")
	fmt.Println(contextx.RequestIDFromContext(ctx))

	// Without value
	fmt.Println(contextx.RequestIDFromContext(context.Background()))

	// nil context (safe)
	fmt.Println(contextx.RequestIDFromContext(nil) == "")
	// Output:
	// req_abc
	//
	// true
}

func ExampleWithTraceID() {
	ctx := contextx.WithTraceID(context.Background(), "trace_xyz")
	fmt.Println(contextx.TraceIDFromContext(ctx))
	// Output: trace_xyz
}

func ExampleTraceIDFromContext_nil() {
	// nil-safe: returns "" instead of panicking
	fmt.Println(contextx.TraceIDFromContext(nil) == "")
	// Output: true
}

// ============================================================================
// Principal EXAMPLES
// ============================================================================

func ExampleWithPrincipal() {
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{
		SubjectID:  "u_123",
		TenantID:   "t_acme",
		Roles:      []string{"admin", "viewer"},
		Scopes:     []string{"skill:read", "skill:write"},
		AuthMethod: "oauth",
	})

	p := contextx.PrincipalFromContext(ctx)
	fmt.Println(p.SubjectID)
	fmt.Println(p.TenantID)
	fmt.Println(p.AuthMethod)
	// Output:
	// u_123
	// t_acme
	// oauth
}

func ExamplePrincipalFromContext() {
	// With principal
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{SubjectID: "u_123"})
	fmt.Println(contextx.PrincipalFromContext(ctx).SubjectID)

	// Without principal (returns nil)
	fmt.Println(contextx.PrincipalFromContext(context.Background()))

	// nil context (returns nil, never panics)
	fmt.Println(contextx.PrincipalFromContext(nil))
	// Output:
	// u_123
	// <nil>
	// <nil>
}

func ExamplePrincipal_IsAuthenticated() {
	authed := &contextx.Principal{SubjectID: "u_123"}
	anon := &contextx.Principal{}
	var nilP *contextx.Principal

	fmt.Println(authed.IsAuthenticated())
	fmt.Println(anon.IsAuthenticated())
	fmt.Println(nilP.IsAuthenticated())
	// Output:
	// true
	// false
	// false
}

func ExamplePrincipal_HasRole() {
	p := &contextx.Principal{Roles: []string{"admin", "viewer"}}
	fmt.Println(p.HasRole("admin"))
	fmt.Println(p.HasRole("superadmin"))
	// Output:
	// true
	// false
}

func ExamplePrincipal_HasScope() {
	p := &contextx.Principal{Scopes: []string{"skill:read", "skill:write"}}
	fmt.Println(p.HasScope("skill:read"))
	fmt.Println(p.HasScope("skill:delete"))
	// Output:
	// true
	// false
}

func ExampleSubjectIDFromContext() {
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{SubjectID: "u_123"})
	fmt.Println(contextx.SubjectIDFromContext(ctx))

	// Empty when no principal
	fmt.Println(contextx.SubjectIDFromContext(context.Background()))
	// Output:
	// u_123
	//
}

// ============================================================================
// Tenant EXAMPLES
// ============================================================================

func ExampleWithTenant() {
	ctx := contextx.WithTenant(context.Background(), "t_acme")
	fmt.Println(contextx.TenantFromContext(ctx))
	// Output: t_acme
}

func ExampleTenantFromContext_fallbackToPrincipal() {
	// When no explicit tenant, falls back to Principal.TenantID
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{
		SubjectID: "u_123",
		TenantID:  "t_from_principal",
	})
	fmt.Println(contextx.TenantFromContext(ctx))
	// Output: t_from_principal
}

func ExampleTenantFromContext_explicitWins() {
	// Explicit WithTenant wins over Principal.TenantID
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{
		TenantID: "t_from_principal",
	})
	ctx = contextx.WithTenant(ctx, "t_explicit")
	fmt.Println(contextx.TenantFromContext(ctx))
	// Output: t_explicit
}

// ============================================================================
// Logger EXAMPLES
// ============================================================================

// fakeLogger satisfies contextx.Logger for examples (in real code, pass logx.Logger)
type fakeLogger struct{}

func (fakeLogger) Debug(string, ...contextx.Field)               {}
func (fakeLogger) Info(string, ...contextx.Field)                {}
func (fakeLogger) Warn(string, ...contextx.Field)                {}
func (fakeLogger) Error(string, ...contextx.Field)               {}
func (l fakeLogger) With(...contextx.Field) contextx.Logger      { return l }
func (l fakeLogger) Named(string) contextx.Logger                { return l }
func (l fakeLogger) WithContext(context.Context) contextx.Logger { return l }
func (fakeLogger) Enabled(contextx.LogLevel) bool                { return true }
func (fakeLogger) Sync() error                                   { return nil }

func ExampleWithLogger() {
	ctx := contextx.WithLogger(context.Background(), fakeLogger{})
	fmt.Println(contextx.LoggerFromContext(ctx) != nil)
	// Output: true
}

func ExampleLoggerFromContext() {
	// With logger
	ctx := contextx.WithLogger(context.Background(), fakeLogger{})
	fmt.Println(contextx.LoggerFromContext(ctx) != nil)

	// Without logger (returns nil)
	fmt.Println(contextx.LoggerFromContext(context.Background()))

	// nil context (returns nil, never panics)
	fmt.Println(contextx.LoggerFromContext(nil))
	// Output:
	// true
	// <nil>
	// <nil>
}

func ExampleLoggerFromContextOr() {
	fallback := fakeLogger{}

	// Returns attached logger when present
	ctx := contextx.WithLogger(context.Background(), fakeLogger{})
	fmt.Println(contextx.LoggerFromContextOr(ctx, fallback) != nil)

	// Returns fallback when no attached logger
	fmt.Println(contextx.LoggerFromContextOr(context.Background(), fallback) != nil)

	// Returns nil when neither (safe)
	fmt.Println(contextx.LoggerFromContextOr(context.Background(), nil))
	// Output:
	// true
	// true
	// <nil>
}

// ============================================================================
// Generic With/From EXAMPLES
// ============================================================================

func ExampleWith() {
	ctx := contextx.With(context.Background(), "custom_key", "custom_value")
	fmt.Println(contextx.From(ctx, "custom_key"))
	// Output: custom_value
}

func ExampleFrom() {
	// Returns nil when key absent
	fmt.Println(contextx.From(context.Background(), "missing"))
	// Output: <nil>
}

func ExampleFromString() {
	ctx := contextx.With(context.Background(), "request_source", "api")
	fmt.Println(contextx.FromString(ctx, "request_source"))
	fmt.Println(contextx.FromString(ctx, "missing"))
	// Output:
	// api
	//
}

// ============================================================================
// InjectRequestContext EXAMPLES (handler boundary pattern)
// ============================================================================

func ExampleInjectRequestContext() {
	ctx := context.Background()

	// Common handler-boundary pattern: attach everything in one call
	ctx = contextx.InjectRequestContext(ctx,
		contextx.WithRequestIDOption("req_abc"),
		contextx.WithTraceIDOption("trace_xyz"),
		contextx.WithPrincipalOption(&contextx.Principal{
			SubjectID: "u_123",
			TenantID:  "t_acme",
		}),
		contextx.WithLoggerOption(fakeLogger{}),
	)

	// All values are now attached:
	fmt.Println(contextx.RequestIDFromContext(ctx))
	fmt.Println(contextx.TraceIDFromContext(ctx))
	fmt.Println(contextx.PrincipalFromContext(ctx).SubjectID)
	fmt.Println(contextx.TenantFromContext(ctx))
	fmt.Println(contextx.LoggerFromContext(ctx) != nil)
	// Output:
	// req_abc
	// trace_xyz
	// u_123
	// t_acme
	// true
}

// ============================================================================
// RequestIDAndTraceIDFromContext EXAMPLES
// ============================================================================

func ExampleRequestIDAndTraceIDFromContext() {
	ctx := contextx.WithRequestID(context.Background(), "req_abc")
	ctx = contextx.WithTraceID(ctx, "trace_xyz")

	// One call for both — common when building errorx errors:
	rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
	fmt.Println(rid)
	fmt.Println(tid)
	// Output:
	// req_abc
	// trace_xyz
}

// ============================================================================
// Merge EXAMPLES
// ============================================================================

func ExampleMerge() {
	parent1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	parent2 := context.Background()

	merged, cancel := contextx.Merge(parent1, parent2)
	defer cancel()

	// Merged inherits values from parent1 first, then parent2
	merged = contextx.WithRequestID(merged, "req_abc")
	fmt.Println(contextx.RequestIDFromContext(merged))

	// Cancelling parent1 cancels merged
	cancel1()
	fmt.Println(merged.Err() != nil)
	// Output:
	// req_abc
	// true
}

func ExampleMerge_nilParents() {
	// nil parents are treated as context.Background()
	merged, cancel := contextx.Merge(nil, nil)
	defer cancel()

	fmt.Println(merged.Err())
	// Output: <nil>
}
