// Package access provides server-side authz/audit middleware built on accessx.
//
// The middleware supports two levels of skip control:
//
//  1. SkipPolicyResolver — checked before the main Resolver. When the policy
//     is SkipAll, Guard.Require records audit and then the request passes
//     through without authn/authz. When the policy is SkipAuthz, the policy is
//     applied to the Check before calling Guard.Require (which skips the
//     SpiceDB check but keeps authn + audit).
//
//  2. Resolver — the standard operation-to-Check mapper. If it returns
//     required=false, the request passes through. Otherwise, Guard.Require
//     enforces the full authn + authz + audit pipeline.
//
// The SkipPolicyResolver is typically created from accessx.NewSkipPolicyResolver
// using the service's security.access configuration, allowing operators to
// configure skip policies without code changes.
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

// SkipPolicyResolver returns an accessx.SkipPolicy for a given operation.
// When the policy is SkipAll, the middleware records audit and skips both authn
// and authz. When the policy is SkipAuthz, the middleware still calls
// Guard.Require with a synthetic check so authentication and audit stay active.
type SkipPolicyResolver func(operation string) accessx.SkipPolicy

// Option configures Server.
type Option func(*options)

type options struct {
	resolver           Resolver
	skipPolicyResolver SkipPolicyResolver
}

// WithResolver configures the operation-to-resource resolver.
func WithResolver(resolver Resolver) Option {
	return func(o *options) { o.resolver = resolver }
}

// WithSkipPolicyResolver configures config-driven access shortcuts. Public
// endpoints use SkipAll. Self/bootstrap/internal endpoints usually use
// SkipAuthz so they still require authentication and generate audit records.
func WithSkipPolicyResolver(resolver SkipPolicyResolver) Option {
	return func(o *options) { o.skipPolicyResolver = resolver }
}

// Server enforces authz and records audit through accessx.Guard. It fills common
// audit fields such as principal, request_id, trace_id, tenant_id, client_ip and
// user_agent from contextx and transport headers.
//
// The middleware checks skip policies in this order:
//  1. SkipAll from SkipPolicyResolver passes through as public.
//  2. SkipAuthz from SkipPolicyResolver authenticates and audits through Guard.
//  3. Resolver checks are enriched and enforced through Guard.
//  4. Resolver required=false passes through.
func Server(guard accessx.Guard, opts ...Option) middleware.Middleware {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, _ := transport.FromServerContext(ctx)
			op := resolveOperation(ctx, tr)

			// 1. Check config-driven skip policy before calling the resolver.
			// SkipAll is public. SkipAuthz still requires authn + audit, so build
			// a minimal check and pass it through Guard.Require. This keeps GetMe,
			// OIDC callback guards, and bootstrap routes consistent even when a
			// service has no generated operation resolver for that route.
			if o.skipPolicyResolver != nil {
				if policy := o.skipPolicyResolver(op); policy != accessx.SkipDefault {
					check := enrich(ctx, tr, accessx.Check{
						SkipPolicy:  policy,
						AuditAction: auditActionFromOperation(op),
					})
					// SkipAll still goes through Guard.Require so public operations are
					// recorded by the audit recorder. Guard.Require does not authenticate or
					// authorize SkipAll checks.
					if _, err := guard.Require(ctx, check); err != nil {
						return nil, err
					}
					return next(ctx, req)
				}
			}

			// 2. Resolve the access check.
			if o.resolver == nil {
				return next(ctx, req)
			}
			check, required, err := o.resolver(ctx, op, req)
			if err != nil {
				return nil, err
			}
			if !required {
				return next(ctx, req)
			}
			check = enrich(ctx, tr, check)

			// 3. Apply skip policy from resolver if not already set on the check.
			if o.skipPolicyResolver != nil && check.SkipPolicy == accessx.SkipDefault {
				if policy := o.skipPolicyResolver(op); policy != accessx.SkipDefault {
					check.SkipPolicy = policy
				}
			}

			if _, err := guard.Require(ctx, check); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

// resolveOperation extracts the operation name from context or transport.
func resolveOperation(ctx context.Context, tr transport.Transporter) string {
	if info, ok := requestx.FromContext(ctx); ok && info.Operation != "" {
		return info.Operation
	}
	if tr != nil {
		return tr.Operation()
	}
	return ""
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

func auditActionFromOperation(operation string) string {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return "access.unknown"
	}
	if i := strings.LastIndex(operation, "/"); i >= 0 && i+1 < len(operation) {
		operation = operation[i+1:]
	}
	operation = strings.ReplaceAll(operation, ".", "_")
	return "access." + strings.ToLower(operation)
}
