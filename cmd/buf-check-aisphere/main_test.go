package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/aisphereio/kernel/internal/protooptions"
)

func TestAnalyzeRequiresAuthzForExternalHTTP(t *testing.T) {
	set := descriptorSet(methodWithUnknown("GetSkill", ".skill.v1.GetSkillRequest", appendOption(nil, protooptions.ExtGoogleHTTP, nil)))

	diags := analyze(set, checkConfig{HighRiskActions: splitCSV("delete")})

	assertDiagContains(t, diags, "external google.api.http method must declare aisphere.authz")
}

func TestAnalyzeValidatesAuthzAuditAndResourceTemplate(t *testing.T) {
	unknown := appendOption(nil, protooptions.ExtGoogleHTTP, nil)
	unknown = appendOption(unknown, protooptions.ExtAuthz, authzPayload("skill.delete", "skill:{missing_id}", "skill-service", 2))
	set := descriptorSet(methodWithUnknown("DeleteSkill", ".skill.v1.GetSkillRequest", unknown))

	diags := analyze(set, checkConfig{HighRiskActions: splitCSV("delete")})

	assertDiagContains(t, diags, "high-risk authz action \"skill.delete\" must declare aisphere.audit")
	assertDiagContains(t, diags, "authz resource template references missing request field {missing_id}")
}

func TestAnalyzeAcceptsNestedResourceTemplateAndAudit(t *testing.T) {
	unknown := appendOption(nil, protooptions.ExtGoogleHTTP, nil)
	unknown = appendOption(unknown, protooptions.ExtAuthz, authzPayload("skill.download", "skill:{owner.id}:{skill_id}", "skill-service", 2))
	unknown = appendOption(unknown, protooptions.ExtAudit, auditPayload("skill.download", "medium"))
	set := descriptorSet(methodWithUnknown("DownloadSkill", ".skill.v1.GetSkillRequest", unknown))

	diags := analyze(set, checkConfig{HighRiskActions: splitCSV("delete")})

	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diags)
	}
}

func assertDiagContains(t *testing.T, diags []string, want string) {
	t.Helper()
	for _, diag := range diags {
		if strings.Contains(diag, want) {
			return
		}
	}
	t.Fatalf("diagnostics missing %q: %#v", want, diags)
}

func descriptorSet(method *descriptorpb.MethodDescriptorProto) *descriptorpb.FileDescriptorSet {
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		{
			Name:    strPtr("skill.proto"),
			Package: strPtr("skill.v1"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: strPtr("Owner"),
					Field: []*descriptorpb.FieldDescriptorProto{
						field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
					},
				},
				{
					Name: strPtr("GetSkillRequest"),
					Field: []*descriptorpb.FieldDescriptorProto{
						field("skill_id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
						field("owner", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".skill.v1.Owner"),
					},
				},
			},
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name:   strPtr("SkillService"),
					Method: []*descriptorpb.MethodDescriptorProto{method},
				},
			},
		},
	}}
}

func methodWithUnknown(name, inputType string, unknown []byte) *descriptorpb.MethodDescriptorProto {
	opts := &descriptorpb.MethodOptions{}
	opts.ProtoReflect().SetUnknown(unknown)
	return &descriptorpb.MethodDescriptorProto{
		Name:       strPtr(name),
		InputType:  strPtr(inputType),
		OutputType: strPtr(".skill.v1.GetSkillReply"),
		Options:    opts,
	}
}

func field(name string, num int32, typ descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:   strPtr(name),
		Number: int32Ptr(num),
		Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
		Type:   typePtr(typ),
	}
	if typeName != "" {
		f.TypeName = strPtr(typeName)
	}
	return f
}

func strPtr(v string) *string { return &v }
func int32Ptr(v int32) *int32 { return &v }
func labelPtr(v descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &v
}
func typePtr(v descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &v
}

func appendOption(dst []byte, ext protowire.Number, payload []byte) []byte {
	dst = protowire.AppendTag(dst, ext, protowire.BytesType)
	return protowire.AppendBytes(dst, payload)
}

func authzPayload(action, resource, audience string, mode int32) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, action)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, resource)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendString(b, audience)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(mode))
	return b
}

func auditPayload(event, risk string) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, event)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, risk)
	return b
}
