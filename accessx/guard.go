package accessx

import (
	"context"

"github.com/aisphereio/kernel/auditx"
		"github.com/aisphereio/kernel/authn"
		"github.com/aisphereio/kernel/authz"
		"github.com/aisphereio/kernel/errorx"
)

// AuthzMode controls whether the Guard performs a SpiceDB/ReBAC authorization
// check or skips it. Operations that bootstrap a new resource (which cannot
// exist in SpiceDB yet) should use AuthzModeNone.
//
// Deprecated: Use SkipPolicy instead. AuthzMode is kept for backward
// compatibility and will be removed in a future version.
type AuthzMode string

const (
	// AuthzModeDefault performs the standard SpiceDB authorization check.
	AuthzModeDefault AuthzMode = ""
	// AuthzModeNone skips the SpiceDB authorization check entirely. The request
	// still goes through authentication, platform policy, and audit.
	//
	// Deprecated: Use SkipPolicy=SkipAuthz instead.
	AuthzModeNone AuthzMode = "None"
)

// SkipPolicy controls whether the Guard performs authentication and/or
// authorization checks. It replaces the previous AllowAll + AuthzMode
// combination with a single, clearer policy.
type SkipPolicy string

const (
	// SkipDefault performs the standard authentication + authorization check.
	SkipDefault SkipPolicy = ""
	// SkipAuthz skips the SpiceDB authorization check but still requires
	// authentication and records audit. Use for operations like GetMe
	// where the resource is the caller's own identity, or CreateOrganization
	// where the target resource does not yet exist in the authorization graph.
	SkipAuthz SkipPolicy = "skip_authz"
	// SkipAll skips both authentication AND authorization. The request passes
	// through without any authenticated principal, but Guard.Require still records
	// audit with an anonymous actor. Use for public endpoints like health checks,
	// login, token exchange, etc.
	// WARNING: No authenticated principal will be available in the handler.
	SkipAll SkipPolicy = "skip_all"
)

type Check struct {
	Credential authn.Credential
	Principal  authn.Principal

	Permission string
	Resource   authz.ObjectRef

	// SkipPolicy controls whether authorization (and optionally authentication)
	// is skipped. The default (SkipDefault) performs the full authn+authz check.
	// When set to SkipAuthz, the Guard skips the SpiceDB check but still
	// authenticates and records audit. When set to SkipAll, both authentication
	// and authorization are skipped.
	//
	// If SkipPolicy is SkipDefault, the deprecated AuthzMode and AllowAll fields
	// are checked for backward compatibility.
	SkipPolicy SkipPolicy

	// AuthzMode controls whether SpiceDB authorization is required.
	// Use AuthzModeNone for operations like CreateOrganization where the
	// target resource does not yet exist in the authorization graph.
	//
	// Deprecated: Use SkipPolicy=SkipAuthz instead.
	AuthzMode AuthzMode

	// AllowAll grants access to every authenticated principal when true.
	// This is useful for self-service operations (e.g. CreateOrganization,
	// CreateProject) where any authenticated user should be allowed.
	// When AllowAll is true, AuthzMode is implicitly AuthzModeNone.
	// This can be made configurable per deployment via platform policy.
	//
	// Deprecated: Use SkipPolicy=SkipAuthz instead.
	AllowAll bool

	TenantID  string
	OrgID     string
	ProjectID string

	SubjectAttrs     authz.AttributeSet
	ResourceAttrs    authz.AttributeSet
	EnvironmentAttrs authz.AttributeSet
	Consistency      authz.Consistency

	AuditAction string
	RequestID   string
	TraceID     string
	ClientIP    string
	UserAgent   string
	Metadata    auditx.AttributeSet
}

type Guard struct {
	// Authn is retained only as a legacy fallback for callers that still pass
	// credentials directly to accessx.Check. The primary framework path is:
	// middleware/authn authenticates first, injects authn.Principal into context,
	// then accessx consumes Check.Principal. New services should not rely on this
	// field for the normal request path.
	Authn authn.Authenticator
	Authz authz.Authorizer
	Audit auditx.Recorder
}

// New constructs a Guard. The authenticator parameter is a legacy fallback;
// new code should authenticate through middleware/authn and pass nil here.
func New(authenticator authn.Authenticator, authorizer authz.Authorizer, recorder auditx.Recorder) Guard {
	return Guard{Authn: authenticator, Authz: authorizer, Audit: recorder}
}

// NewGuard constructs the preferred access-only guard. Authentication must have
// run before accessx.Require, and the principal must be available in Check or
// context.
func NewGuard(authorizer authz.Authorizer, recorder auditx.Recorder) Guard {
	return Guard{Authz: authorizer, Audit: recorder}
}

func DenyAllGuard() Guard {
	return Guard{Authz: authz.DenyAll(), Audit: auditx.Noop()}
}

func AllowAllForDevOnly() Guard {
	return Guard{Authz: authz.AllowAllForDevOnly(), Audit: auditx.Noop()}
}

