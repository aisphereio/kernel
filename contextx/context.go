package contextx

import (
	"context"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/logx"
)

// ctxKey is an unexported key type so no external package can collide with
// our context keys.
type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTraceID
	keyPrincipal
	keyTenant
	keyLogger
	keyCustom // for With/From generic key-value
)

// ============================================================================
// RequestID
// ============================================================================

// WithRequestID attaches a request ID to ctx. Pass empty string to clear.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyRequestID, requestID)
}

// RequestIDFromContext returns the request ID attached via WithRequestID.
// Returns "" if ctx is nil or no request ID is attached. Never panics.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// ============================================================================
// TraceID
// ============================================================================

// WithTraceID attaches a trace ID to ctx.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyTraceID, traceID)
}

// TraceIDFromContext returns the trace ID attached via WithTraceID.
// Returns "" if ctx is nil or no trace ID is attached. Never panics.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(keyTraceID).(string)
	return v
}

// ============================================================================
// Principal
// ============================================================================

// Principal is an alias to authn.Principal. contextx keeps these helpers for
// generic request-context plumbing, but authn owns the canonical identity
// model so transport middleware, accessx and business code all see the same
// fields and methods.
type Principal = authn.Principal

// WithPrincipal attaches a Principal to ctx. Pass nil to clear.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyPrincipal, p)
}

// PrincipalFromContext returns the Principal attached via WithPrincipal.
// Returns nil if ctx is nil or no principal is attached. Never panics.
func PrincipalFromContext(ctx context.Context) *Principal {
	if ctx == nil {
		return nil
	}
	p, _ := ctx.Value(keyPrincipal).(*Principal)
	return p
}

// SubjectIDFromContext is a convenience helper returning Principal.SubjectID
// (or "" if no principal). Common in log fields and audit records.
func SubjectIDFromContext(ctx context.Context) string {
	p := PrincipalFromContext(ctx)
	if p == nil {
		return ""
	}
	return p.SubjectID
}

// ============================================================================
// Tenant
// ============================================================================

// WithTenant attaches a tenant ID to ctx. This is separate from
// Principal.TenantID because some background jobs run without a Principal
// but still need tenant isolation.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyTenant, tenantID)
}

// TenantFromContext returns the tenant ID attached via WithTenant.
// Falls back to PrincipalFromContext(ctx).TenantID if no explicit tenant.
// Returns "" if neither is set. Never panics.
func TenantFromContext(ctx context.Context) string {
	if ctx != nil {
		if v, _ := ctx.Value(keyTenant).(string); v != "" {
			return v
		}
	}
	if p := PrincipalFromContext(ctx); p != nil {
		return p.TenantID
	}
	return ""
}

// ============================================================================
// Logger
// ============================================================================

// Logger is the Kernel logger interface accepted by contextx.
type Logger = logx.Logger

// Field is the structured log field type used by Logger.
type Field = logx.Field

// LogLevel is the Kernel business-facing log level.
type LogLevel = logx.LogLevel

const (
	DebugLevel LogLevel = logx.DebugLevel
	InfoLevel  LogLevel = logx.InfoLevel
	WarnLevel  LogLevel = logx.WarnLevel
	ErrorLevel LogLevel = logx.ErrorLevel
)

// WithLogger attaches a Logger to ctx. Use this at the handler boundary so
// downstream code can fetch it via LoggerFromContext without prop-drilling.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, keyLogger, logger)
}

// LoggerFromContext returns the Logger attached via WithLogger.
// Returns nil if ctx is nil or no logger is attached. Callers should
// nil-check or use LoggerFromContextOr for a fallback.
func LoggerFromContext(ctx context.Context) Logger {
	if ctx == nil {
		return nil
	}
	l, _ := ctx.Value(keyLogger).(Logger)
	return l
}

// LoggerFromContextOr returns the attached Logger, or fallback if none.
// If fallback is also nil, returns nil. This is the safe pattern for
// code that needs a non-nil logger.
func LoggerFromContextOr(ctx context.Context, fallback Logger) Logger {
	if l := LoggerFromContext(ctx); l != nil {
		return l
	}
	return fallback
}

// ============================================================================
// Generic key-value (for custom context values)
// ============================================================================

// With attaches a custom key-value pair to ctx. Use only when the typed
// helpers above don't fit. Prefer typed helpers for type safety.
func With(ctx context.Context, key string, value any) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, customKey{key}, value)
}

// From returns a custom value attached via With. Returns nil if absent.
func From(ctx context.Context, key string) any {
	if ctx == nil {
		return nil
	}
	return ctx.Value(customKey{key})
}

// FromString is a typed convenience for From returning string.
func FromString(ctx context.Context, key string) string {
	v, _ := From(ctx, key).(string)
	return v
}

type customKey struct {
	key string
}

// ============================================================================
// Helpers for errorx integration (no import cycle)
// ============================================================================

// RequestIDAndTraceIDFromContext returns both request_id and trace_id in one
// call. Common when constructing errorx errors:
//
//	rid, tid := contextx.RequestIDAndTraceIDFromContext(ctx)
//	return errorx.NotFound("CODE", "msg",
//	    errorx.WithRequestID(rid),
//	    errorx.WithTraceID(tid),
//	)
func RequestIDAndTraceIDFromContext(ctx context.Context) (requestID, traceID string) {
	return RequestIDFromContext(ctx), TraceIDFromContext(ctx)
}

// InjectRequestContext is a convenience that attaches request_id, trace_id,
// principal, tenant, and logger in one call. Common at handler boundaries.
//
//	logger, _ := logx.New(cfg)
//	ctx = contextx.InjectRequestContext(ctx,
//	    contextx.WithRequestIDOption(reqID),
//	    contextx.WithTraceIDOption(traceID),
//	    contextx.WithPrincipalOption(principal),
//	    contextx.WithLoggerOption(logger),
//	)
func InjectRequestContext(ctx context.Context, opts ...RequestContextOption) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	o := requestContextOpts{}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.requestID != "" {
		ctx = WithRequestID(ctx, o.requestID)
	}
	if o.traceID != "" {
		ctx = WithTraceID(ctx, o.traceID)
	}
	if o.principal != nil {
		ctx = WithPrincipal(ctx, o.principal)
	}
	if o.tenantID != "" {
		ctx = WithTenant(ctx, o.tenantID)
	}
	if o.logger != nil {
		ctx = WithLogger(ctx, o.logger)
	}
	return ctx
}

type requestContextOpts struct {
	requestID string
	traceID   string
	principal *Principal
	tenantID  string
	logger    Logger
}

// RequestContextOption configures InjectRequestContext.
type RequestContextOption func(*requestContextOpts)

// WithRequestIDOption sets request_id for InjectRequestContext.
func WithRequestIDOption(id string) RequestContextOption {
	return func(o *requestContextOpts) { o.requestID = id }
}

// WithTraceIDOption sets trace_id for InjectRequestContext.
func WithTraceIDOption(id string) RequestContextOption {
	return func(o *requestContextOpts) { o.traceID = id }
}

// WithPrincipalOption sets principal for InjectRequestContext.
func WithPrincipalOption(p *Principal) RequestContextOption {
	return func(o *requestContextOpts) { o.principal = p }
}

// WithTenantOption sets tenant_id for InjectRequestContext.
func WithTenantOption(id string) RequestContextOption {
	return func(o *requestContextOpts) { o.tenantID = id }
}

// WithLoggerOption sets logger for InjectRequestContext.
func WithLoggerOption(l Logger) RequestContextOption {
	return func(o *requestContextOpts) { o.logger = l }
}
