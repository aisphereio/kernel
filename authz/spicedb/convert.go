package spicedb

import (
	"context"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authz"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bearerToken struct {
	token    string
	insecure bool
}

func (b bearerToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

func (b bearerToken) RequireTransportSecurity() bool { return !b.insecure }

var _ credentials.PerRPCCredentials = bearerToken{}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.cfg.Timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.cfg.Timeout)
}

func objectToProto(obj authz.ObjectRef) *v1.ObjectReference {
	return &v1.ObjectReference{ObjectType: obj.Type, ObjectId: obj.ID}
}

func objectFromProto(obj *v1.ObjectReference) authz.ObjectRef {
	if obj == nil {
		return authz.ObjectRef{}
	}
	return authz.ObjectRef{Type: obj.GetObjectType(), ID: obj.GetObjectId()}
}

func subjectToProto(subject authz.SubjectRef) *v1.SubjectReference {
	return &v1.SubjectReference{Object: &v1.ObjectReference{ObjectType: subject.Type, ObjectId: subject.ID}, OptionalRelation: subject.Relation}
}

func subjectFromProto(subject *v1.SubjectReference) authz.SubjectRef {
	if subject == nil || subject.GetObject() == nil {
		return authz.SubjectRef{}
	}
	return authz.SubjectRef{Type: subject.GetObject().GetObjectType(), ID: subject.GetObject().GetObjectId(), Relation: subject.GetOptionalRelation()}
}

func relationshipToProto(rel authz.Relationship) *v1.Relationship {
	out := &v1.Relationship{
		Resource: objectToProto(rel.Resource),
		Relation: rel.Relation,
		Subject:  subjectToProto(rel.Subject),
	}
	if rel.CaveatName != "" {
		out.OptionalCaveat = &v1.ContextualizedCaveat{CaveatName: rel.CaveatName, Context: attrsToStruct(rel.CaveatContext)}
	}
	if !rel.ExpiresAt.IsZero() {
		out.OptionalExpiration = timestamppb.New(rel.ExpiresAt)
	}
	return out
}

func relationshipFromProto(rel *v1.Relationship) authz.Relationship {
	if rel == nil {
		return authz.Relationship{}
	}
	out := authz.Relationship{Resource: objectFromProto(rel.GetResource()), Relation: rel.GetRelation(), Subject: subjectFromProto(rel.GetSubject())}
	if rel.GetOptionalCaveat() != nil {
		out.CaveatName = rel.GetOptionalCaveat().GetCaveatName()
		out.CaveatContext = structToAttrs(rel.GetOptionalCaveat().GetContext())
	}
	if rel.GetOptionalExpiration() != nil {
		out.ExpiresAt = rel.GetOptionalExpiration().AsTime()
	}
	return out
}

func filterToProto(filter authz.RelationshipFilter) *v1.RelationshipFilter {
	out := &v1.RelationshipFilter{ResourceType: filter.ResourceType, OptionalResourceId: filter.ResourceID, OptionalRelation: filter.Relation}
	if filter.SubjectType != "" {
		out.OptionalSubjectFilter = &v1.SubjectFilter{SubjectType: filter.SubjectType, OptionalSubjectId: filter.SubjectID, OptionalRelation: &v1.SubjectFilter_RelationFilter{Relation: filter.SubjectRel}}
	}
	return out
}

func consistencyToProto(consistency authz.Consistency, fullyConsistent bool) *v1.Consistency {
	if consistency.Mode == authz.ConsistencyFullyConsistent || fullyConsistent {
		return &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}}
	}
	if consistency.Mode == authz.ConsistencyAtLeastAsFresh && consistency.Token != "" {
		return &v1.Consistency{Requirement: &v1.Consistency_AtLeastAsFresh{AtLeastAsFresh: &v1.ZedToken{Token: consistency.Token}}}
	}
	return &v1.Consistency{Requirement: &v1.Consistency_MinimizeLatency{MinimizeLatency: true}}
}

func decisionFromProto(resp *v1.CheckPermissionResponse) authz.Decision {
	if resp == nil {
		return authz.NoMatch("spicedb returned empty decision")
	}
	switch resp.GetPermissionship() {
	case v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION:
		return authz.Allow("spicedb permission matched")
	case v1.CheckPermissionResponse_PERMISSIONSHIP_CONDITIONAL_PERMISSION:
		missing := []string(nil)
		if resp.GetPartialCaveatInfo() != nil {
			missing = append(missing, resp.GetPartialCaveatInfo().GetMissingRequiredContext()...)
		}
		return authz.Decision{Effect: authz.DecisionNoMatch, Allowed: false, Reason: "spicedb permission requires additional caveat context", Partial: true, MissingContext: missing}
	default:
		return authz.NoMatch("spicedb permission did not match")
	}
}

func attrsToStruct(attrs authz.AttributeSet) *structpb.Struct {
	if len(attrs) == 0 {
		return nil
	}
	s, err := structpb.NewStruct(map[string]any(attrs))
	if err != nil {
		return nil
	}
	return s
}

func structToAttrs(s *structpb.Struct) authz.AttributeSet {
	if s == nil {
		return nil
	}
	out := authz.AttributeSet{}
	for k, v := range s.AsMap() {
		out[k] = v
	}
	return out
}

func mergeAttrs(sets ...authz.AttributeSet) authz.AttributeSet {
	out := authz.AttributeSet{}
	for _, set := range sets {
		for k, v := range set {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tokenFromZed(token *v1.ZedToken) string {
	if token == nil {
		return ""
	}
	return token.GetToken()
}

func uint32FromInt(v int) uint32 {
	if v <= 0 {
		return 0
	}
	return uint32(v)
}

func cleanRelation(rel string) string { return strings.TrimSpace(rel) }

var _ = time.Time{}
