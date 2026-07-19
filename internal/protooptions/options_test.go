package protooptions

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestParseAuthzAuditCapability(t *testing.T) {
	unknown := appendOption(nil, ExtAuthz, authzPayload("skill.download", "skill:{skill_id}", "skill-service", 2))
	unknown = appendOption(unknown, ExtAudit, auditPayload("skill.package.download", "medium"))
	unknown = appendOption(unknown, ExtCapability, capabilityPayload("download", "skill"))

	authzRaw, ok := LastExtensionPayload(unknown, ExtAuthz)
	if !ok {
		t.Fatal("authz payload missing")
	}
	authz := ParseAuthz(authzRaw)
	if authz.Action != "skill.download" || authz.Resource != "skill:{skill_id}" || authz.Audience != "skill-service" || authz.Mode != 2 {
		t.Fatalf("unexpected authz: %#v", authz)
	}

	audit := ParseAudit(mustLast(t, unknown, ExtAudit))
	if audit.Event != "skill.package.download" || audit.Risk != "medium" {
		t.Fatalf("unexpected audit: %#v", audit)
	}

	capability := ParseCapability(mustLast(t, unknown, ExtCapability))
	if capability.Group != "skill" || capability.Name != "download" {
		t.Fatalf("unexpected capability: %#v", capability)
	}
}

func TestParseAccessPolicyGatewayUpstreamService(t *testing.T) {
	var gateway []byte
	gateway = protowire.AppendTag(gateway, 4, protowire.BytesType)
	gateway = protowire.AppendString(gateway, "hub-service")

	var payload []byte
	payload = protowire.AppendTag(payload, 1, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 2)
	payload = protowire.AppendTag(payload, 7, protowire.BytesType)
	payload = protowire.AppendBytes(payload, gateway)

	policy := ParseAccessPolicy(payload)
	if policy.Exposure != 2 || policy.Gateway.UpstreamService != "hub-service" {
		t.Fatalf("unexpected access policy: %#v", policy)
	}
}

func mustLast(t *testing.T, unknown []byte, ext protowire.Number) []byte {
	t.Helper()
	b, ok := LastExtensionPayload(unknown, ext)
	if !ok {
		t.Fatalf("missing extension %d", ext)
	}
	return b
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
