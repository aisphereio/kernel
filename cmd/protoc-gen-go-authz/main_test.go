package main

import (
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/aisphereio/kernel/internal/protooptions"
)

func TestParseMethodRuleFromUnknownOptions(t *testing.T) {
	plugin := newTestPlugin(t, testFileDescriptor(authzUnknown("skill.download", "skill:{skill_id}", "skill-service", 2)))
	file := plugin.Files[0]
	svc := file.Services[0]
	method := svc.Methods[0]

	r, ok := parseMethodRule(svc, method)
	if !ok {
		t.Fatal("expected authz rule")
	}
	if r.Service != "skill.v1.SkillService" || r.Method != "DownloadSkill" || r.FullMethod != "/skill.v1.SkillService/DownloadSkill" {
		t.Fatalf("unexpected method identity: %#v", r)
	}
	if r.Action != "skill.download" || r.Resource != "skill:{skill_id}" || r.Audience != "skill-service" || r.Mode != "SCOPED_TOKEN" {
		t.Fatalf("unexpected rule: %#v", r)
	}
	if r.AuditEvent != "skill.download" || r.AuditRisk != "medium" || r.Capability != "skill.download" {
		t.Fatalf("unexpected audit/capability: %#v", r)
	}
}

func TestModeName(t *testing.T) {
	tests := map[int]string{0: "UNSPECIFIED", 1: "CHECK_ONLY", 2: "SCOPED_TOKEN", 3: "SELF_CHECK", 99: "UNSPECIFIED"}
	for in, want := range tests {
		if got := modeName(in); got != want {
			t.Fatalf("modeName(%d) = %q, want %q", in, got, want)
		}
	}
}

func newTestPlugin(t *testing.T, files ...*descriptorpb.FileDescriptorProto) *protogen.Plugin {
	t.Helper()
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{files[0].GetName()},
		ProtoFile:      files,
	}
	plugin, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatal(err)
	}
	return plugin
}

func testFileDescriptor(unknown []byte) *descriptorpb.FileDescriptorProto {
	opts := &descriptorpb.MethodOptions{}
	opts.ProtoReflect().SetUnknown(unknown)
	return &descriptorpb.FileDescriptorProto{
		Name:    strPtr("skill.proto"),
		Package: strPtr("skill.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: strPtr("github.com/aisphereio/kernel/internal/test;testpb")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("DownloadSkillRequest"), Field: []*descriptorpb.FieldDescriptorProto{field("skill_id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "")}},
			{Name: strPtr("DownloadSkillReply")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: strPtr("SkillService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       strPtr("DownloadSkill"),
				InputType:  strPtr(".skill.v1.DownloadSkillRequest"),
				OutputType: strPtr(".skill.v1.DownloadSkillReply"),
				Options:    opts,
			}},
		}},
	}
}

func strPtr(v string) *string { return &v }
func int32Ptr(v int32) *int32 { return &v }
func labelPtr(v descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &v
}
func typePtr(v descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &v
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

func authzUnknown(action, resource, audience string, mode int32) []byte {
	var unknown []byte
	unknown = appendOption(unknown, protooptions.ExtAuthz, authzPayload(action, resource, audience, mode))
	unknown = appendOption(unknown, protooptions.ExtAudit, auditPayload("skill.download", "medium"))
	unknown = appendOption(unknown, protooptions.ExtCapability, capabilityPayload("download", "skill"))
	return unknown
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

func capabilityPayload(name, group string) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, name)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, group)
	return b
}