func (g Guard) Require(ctx context.Context, check Check) (authn.Principal, error) {
	// SkipAll skips both authentication and authorization entirely.
	// Use for public endpoints (health checks, login, etc.) where no
	// principal is expected.
	if check.SkipPolicy == SkipAll {
		g.record(ctx, check, authn.Principal{}, auditx.ResultSuccess, nil)
		return authn.Anonymous(), nil
	}

	principal, err := g.Authenticate(ctx, check)
	if err != nil {
		g.record(ctx, check, principal, auditx.ResultFailure, err)
		return principal, err
	}

	// Check skip conditions in priority order:
	// 1. SkipPolicy (new) — SkipAuthz skips SpiceDB but keeps authn + audit
	// 2. AllowAll (deprecated) — grants access to every authenticated principal
	// 3. AuthzModeNone (deprecated) — skips SpiceDB check
	if check.SkipPolicy == SkipAuthz || check.AllowAll || check.AuthzMode == AuthzModeNone {
		g.record(ctx, check, principal, auditx.ResultSuccess, nil)
		return principal, nil
	}

	decision, err := g.Authorize(ctx, principal, check)
	if err != nil {
		g.record(ctx, check, principal, auditx.ResultFailure, err)
		return principal, err
	}
if !decision.IsAllowed() {
			err := errPermissionDeniedWith(decision)
			g.record(ctx, check, principal, auditx.ResultDenied, err)
			return principal, err
		}
	g.record(ctx, check, principal, auditx.ResultSuccess, nil)
	return principal, nil
}

func (g Guard) Can(ctx context.Context, check Check) (bool, error) {
	// SkipAll skips both authentication and authorization.
	if check.SkipPolicy == SkipAll {
		return true, nil
	}
	principal, err := g.Authenticate(ctx, check)
	if err != nil {
		return false, err
	}
	// SkipAuthz or deprecated AllowAll/AuthzModeNone — skip SpiceDB check.
	if check.SkipPolicy == SkipAuthz || check.AllowAll || check.AuthzMode == AuthzModeNone {
		return true, nil
	}
	decision, err := g.Authorize(ctx, principal, check)
	if err != nil {
		return false, err
	}
	return decision.IsAllowed(), nil
}

func (g Guard) Authenticate(ctx context.Context, check Check) (authn.Principal, error) {
	if check.Principal.IsAuthenticated() {
		return check.Principal.Normalize(), nil
	}
	if g.Authn == nil {
		if check.Credential.Token == "" {
			return authn.Principal{}, authn.ErrUnauthenticated("authentication required")
		}
		return authn.Principal{}, authn.ErrIdentityBackendFailed("authenticator is not configured", nil)
	}
	principal, err := g.Authn.Authenticate(ctx, check.Credential)
	if err != nil {
		return authn.Principal{}, err
	}
	if !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrUnauthenticated("authentication required")
	}
	return principal.Normalize(), nil
}

func (g Guard) Authorize(ctx context.Context, principal authn.Principal, check Check) (authz.Decision, error) {
	if g.Authz == nil {
		return authz.Deny("authorizer is not configured"), nil
	}
	return g.Authz.Check(ctx, authz.CheckRequest{
		Subject: authz.SubjectRef{
			Type: principal.SubjectType,
			ID:   principal.SubjectID,
		},
		Resource:         check.Resource,
		Permission:       check.Permission,
		TenantID:         firstNonEmpty(check.TenantID, principal.TenantID),
		OrgID:            firstNonEmpty(check.OrgID, principal.OrgID),
		ProjectID:        firstNonEmpty(check.ProjectID, principal.ProjectID),
		SubjectAttrs:     mergeAttrs(authz.AttributeSet(principal.Attributes), check.SubjectAttrs),
		ResourceAttrs:    check.ResourceAttrs,
		EnvironmentAttrs: check.EnvironmentAttrs,
		Consistency:      check.Consistency,
	})
}

func (g Guard) Record(ctx context.Context, record auditx.Record) error {
	if g.Audit == nil {
		return nil
	}
	return g.Audit.Record(ctx, record)
}

func (g Guard) record(ctx context.Context, check Check, principal authn.Principal, result string, err error) {
	if g.Audit == nil {
		return
	}
	action := check.AuditAction
	if action == "" {
		action = "authz.check"
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	_ = g.Audit.Record(ctx, auditx.Record{
		Action:   action,
		Result:   result,
		Severity: auditx.SeverityInfo,
		Reason:   reason,
		Actor: auditx.Actor{
			SubjectID:   principal.SubjectID,
			SubjectType: principal.SubjectType,
			TenantID:    firstNonEmpty(check.TenantID, principal.TenantID),
			OrgID:       firstNonEmpty(check.OrgID, principal.OrgID),
			ProjectID:   firstNonEmpty(check.ProjectID, principal.ProjectID),
			Name:        principal.Name,
			Email:       principal.Email,
			Attributes:  auditx.AttributeSet(principal.Attributes),
		},
		Resource: auditx.Resource{
			Type:       check.Resource.Type,
			ID:         check.Resource.ID,
			TenantID:   firstNonEmpty(check.TenantID, principal.TenantID),
			OrgID:      firstNonEmpty(check.OrgID, principal.OrgID),
			ProjectID:  firstNonEmpty(check.ProjectID, principal.ProjectID),
			Attributes: auditx.AttributeSet(check.ResourceAttrs),
		},
		RequestID: check.RequestID,
		TraceID:   check.TraceID,
		ClientIP:  check.ClientIP,
		UserAgent: check.UserAgent,
		Metadata:  check.Metadata,
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mergeAttrs(base, overlay authz.AttributeSet) authz.AttributeSet {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	merged := make(authz.AttributeSet, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged
}

// errPermissionDeniedWith creates a permission denied error enriched with
// decision metadata (reason, decision_id) so that transport encoders can
// surface them in HTTP/gRPC error responses.
func errPermissionDeniedWith(d authz.Decision) error {
	message := d.Reason
	if message == "" {
		message = "permission denied"
	}
	opts := []errorx.Option{
		errorx.WithPublicMetadata("reason", d.Reason),
	}
	if d.ConsistencyToken != "" {
		opts = append(opts, errorx.WithPublicMetadata("decision_id", d.ConsistencyToken))
	}
	return errorx.Forbidden(authz.CodePermissionDenied, message, opts...)
}
