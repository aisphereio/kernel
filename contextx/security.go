package contextx

import "context"

const (
	customKeyAuthzDecisionID = "authz_decision_id"
	customKeyScopedToken     = "scoped_token"
	customKeyInternalToken   = "internal_token"
)

// WithAuthzDecisionID attaches the authorization decision id produced by IAM or
// an authz guard. Incoming clients must not be allowed to set this directly; it
// should be created by a trusted Gateway/service boundary.
func WithAuthzDecisionID(ctx context.Context, decisionID string) context.Context {
	return With(ctx, customKeyAuthzDecisionID, decisionID)
}

// AuthzDecisionIDFromContext returns the trusted authorization decision id.
func AuthzDecisionIDFromContext(ctx context.Context) string {
	return FromString(ctx, customKeyAuthzDecisionID)
}

// WithScopedToken attaches a short-lived IAM-scoped token to ctx. gRPC/HTTP
// client middleware can propagate this as the outgoing Authorization header.
func WithScopedToken(ctx context.Context, token string) context.Context {
	return With(ctx, customKeyScopedToken, token)
}

// ScopedTokenFromContext returns a scoped token previously attached by
// WithScopedToken.
func ScopedTokenFromContext(ctx context.Context) string {
	return FromString(ctx, customKeyScopedToken)
}

// WithInternalToken attaches a service/internal token to ctx. It is used when a
// service identity token is pre-issued outside of a client interceptor.
func WithInternalToken(ctx context.Context, token string) context.Context {
	return With(ctx, customKeyInternalToken, token)
}

// InternalTokenFromContext returns an internal token previously attached by
// WithInternalToken.
func InternalTokenFromContext(ctx context.Context) string {
	return FromString(ctx, customKeyInternalToken)
}
