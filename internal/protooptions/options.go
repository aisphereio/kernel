// Package protooptions contains low-level parsers for Aisphere protobuf custom
// options.
//
// The cmd generators intentionally parse unknown option payloads by extension
// number so they do not depend on already-generated Go code for the options
// protos during bootstrapping.
package protooptions

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	// google.api.http = 72295728, see google/api/annotations.proto.
	ExtGoogleHTTP protowire.Number = 72295728

	// aisphere.options.v1 method extensions, see api/aisphere/options/v1/authz.proto.
	ExtAuthz      protowire.Number = 51001
	ExtAudit      protowire.Number = 51002
	ExtCapability protowire.Number = 51003
)

type AuthzRule struct {
	Action   string
	Resource string
	Audience string
	Mode     int32
}

type AuditRule struct {
	Event string
	Risk  string
}

type CapabilityRule struct {
	Name  string
	Group string
}

func MethodUnknown(m *descriptorpb.MethodDescriptorProto) []byte {
	if m == nil || m.Options == nil {
		return nil
	}
	return m.Options.ProtoReflect().GetUnknown()
}

func HasExtension(b []byte, ext protowire.Number) bool {
	return len(FindExtensionPayloads(b, ext)) > 0
}

func FindExtensionPayloads(b []byte, ext protowire.Number) [][]byte {
	var payloads [][]byte
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return payloads
		}
		b = b[n:]
		if num == ext && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return payloads
			}
			payloads = append(payloads, append([]byte(nil), v...))
			b = b[n:]
			continue
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return payloads
		}
		b = b[n:]
	}
	return payloads
}

func LastExtensionPayload(b []byte, ext protowire.Number) ([]byte, bool) {
	payloads := FindExtensionPayloads(b, ext)
	if len(payloads) == 0 {
		return nil, false
	}
	return payloads[len(payloads)-1], true
}

func ParseAuthz(b []byte) AuthzRule {
	var o AuthzRule
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Action = v
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Resource = v
					b = b[n:]
					continue
				}
			}
		case 3:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Audience = v
					b = b[n:]
					continue
				}
			}
		case 4:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Mode = int32(v)
					b = b[n:]
					continue
				}
			}
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return o
		}
		b = b[n:]
	}
	return o
}

func ParseAudit(b []byte) AuditRule {
	var o AuditRule
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Event = v
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Risk = v
					b = b[n:]
					continue
				}
			}
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return o
		}
		b = b[n:]
	}
	return o
}

func ParseCapability(b []byte) CapabilityRule {
	var o CapabilityRule
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Name = v
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Group = v
					b = b[n:]
					continue
				}
			}
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return o
		}
		b = b[n:]
	}
	return o
}
