// Package contextx provides business-facing request context utilities for
// Aisphere Kernel.
//
// contextx is the ONLY package that business code should use for request-scoped
// values: request_id, trace_id, principal, tenant, and the request-bound logger.
// It works with logx (FromContext) and errorx (WithRequestID / WithTraceID)
// without creating import cycles.
//
// # Design principle
//
// contextx only DEFINES request-scoped value injection and extraction. It does
// NOT do logging (use logx), NOT do error wrapping (use errorx), NOT do tracing
// span management (use contrib/otel/tracing). It only carries values through
// context.Context.
//
// contextx depends only on small Kernel foundation packages: authn for the
// canonical Principal model and logx for the Logger type alias. It does NOT
// import errorx, otel, or any transport package.
//
// # 30-second quickstart
//
//	func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//	    ctx := r.Context()
//	    ctx = contextx.WithRequestID(ctx, requestIDFromHeader(r))
//	    ctx = contextx.WithTraceID(ctx, traceIDFromHeaders(r))
//	    ctx = contextx.WithPrincipal(ctx, &contextx.Principal{
//	        SubjectID: "u_123",
//	        TenantID:  "t_acme",
//	        Roles:     []string{"admin"},
//	    })
//	    ctx = contextx.WithLogger(ctx, logger)
//
//	    // Downstream code can extract:
//	    rid := contextx.RequestIDFromContext(ctx)
//	    p := contextx.PrincipalFromContext(ctx)
//	    logger := contextx.LoggerFromContext(ctx)
//	}
//
// # Value types
//
//	contextx.WithRequestID(ctx, "req_abc")        → contextx.RequestIDFromContext(ctx)
//	contextx.WithTraceID(ctx, "trace_xyz")        → contextx.TraceIDFromContext(ctx)
//	contextx.WithPrincipal(ctx, &Principal{...})  → contextx.PrincipalFromContext(ctx)
//	contextx.WithTenant(ctx, "t_acme")            → contextx.TenantFromContext(ctx)
//	contextx.WithLogger(ctx, logger)              → contextx.LoggerFromContext(ctx)
//	contextx.With(ctx, "custom_key", value)       → contextx.From(ctx, "custom_key")
//
// # nil safety
//
// All `XxxFromContext(nil)` and `XxxFromContext(ctx)` (when value absent) return
// zero values, never panic. This is critical because business code calls these
// in hot paths.
//
// # Merge two contexts
//
// When you need to merge values from two contexts (e.g. parent request + child
// goroutine), use contextx.Merge:
//
//	merged, cancel := contextx.Merge(parentCtx, childCtx)
//	defer cancel()
//	// merged.Done() fires when EITHER parent fires
//
// # Further reading
//
// See contextx/README.md for the single-source-of-truth user guide, and
// docs/ai/contextx.md for the AI coding recipe.
package contextx
