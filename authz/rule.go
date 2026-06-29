package authz

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// RuleMode describes how a generated authz rule should be enforced.
type RuleMode string

const (
	RuleModeUnspecified RuleMode = "UNSPECIFIED"
	// RuleModeCheckOnly means callers should call IAM.Check and rely on the
	// resulting decision_id for downstream propagation.
	RuleModeCheckOnly RuleMode = "CHECK_ONLY"
	// RuleModeScopedToken means callers should ask IAM for a short-lived scoped
	// token and pass that token to the target service.
	RuleModeScopedToken RuleMode = "SCOPED_TOKEN"
	// RuleModeSelfCheck means the resource service should perform the final IAM
	// check at its own boundary. This is suited for high-risk operations.
	RuleModeSelfCheck RuleMode = "SELF_CHECK"
)

// Rule is the generated, protocol-neutral authorization contract for one RPC.
// It is intentionally small and can be reused by grpcx, REST/BFF handlers,
// docs, tests, and future Buf check plugins.
type Rule struct {
	Service    string   `json:"service" yaml:"service"`
	Method     string   `json:"method" yaml:"method"`
	FullMethod string   `json:"full_method" yaml:"full_method"`
	Action     string   `json:"action" yaml:"action"`
	Resource   string   `json:"resource" yaml:"resource"`
	Audience   string   `json:"audience" yaml:"audience"`
	Mode       RuleMode `json:"mode" yaml:"mode"`

	AuditEvent string `json:"audit_event,omitempty" yaml:"audit_event,omitempty"`
	AuditRisk  string `json:"audit_risk,omitempty" yaml:"audit_risk,omitempty"`
	Capability string `json:"capability,omitempty" yaml:"capability,omitempty"`
}

// Rules is a generated rules table keyed by gRPC full method name, for example
// /skill.v1.SkillService/DownloadSkillPackage.
type Rules map[string]Rule

// ScopedTokenRequest asks an IAM implementation for a token that is bound to a
// concrete service audience, action, and resource.
type ScopedTokenRequest struct {
	Subject  SubjectRef
	Action   string
	Resource ObjectRef
	Audience string
	Rule     Rule

	TenantID  string
	OrgID     string
	ProjectID string

	DecisionID string
	Reason     string
	Attributes AttributeSet
}

// Guard is the high-level API used by generated secure clients. Implement it
// with your IAM adapter. Kernel provides the contract; concrete deployments can
// back it with SpiceDB/OpenFGA/OPA/decision cache/scoped JWT issuance.
type Guard interface {
	Require(ctx context.Context, req CheckRequest) (Decision, error)
	RequireScopedToken(ctx context.Context, req ScopedTokenRequest) (string, Decision, error)
}

// RuleResolver expands generated resource templates such as skill:{skill_id}
// from a protobuf request object. It uses reflection against exported Go fields
// and protobuf-style snake_case names.
type RuleResolver struct{}

// ResolveResource expands rule.Resource into an ObjectRef. Resource templates
// are expected to use the form "type:{field_name}" or "type:literal".
func (RuleResolver) ResolveResource(rule Rule, req any) (ObjectRef, error) {
	resource := strings.TrimSpace(rule.Resource)
	if resource == "" {
		return ObjectRef{}, nil
	}
	parts := strings.SplitN(resource, ":", 2)
	if len(parts) != 2 {
		return ObjectRef{ID: expandTemplate(resource, req)}, nil
	}
	id := expandTemplate(parts[1], req)
	if strings.Contains(id, "{") || strings.Contains(id, "}") {
		return ObjectRef{}, fmt.Errorf("authz: unresolved resource template %q", rule.Resource)
	}
	return ObjectRef{Type: parts[0], ID: id}, nil
}

func expandTemplate(tpl string, req any) string {
	out := tpl
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			return out
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			return out
		}
		end += start
		name := out[start+1 : end]
		value := fieldString(req, name)
		out = out[:start] + value + out[end+1:]
	}
}

func fieldString(req any, name string) string {
	if req == nil {
		return ""
	}
	v := reflect.ValueOf(req)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	t := v.Type()
	camel := snakeToCamel(name)
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		if f.Name == camel || strings.EqualFold(f.Name, name) || strings.EqualFold(snakeCase(f.Name), name) {
			fv := v.Field(i)
			if fv.Kind() == reflect.String {
				return fv.String()
			}
			if fv.CanInterface() {
				return fmt.Sprint(fv.Interface())
			}
		}
	}
	return ""
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
