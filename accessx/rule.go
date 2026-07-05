package accessx

// AuthnMode is the provider-neutral authentication requirement declared by a
// generated protobuf operation, gateway route, or service manifest. It is not a
// Casdoor concept; concrete middleware maps it to TokenVerifier/Authenticator
// behavior.
type AuthnMode string

const (
	AuthnModePublic   AuthnMode = "public"
	AuthnModeRequired AuthnMode = "required"
	AuthnModeOptional AuthnMode = "optional"
	AuthnModeInternal AuthnMode = "internal"
)

// ShortCircuitMode names the reason an operation intentionally bypasses the
// normal authorization check. SkipAuthz still requires authentication; SkipAll
// is reserved for truly public endpoints.
type ShortCircuitMode string

const (
	ShortCircuitNone      ShortCircuitMode = "none"
	ShortCircuitPublic    ShortCircuitMode = "public"
	ShortCircuitSelf      ShortCircuitMode = "self"
	ShortCircuitBootstrap ShortCircuitMode = "bootstrap"
	ShortCircuitInternal  ShortCircuitMode = "internal"
	ShortCircuitSystem    ShortCircuitMode = "system"
)

type ResourceResolverSpec struct {
	Type     string `json:"type" yaml:"type"`
	StaticID string `json:"static_id" yaml:"static_id"`
	IDFrom   string `json:"id_from" yaml:"id_from"`
}

type SubjectResolverSpec struct {
	Type     string `json:"type" yaml:"type"`
	IDFrom   string `json:"id_from" yaml:"id_from"`
	Relation string `json:"relation" yaml:"relation"`
}

type ShortCircuitSpec struct {
	Mode   ShortCircuitMode `json:"mode" yaml:"mode"`
	Reason string           `json:"reason" yaml:"reason"`
}

type AuditSpec struct {
	Action string `json:"action" yaml:"action"`
	Level  string `json:"level" yaml:"level"`
}

// AccessRule is the declarative form of a runtime accessx.Check. Gateway route
// registry and protobuf codegen can store this shape without importing Casdoor
// or SpiceDB implementation packages.
type AccessRule struct {
	ID           string               `json:"id" yaml:"id"`
	Authn        AuthnMode            `json:"authn" yaml:"authn"`
	Permission   string               `json:"permission" yaml:"permission"`
	Resource     ResourceResolverSpec `json:"resource" yaml:"resource"`
	Subject      SubjectResolverSpec  `json:"subject" yaml:"subject"`
	ShortCircuit ShortCircuitSpec     `json:"short_circuit" yaml:"short_circuit"`
	Audit        AuditSpec            `json:"audit" yaml:"audit"`
}

func (s ShortCircuitSpec) SkipPolicy() SkipPolicy {
	switch s.Mode {
	case ShortCircuitPublic:
		return SkipAll
	case ShortCircuitSelf, ShortCircuitBootstrap, ShortCircuitInternal, ShortCircuitSystem:
		return SkipAuthz
	default:
		return SkipDefault
	}
}
