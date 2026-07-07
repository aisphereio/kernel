// Package protooptions contains low-level parsers for Aisphere protobuf custom
// options.
//
// The cmd generators intentionally parse unknown option payloads by extension
// number so they do not depend on already-generated Go code for the options
// protos during bootstrapping.
package protooptions

import (
	"math"

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

	// aisphere.access.v1.policy, see api/aisphere/access/v1/access.proto.
	ExtAccess protowire.Number = 51010
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

// AccessPolicy is a low-level representation of aisphere.access.v1.AccessPolicy.
// The parser keeps cmd tools independent from generated option code.
type AccessPolicy struct {
	Exposure  int32
	Authz     AuthzRule
	Audit     AccessAudit
	RateLimit AccessRateLimit
	Breaker   AccessBreaker
	Reason    string
	Gateway   AccessGateway
}

type AccessAudit struct {
	Enabled bool
	Event   string
	Risk    string
}

type AccessRateLimit struct {
	Enabled bool
	Key     string
	QPS     float64
	Burst   int32
	Backend int32
	Scope   int32
}

type AccessBreaker struct {
	Enabled bool
	Name    string
}

type AccessGateway struct {
	Profiles []string
	Tags     []string
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

func ParseAccessPolicy(b []byte) AccessPolicy {
	var o AccessPolicy
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Exposure = int32(v)
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n >= 0 {
					o.Authz = ParseAccessAuthz(v)
					b = b[n:]
					continue
				}
			}
		case 3:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n >= 0 {
					o.Audit = ParseAccessAudit(v)
					b = b[n:]
					continue
				}
			}
		case 4:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n >= 0 {
					o.RateLimit = ParseAccessRateLimit(v)
					b = b[n:]
					continue
				}
			}
		case 5:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n >= 0 {
					o.Breaker = ParseAccessBreaker(v)
					b = b[n:]
					continue
				}
			}
		case 6:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Reason = v
					b = b[n:]
					continue
				}
			}
		case 7:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeBytes(b)
				if n >= 0 {
					o.Gateway = ParseAccessGateway(v)
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

func ParseAccessAuthz(b []byte) AuthzRule { return ParseAuthz(b) }

func ParseAccessAudit(b []byte) AccessAudit {
	var o AccessAudit
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Enabled = v != 0
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Event = v
					b = b[n:]
					continue
				}
			}
		case 3:
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

func ParseAccessRateLimit(b []byte) AccessRateLimit {
	var o AccessRateLimit
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Enabled = v != 0
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Key = v
					b = b[n:]
					continue
				}
			}
		case 3:
			if typ == protowire.Fixed64Type {
				v, n := protowire.ConsumeFixed64(b)
				if n >= 0 {
					o.QPS = math.Float64frombits(v)
					b = b[n:]
					continue
				}
			}
		case 4:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Burst = int32(v)
					b = b[n:]
					continue
				}
			}
		case 5:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Backend = int32(v)
					b = b[n:]
					continue
				}
			}
		case 6:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Scope = int32(v)
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

func ParseAccessBreaker(b []byte) AccessBreaker {
	var o AccessBreaker
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 1:
			if typ == protowire.VarintType {
				v, n := protowire.ConsumeVarint(b)
				if n >= 0 {
					o.Enabled = v != 0
					b = b[n:]
					continue
				}
			}
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Name = v
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

func ParseAccessGateway(b []byte) AccessGateway {
	var o AccessGateway
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return o
		}
		b = b[n:]
		switch num {
		case 2:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Profiles = append(o.Profiles, v)
					b = b[n:]
					continue
				}
			}
		case 3:
			if typ == protowire.BytesType {
				v, n := protowire.ConsumeString(b)
				if n >= 0 {
					o.Tags = append(o.Tags, v)
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
