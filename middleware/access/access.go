// Package access provides server-side authz/audit middleware built on accessx.
package access

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/requestx"
	transport "github.com/aisphereio/kernel/transportx"
)

// Resolver maps a transport operation and protobuf request into an accessx
// check. The bool return reports whether the operation requires a check.
type Resolver func(ctx context.Context, operation string, req any) (accessx.Check, bool, error)

// Option configures Server.
type Option func(*options)

type options struct {
	resolver Resolver
}

// WithResolver configures the operation-to-resource resolver.
func WithResolver(resolver Resolver) Option {
	return func(o *options) { o.resolver = resolver }
}

// Server enforces authz and records audit through accessx.Guard. It fills common
// audit fields such as principal, request_id, trace_id, tenant_id, client_ip and
// user_agent from contextx and transport headers.
func Server(guard accessx.Guard, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if o.resolver == nil {
				return next(ctx, req)
			}
			tr, _ := transport.FromServerContext(ctx)
			op := ""
			if info, ok := requestx.FromContext(ctx); ok {
				op = info.Operation
			}
			if op == "" && tr != nil {
				op = tr.Operation()
			}
			check, required, err := o.resolver(ctx, op, req)
			if err != nil {
				return nil, err
			}
			if !required {
				return next(ctx, req)
			}
			check = enrich(ctx, tr, check)
			if _, err := guard.Require(ctx, check); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

func enrich(ctx context.Context, tr transport.Transporter, check accessx.Check) accessx.Check {
	if info, ok := requestx.FromContext(ctx); ok {
		if check.AuditAction == "" {
			check.AuditAction = info.Action
		}
		if check.TenantID == "" {
			check.TenantID = info.TenantID
		}
		if check.RequestID == "" {
			check.RequestID = info.RequestID
		}
		if check.TraceID == "" {
			check.TraceID = info.TraceID
		}
		if check.Metadata == nil {
			check.Metadata = map[string]any{}
		}
		if info.Service != "" {
			check.Metadata["service"] = info.Service
		}
		if info.Method != "" {
			check.Metadata["method"] = info.Method
		}
		if info.Operation != "" {
			check.Metadata["operation"] = info.Operation
		}
	}

	if !check.Principal.IsAuthenticated() {
		if p, ok := authn.PrincipalFromContext(ctx); ok {
			check.Principal = p
		}
	}
	if check.RequestID == "" {
		check.RequestID = contextx.RequestIDFromContext(ctx)
	}
	if check.TraceID == "" {
		check.TraceID = contextx.TraceIDFromContext(ctx)
	}
	if check.TenantID == "" {
		check.TenantID = contextx.TenantFromContext(ctx)
	}
	if tr != nil && tr.RequestHeader() != nil {
		h := tr.RequestHeader()
		if check.ClientIP == "" {
			check.ClientIP = firstHeader(h, "X-Forwarded-For", "x-forwarded-for", "X-Real-IP", "x-real-ip")
		}
		if check.UserAgent == "" {
			check.UserAgent = firstHeader(h, "User-Agent", "user-agent")
		}
	}
	return check
}

func firstHeader(h transport.Header, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(h.Get(name)); v != "" {
			if i := strings.Index(v, ","); i >= 0 {
				return strings.TrimSpace(v[:i])
			}
			return v
		}
	}
	return ""
}
