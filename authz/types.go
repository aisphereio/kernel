// Package authz defines Kernel's provider-neutral authorization contracts.
package authz

import (
	"strings"
	"time"
)

// AttributeSet carries ABAC/caveat context. Values must be JSON-serializable
// when passed to remote engines such as SpiceDB caveats or OPA.
type AttributeSet map[string]any

type ObjectRef struct {
	Type string
	ID   string
}

func (r ObjectRef) IsZero() bool {
	return strings.TrimSpace(r.Type) == "" || strings.TrimSpace(r.ID) == ""
}

func (r ObjectRef) String() string {
	if r.Type == "" {
		return r.ID
	}
	return r.Type + ":" + r.ID
}

type SubjectRef struct {
	Type     string
	ID       string
	Relation string
}

func (s SubjectRef) IsZero() bool {
	return strings.TrimSpace(s.Type) == "" || strings.TrimSpace(s.ID) == ""
}

func (s SubjectRef) String() string {
	base := s.Type + ":" + s.ID
	if s.Relation != "" {
		base += "#" + s.Relation
	}
	return base
}

const (
	SubjectTypeUser    = "user"
	SubjectTypeService = "service"
	SubjectTypeGroup   = "group"
	SubjectTypeTeam    = "team"
)

type ConsistencyMode string

const (
	ConsistencyMinimizeLatency ConsistencyMode = "minimize_latency"
	ConsistencyAtLeastAsFresh  ConsistencyMode = "at_least_as_fresh"
	ConsistencyFullyConsistent ConsistencyMode = "fully_consistent"
)

// Consistency carries a SpiceDB-style causal token without making callers know
// about the concrete backend. Token maps to SpiceDB ZedToken for ReBAC engines.
type Consistency struct {
	Mode  ConsistencyMode
	Token string
}

// Relationship is a Zanzibar/ReBAC tuple, optionally caveated for ABAC.
// Example: document:doc_1#viewer@user:u_1.
type Relationship struct {
	Resource      ObjectRef
	Relation      string
	Subject       SubjectRef
	CaveatName    string
	CaveatContext AttributeSet

	// ExpiresAt supports time-limited sharing and temporary grants. For SpiceDB
	// adapters this can be implemented with relationship expiration when
	// available, or encoded as a caveat otherwise. Zero means no expiry.
	ExpiresAt time.Time
}

type RelationshipFilter struct {
	ResourceType string
	ResourceID   string
	Relation     string
	SubjectType  string
	SubjectID    string
	SubjectRel   string
}

type WriteResult struct {
	ConsistencyToken string
	Written          int
	Deleted          int
}

type DecisionEffect string

const (
	DecisionAllow   DecisionEffect = "allow"
	DecisionDeny    DecisionEffect = "deny"
	DecisionNoMatch DecisionEffect = "no_match"
)

type CheckRequest struct {
	Subject    SubjectRef
	Resource   ObjectRef
	Permission string

	TenantID  string
	OrgID     string
	ProjectID string

	SubjectAttrs     AttributeSet
	ResourceAttrs    AttributeSet
	EnvironmentAttrs AttributeSet
	Consistency      Consistency
}

type Decision struct {
	// Effect preserves the important difference between explicit deny and no
	// matching relationship/policy. Allowed is kept for ergonomic callers and
	// backwards-compatible checks.
	Effect           DecisionEffect
	Allowed          bool
	Reason           string
	ConsistencyToken string
	Partial          bool
	MissingContext   []string
}

func Allow(reason string) Decision {
	return Decision{Effect: DecisionAllow, Allowed: true, Reason: reason}
}

func Deny(reason string) Decision {
	return Decision{Effect: DecisionDeny, Allowed: false, Reason: reason}
}

func NoMatch(reason string) Decision {
	return Decision{Effect: DecisionNoMatch, Allowed: false, Reason: reason}
}

func (d Decision) IsAllowed() bool { return d.Allowed || d.Effect == DecisionAllow }

func (d Decision) IsDenied() bool { return d.Effect == DecisionDeny }

func (d Decision) IsNoMatch() bool { return d.Effect == DecisionNoMatch }

type BatchCheckRequest struct {
	Checks      []CheckRequest
	Consistency Consistency
}

type BatchCheckResult struct {
	Decisions []Decision
}

type LookupResourcesRequest struct {
	Subject      SubjectRef
	ResourceType string
	Permission   string

	TenantID  string
	OrgID     string
	ProjectID string

	SubjectAttrs     AttributeSet
	EnvironmentAttrs AttributeSet
	Consistency      Consistency
	Limit            int
	Cursor           string
}

type LookupResourcesResult struct {
	Resources        []ObjectRef
	NextCursor       string
	ConsistencyToken string
}

type LookupSubjectsRequest struct {
	Resource    ObjectRef
	Permission  string
	SubjectType string

	TenantID  string
	OrgID     string
	ProjectID string

	ResourceAttrs    AttributeSet
	EnvironmentAttrs AttributeSet
	Consistency      Consistency
	Limit            int
	Cursor           string
}

type LookupSubjectsResult struct {
	Subjects         []SubjectRef
	NextCursor       string
	ConsistencyToken string
}

type Schema struct {
	Text    string
	Version string
}
