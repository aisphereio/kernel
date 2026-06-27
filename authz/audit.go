package authz

import (
	"context"

	"github.com/aisphereio/kernel/auditx"
)

type AuditRecordBuilder func(req CheckRequest, decision Decision, checkErr error) auditx.Record

// AuditedAuthorizer decorates any Authorizer and records the decision. Audit
// failures do not block authorization unless FailClosed is true.
type AuditedAuthorizer struct {
	Next       Authorizer
	Recorder   auditx.Recorder
	Builder    AuditRecordBuilder
	FailClosed bool
}

func NewAuditedAuthorizer(next Authorizer, recorder auditx.Recorder, opts ...AuditedAuthorizerOption) AuditedAuthorizer {
	a := AuditedAuthorizer{Next: next, Recorder: recorder, Builder: DefaultAuditRecord}
	for _, opt := range opts {
		if opt != nil {
			opt(&a)
		}
	}
	return a
}

type AuditedAuthorizerOption func(*AuditedAuthorizer)

func WithAuditRecordBuilder(builder AuditRecordBuilder) AuditedAuthorizerOption {
	return func(a *AuditedAuthorizer) {
		if builder != nil {
			a.Builder = builder
		}
	}
}

func WithAuditFailClosed(enabled bool) AuditedAuthorizerOption {
	return func(a *AuditedAuthorizer) { a.FailClosed = enabled }
}

func (a AuditedAuthorizer) Check(ctx context.Context, req CheckRequest) (Decision, error) {
	if a.Next == nil {
		return Decision{}, ErrBackendFailed("audited authorizer next is not configured", nil)
	}
	decision, err := a.Next.Check(ctx, req)
	if a.Recorder != nil {
		builder := a.Builder
		if builder == nil {
			builder = DefaultAuditRecord
		}
		if auditErr := a.Recorder.Record(ctx, builder(req, decision, err)); auditErr != nil && a.FailClosed {
			return Decision{}, auditErr
		}
	}
	return decision, err
}

func DefaultAuditRecord(req CheckRequest, decision Decision, checkErr error) auditx.Record {
	result := auditx.ResultDenied
	reason := decision.Reason
	if checkErr != nil {
		result = auditx.ResultFailure
		reason = checkErr.Error()
	} else if decision.IsAllowed() {
		result = auditx.ResultSuccess
	}
	return auditx.Record{
		Action: "authz.check",
		Result: result,
		Reason: reason,
		Actor: auditx.Actor{
			SubjectType: req.Subject.Type,
			SubjectID:   req.Subject.ID,
			TenantID:    req.TenantID,
			OrgID:       req.OrgID,
			ProjectID:   req.ProjectID,
			Attributes:  auditx.AttributeSet(req.SubjectAttrs),
		},
		Resource: auditx.Resource{
			Type:       req.Resource.Type,
			ID:         req.Resource.ID,
			TenantID:   req.TenantID,
			OrgID:      req.OrgID,
			ProjectID:  req.ProjectID,
			Attributes: auditx.AttributeSet(req.ResourceAttrs),
		},
		Metadata: auditx.AttributeSet{
			"permission":        req.Permission,
			"allowed":           decision.Allowed,
			"consistency_token": decision.ConsistencyToken,
			"partial":           decision.Partial,
		},
	}
}
