package contextx_test

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/contextx"
)

// This file demonstrates COMPLETE business scenarios showing how contextx
// fits into handler → service → repository flow. AI tools should copy these
// patterns when generating new business code.

// ============================================================================
// SCENARIO 1: Handler boundary — inject request context
// ============================================================================

// Example_businessHandlerBoundary shows the standard pattern at HTTP handler
// entry: extract request_id / trace_id / principal from transport, attach to
// ctx in one call.
func Example_businessHandlerBoundary() {
	// Simulate handler entry
	ctx := context.Background()

	// In real code: reqID := r.Header.Get("X-Request-ID")
	// In real code: principal := authn.Verify(r.Header.Get("Authorization"))
	ctx = contextx.InjectRequestContext(ctx,
		contextx.WithRequestIDOption("req_abc"),
		contextx.WithTraceIDOption("trace_xyz"),
		contextx.WithPrincipalOption(&contextx.Principal{
			SubjectID:  "u_123",
			TenantID:   "t_acme",
			Roles:      []string{"viewer"},
			AuthMethod: "oauth",
		}),
	)

	// All downstream code can extract:
	fmt.Println(contextx.RequestIDFromContext(ctx))
	fmt.Println(contextx.PrincipalFromContext(ctx).SubjectID)
	// Output:
	// req_abc
	// u_123
}

// ============================================================================
// SCENARIO 2: Service layer — use principal for authz
// ============================================================================

// Example_businessServiceAuthz shows service-layer permission check using
// Principal from context.
func Example_businessServiceAuthz() {
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{
		SubjectID: "u_123",
		Roles:     []string{"admin"},
	})

	// In real code: if !contextx.PrincipalFromContext(ctx).HasRole("admin") { return errorx.Forbidden(...) }
	p := contextx.PrincipalFromContext(ctx)
	if p.HasRole("admin") {
		fmt.Println("allowed")
	} else {
		fmt.Println("denied")
	}
	// Output: allowed
}

// ============================================================================
// SCENARIO 3: Repository layer — attach request_id to errorx
// ============================================================================

// Example_businessRepositoryErrorx shows how repository extracts request_id
// from context to attach to errorx errors.
func Example_businessRepositoryErrorx() {
	ctx := contextx.WithRequestID(context.Background(), "req_abc")
	ctx = contextx.WithTraceID(ctx, "trace_xyz")

	// Common pattern when returning errorx:
	rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
	fmt.Println(rid)
	fmt.Println(tid)

	// In real code:
	// return errorx.NotFound("AIHUB_SKILL_NOT_FOUND", "skill not found",
	//     errorx.WithRequestID(rid),
	//     errorx.WithTraceID(tid),
	// )
	// Output:
	// req_abc
	// trace_xyz
}

// ============================================================================
// SCENARIO 4: Tenant isolation in multi-tenant query
// ============================================================================

// Example_businessTenantIsolation shows how to enforce tenant isolation in
// repository queries using TenantFromContext.
func Example_businessTenantIsolation() {
	ctx := contextx.WithTenant(context.Background(), "t_acme")

	tenantID := contextx.TenantFromContext(ctx)
	if tenantID == "" {
		fmt.Println("no tenant — single-tenant mode")
	} else {
		fmt.Printf("querying for tenant=%s\n", tenantID)
	}

	// In real code: db.Where("tenant_id = ?", tenantID).Find(&rows)
	// Output: querying for tenant=t_acme
}

// ============================================================================
// SCENARIO 5: Logger propagation
// ============================================================================

// Example_businessLoggerPropagation shows how to fetch the request-scoped
// logger in deep repository code without prop-drilling.
func Example_businessLoggerPropagation() {
	// At handler boundary:
	ctx := contextx.WithLogger(context.Background(), fakeLogger{})

	// Deep in repository:
	logger := contextx.LoggerFromContext(ctx)
	if logger != nil {
		logger.Info("querying skill")
	}
	fmt.Println("ok")
	// Output: ok
}

// ============================================================================
// SCENARIO 6: Worker with merged context
// ============================================================================

