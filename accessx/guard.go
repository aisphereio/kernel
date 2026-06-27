package accessx

import (
	"context"

	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

type Check struct {
	Credential authn.Credential
	Principal  authn.Principal

	Permission string
	Resource   authz.ObjectRef

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
	Authn authn.Authenticator
	Authz authz.Authorizer
	Audit auditx.Recorder
}

func New(authenticator authn.Authenticator, authorizer authz.Authorizer, recorder auditx.Recorder) Guard {
	return Guard{Authn: authenticator, Authz: authorizer, Audit: recorder}
}

func DenyAllGuard() Guard {
	return Guard{Authz: authz.DenyAll(), Audit: auditx.Noop()}
}

func AllowAllForDevOnly() Guard {
	return Guard{Authz: authz.AllowAllForDevOnly(), Audit: auditx.Noop()}
}

func (g Guard) Require(ctx context.Context, check Check) (authn.Principal, error) {
	principal, err := g.Authenticate(ctx, check)
	if err != nil {
		g.record(ctx, check, principal, auditx.ResultFailure, err)
		return principal, err
	}
	decision, err := g.Authorize(ctx, principal, check)
	if err != nil {
		g.record(ctx, check, principal, auditx.ResultFailure, err)
		return principal, err
	}
	if !decision.IsAllowed() {
		err := authz.ErrPermissionDenied(decision.Reason)
		g.record(ctx, check, principal, auditx.ResultDenied, err)
		return principal, err
	}
	g.record(ctx, check, principal, auditx.ResultSuccess, nil)
	return principal, nil
}

func (g Guard) Can(ctx context.Context, check Check) (bool, error) {
	principal, err := g.Authenticate(ctx, check)
	if err != nil {
		return false, err
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
			return authn.Anonymous(), nil
		}
		return authn.Principal{}, authn.ErrIdentityBackendFailed("authenticator is not configured", nil)
	}
	return g.Authn.Authenticate(ctx, check.Credential)
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
