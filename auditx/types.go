// Package auditx defines Kernel's durable security audit contracts.
package auditx

import "time"

type AttributeSet map[string]any

const (
	ResultSuccess = "success"
	ResultDenied  = "denied"
	ResultFailure = "failure"
)

const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

type Actor struct {
	SubjectID   string
	SubjectType string
	TenantID    string
	OrgID       string
	ProjectID   string
	Name        string
	Email       string
	Attributes  AttributeSet
}

type Resource struct {
	Type       string
	ID         string
	TenantID   string
	OrgID      string
	ProjectID  string
	Attributes AttributeSet
}

// Record is append-only security evidence. Updates/deletes should create new
// records rather than mutating previous ones.
type Record struct {
	ID        string
	Time      time.Time
	Action    string
	Result    string
	Severity  string
	Reason    string
	Actor     Actor
	Resource  Resource
	RequestID string
	TraceID   string
	ClientIP  string
	UserAgent string
	Metadata  AttributeSet
}

func (r Record) WithTime(now time.Time) Record {
	if r.Time.IsZero() {
		r.Time = now
	}
	return r
}