// Example_businessWorkerMergedContext shows a background worker that merges
// a job context with a parent request context (e.g. for trace propagation).
func Example_businessWorkerMergedContext() {
	parentCtx := contextx.WithTraceID(context.Background(), "trace_parent")
	jobCtx := contextx.WithRequestID(context.Background(), "req_job")

	merged, cancel := contextx.Merge(parentCtx, jobCtx)
	defer cancel()

	// Merged inherits values from both parents
	fmt.Println(contextx.TraceIDFromContext(merged))   // from parent1
	fmt.Println(contextx.RequestIDFromContext(merged)) // from parent2
	// Output:
	// trace_parent
	// req_job
}

// ============================================================================
// SCENARIO 7: Anonymous request (no principal)
// ============================================================================

// Example_businessAnonymousRequest shows nil-safety: code that handles both
// authenticated and anonymous requests without nil-checks everywhere.
func Example_businessAnonymousRequest() {
	ctx := context.Background() // no principal attached

	// Safe to call directly — returns nil, never panics
	p := contextx.PrincipalFromContext(ctx)
	if p == nil || !p.IsAuthenticated() {
		fmt.Println("anonymous request")
	} else {
		fmt.Printf("authenticated: %s\n", p.SubjectID)
	}

	// SubjectIDFromContext returns "" for anonymous
	fmt.Printf("subject_id=%q\n", contextx.SubjectIDFromContext(ctx))
	// Output:
	// anonymous request
	// subject_id=""
}

// ============================================================================
// SCENARIO 8: Custom context values
// ============================================================================

// Example_businessCustomValues shows how to attach domain-specific values
// that don't fit the typed helpers.
func Example_businessCustomValues() {
	ctx := context.Background()

	// Attach a feature flag
	ctx = contextx.With(ctx, "feature_x_enabled", true)
	ctx = contextx.With(ctx, "ab_test_variant", "control")

	// Extract
	if v, ok := contextx.From(ctx, "feature_x_enabled").(bool); ok && v {
		fmt.Println("feature X enabled")
	}
	fmt.Println(contextx.FromString(ctx, "ab_test_variant"))
	// Output:
	// feature X enabled
	// control
}

// ============================================================================
// SCENARIO 9: Multi-tenant with principal fallback
// ============================================================================

// Example_businessTenantFallback shows the tenant resolution priority:
// explicit WithTenant > Principal.TenantID > empty.
func Example_businessTenantFallback() {
	// Case 1: explicit tenant wins
	ctx1 := contextx.WithPrincipal(context.Background(), &contextx.Principal{TenantID: "t_principal"})
	ctx1 = contextx.WithTenant(ctx1, "t_explicit")
	fmt.Println(contextx.TenantFromContext(ctx1))

	// Case 2: falls back to principal tenant
	ctx2 := contextx.WithPrincipal(context.Background(), &contextx.Principal{TenantID: "t_principal"})
	fmt.Println(contextx.TenantFromContext(ctx2))

	// Case 3: no tenant anywhere
	ctx3 := context.Background()
	fmt.Println(contextx.TenantFromContext(ctx3) == "")
	// Output:
	// t_explicit
	// t_principal
	// true
}

// ============================================================================
// SCENARIO 10: Scope-based authorization
// ============================================================================

// Example_businessScopeAuthz shows OAuth scope-based authorization.
func Example_businessScopeAuthz() {
	ctx := contextx.WithPrincipal(context.Background(), &contextx.Principal{
		SubjectID: "svc:agentkit",
		Scopes:    []string{"skill:read", "skill:download"},
	})

	p := contextx.PrincipalFromContext(ctx)
	requiredScope := "skill:write"

	if !p.HasScope(requiredScope) {
		fmt.Printf("denied: missing scope %s\n", requiredScope)
		// In real code: return errorx.Forbidden("AIHUB_SCOPE_DENIED", "missing scope",
		//     errorx.WithMetadata("required_scope", requiredScope),
		// )
	} else {
		fmt.Println("allowed")
	}
	// Output: denied: missing scope skill:write
}
