// Package ctxinject injects transport metadata into Kernel request contexts.
//
// It is intended to be one of the first server-side middlewares in both HTTP
// and gRPC stacks. Downstream middleware and business code can then rely on
// contextx helpers instead of re-parsing headers at every boundary.
package ctxinject

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/contextx"
	"github.com/aisphereio/kernel/middleware"
	transport "github.com/aisphereio/kernel/transportx"
)

const (
	HeaderRequestID = "X-Request-ID"
	HeaderTraceID   = "X-Trace-ID"
	HeaderTenantID  = "X-Tenant-ID"
)

// Option configures Server.
type Option func(*options)

type options struct {
	requestIDHeaders []string
	traceIDHeaders   []string
	tenantHeaders    []string
	replyRequestID   string
	replyTraceID     string
	replyTenantID    string
}

// WithRequestIDHeaders configures accepted request-id header names.
func WithRequestIDHeaders(headers ...string) Option {
	return func(o *options) {
		o.requestIDHeaders = clean(headers)
	}
}

// WithTraceIDHeaders configures accepted trace-id header names.
func WithTraceIDHeaders(headers ...string) Option {
	return func(o *options) {
		o.traceIDHeaders = clean(headers)
	}
}

// WithTenantHeaders configures accepted tenant-id header names.
func WithTenantHeaders(headers ...string) Option {
	return func(o *options) {
		o.tenantHeaders = clean(headers)
	}
}

// WithReplyHeaders configures response header names used to echo injected
// values. Pass an empty name to disable echoing that value.
func WithReplyHeaders(requestID, traceID, tenantID string) Option {
	return func(o *options) {
		o.replyRequestID = strings.TrimSpace(requestID)
		o.replyTraceID = strings.TrimSpace(traceID)
		o.replyTenantID = strings.TrimSpace(tenantID)
	}
}

// Server returns a middleware that extracts request_id, trace_id and tenant_id
// from transport headers into contextx. It works for HTTP and gRPC because it
// only depends on transportx.Transporter.
func Server(opts ...Option) middleware.Middleware {
	o := &options{
		requestIDHeaders: []string{HeaderRequestID, "x-request-id", "request-id"},
		traceIDHeaders:   []string{HeaderTraceID, "x-trace-id", "traceparent"},
		tenantHeaders:    []string{HeaderTenantID, "x-tenant-id", "tenant-id"},
		replyRequestID:   HeaderRequestID,
		replyTraceID:     HeaderTraceID,
		replyTenantID:    HeaderTenantID,
	}
	for _, opt := range opts {
		opt(o)
	}
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok || tr == nil {
				return next(ctx, req)
			}
			reqHeader := tr.RequestHeader()
			requestID := firstHeader(reqHeader, o.requestIDHeaders...)
			traceID := firstHeader(reqHeader, o.traceIDHeaders...)
			tenantID := firstHeader(reqHeader, o.tenantHeaders...)
			if requestID != "" {
				ctx = contextx.WithRequestID(ctx, requestID)
			}
			if traceID != "" {
				ctx = contextx.WithTraceID(ctx, traceID)
			}
			if tenantID != "" {
				ctx = contextx.WithTenant(ctx, tenantID)
			}
			replyHeader := tr.ReplyHeader()
			if replyHeader != nil {
				setIf(replyHeader, o.replyRequestID, requestID)
				setIf(replyHeader, o.replyTraceID, traceID)
				setIf(replyHeader, o.replyTenantID, tenantID)
			}
			return next(ctx, req)
		}
	}
}

func firstHeader(h transport.Header, names ...string) string {
	if h == nil {
		return ""
	}
	for _, name := range names {
		if v := h.Get(name); strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func setIf(h transport.Header, key, value string) {
	if h == nil || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	h.Set(key, value)
}

func clean(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
