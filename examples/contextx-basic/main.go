// Package main demonstrates contextx basic usage.
//
// Run:
//
//	go run ./examples/contextx-basic
package main

import (
	"context"
	"fmt"

	"github.com/aisphereio/kernel/contextx"
)

// fakeLogger satisfies contextx.Logger for the example.
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

func main() {
	ctx := context.Background()

	// 1. Inject request context at boundary (one call)
	ctx = contextx.InjectRequestContext(ctx,
		contextx.WithRequestIDOption("req_abc"),
		contextx.WithTraceIDOption("trace_xyz"),
		contextx.WithPrincipalOption(&contextx.Principal{
			SubjectID:  "u_123",
			TenantID:   "t_acme",
			Roles:      []string{"admin", "viewer"},
			Scopes:     []string{"skill:read", "skill:write"},
			AuthMethod: "oauth",
		}),
		contextx.WithLoggerOption(fakeLogger{}),
	)

	// 2. Extract values anywhere downstream
	fmt.Println("request_id:", contextx.RequestIDFromContext(ctx))
	fmt.Println("trace_id:", contextx.TraceIDFromContext(ctx))

	p := contextx.PrincipalFromContext(ctx)
	fmt.Println("subject_id:", p.SubjectID)
	fmt.Println("tenant:", contextx.TenantFromContext(ctx))
	fmt.Println("is_admin:", p.HasRole("admin"))
	fmt.Println("can_write:", p.HasScope("skill:write"))

	// 3. Both IDs in one call (for errorx)
	rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
	fmt.Printf("for errorx: request_id=%s trace_id=%s\n", rid, tid)

	// 4. nil safety
	fmt.Println("anonymous subject_id:", contextx.SubjectIDFromContext(context.Background()))

	// 5. Custom values
	ctx = contextx.With(ctx, "feature_x_enabled", true)
	if v, ok := contextx.From(ctx, "feature_x_enabled").(bool); ok && v {
		fmt.Println("feature X enabled")
	}

	// 6. Merge two contexts
	parentCtx := contextx.WithTraceID(context.Background(), "trace_parent")
	jobCtx := contextx.WithRequestID(context.Background(), "req_job")
	merged, cancel := contextx.Merge(parentCtx, jobCtx)
	defer cancel()
	fmt.Println("merged trace_id:", contextx.TraceIDFromContext(merged))
	fmt.Println("merged request_id:", contextx.RequestIDFromContext(merged))
}
