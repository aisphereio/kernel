package authz

import "context"

// AuthorizerGuard adapts a simple Authorizer into the generated Guard contract.
// It supports CHECK_ONLY flows. SCOPED_TOKEN requires an IAM implementation that
// can issue scoped tokens and should use a custom Guard.
type AuthorizerGuard struct{ Authorizer Authorizer }

func NewAuthorizerGuard(authorizer Authorizer) AuthorizerGuard {
	return AuthorizerGuard{Authorizer: authorizer}
}

func (g AuthorizerGuard) Require(ctx context.Context, req CheckRequest) (Decision, error) {
	if g.Authorizer == nil {
		return Deny("authorizer_not_configured"), ErrPermissionDenied("authorizer_not_configured")
	}
	decision, err := g.Authorizer.Check(ctx, req)
	if err != nil {
		return Decision{}, err
	}
	if !decision.IsAllowed() {
		return decision, ErrPermissionDenied(decision.Reason)
	}
	return decision, nil
}

func (g AuthorizerGuard) RequireScopedToken(ctx context.Context, req ScopedTokenRequest) (string, Decision, error) {
	decision, err := g.Require(ctx, CheckRequest{
		Subject:    req.Subject,
		Resource:   req.Resource,
		Permission: req.Action,
		TenantID:   req.TenantID,
		OrgID:      req.OrgID,
		ProjectID:  req.ProjectID,
	})
	if err != nil {
		return "", decision, err
	}
	return "", decision, ErrPermissionDenied("scoped token issuer is not configured")
}
