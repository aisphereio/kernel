// Package requestx provides the canonical request metadata model used by Kernel
// middleware. It is inspired by Kubernetes apiserver RequestInfo: every request
// is normalized once, then authn/authz/audit/ratelimit/log/metrics/trace stages
// read the same contract instead of each transport inventing its own metadata.
package requestx

import (
	"context"
	"strings"

	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/contextx"
	transport "github.com/aisphereio/kernel/transportx"
)

// Protocol identifies the inbound or outbound protocol.
type Protocol string

const (
	ProtocolUnknown Protocol = "unknown"
	ProtocolHTTP    Protocol = "http"
	ProtocolGRPC    Protocol = "grpc"
)

// Direction identifies whether the request is entering this service or calling a downstream.
type Direction string

const (
	DirectionUnknown Direction = "unknown"
	DirectionServer  Direction = "server"
	DirectionClient  Direction = "client"
)

// Info is Kernel's normalized request metadata. Middleware should enrich and
// consume this value instead of re-parsing operation strings and headers.
type Info struct {
	Direction Direction
	Protocol  Protocol

	Service   string
	Method    string
	Operation string
	Endpoint  string

	Exposure accessv1.Exposure
	Action   string
	Resource string

	TenantID  string
	RequestID string
	TraceID   string
	UserID    string

	CallerService string
	TargetService string

	IsSystem      bool
	IsInternal    bool
	IsLongRunning bool

	Labels map[string]string
}

// Clone returns a deep-enough copy safe for middleware enrichment.
func (i Info) Clone() Info {
	out := i
	if i.Labels != nil {
		out.Labels = make(map[string]string, len(i.Labels))
		for k, v := range i.Labels {
			out.Labels[k] = v
		}
	}
	return out
}

// Normalize fills derived fields and safe defaults.
func (i Info) Normalize() Info {
	if i.Direction == "" {
		i.Direction = DirectionUnknown
	}
	if i.Protocol == "" {
		i.Protocol = ProtocolUnknown
	}
	if i.Operation != "" && (i.Service == "" || i.Method == "") {
		svc, method := splitOperation(i.Operation)
		if i.Service == "" {
			i.Service = svc
		}
		if i.Method == "" {
			i.Method = method
		}
	}
	switch i.Exposure {
	case accessv1.Exposure_SYSTEM:
		i.IsSystem = true
	case accessv1.Exposure_INTERNAL:
		i.IsInternal = true
	}
	return i
}

// Merge overlays non-zero fields from other onto i.
func (i Info) Merge(other Info) Info {
	out := i.Clone()
	if other.Direction != "" {
		out.Direction = other.Direction
	}
	if other.Protocol != "" {
		out.Protocol = other.Protocol
	}
	if other.Service != "" {
		out.Service = other.Service
	}
	if other.Method != "" {
		out.Method = other.Method
	}
	if other.Operation != "" {
		out.Operation = other.Operation
	}
	if other.Endpoint != "" {
		out.Endpoint = other.Endpoint
	}
	if other.Exposure != accessv1.Exposure_EXPOSURE_UNSPECIFIED {
		out.Exposure = other.Exposure
	}
	if other.Action != "" {
		out.Action = other.Action
	}
	if other.Resource != "" {
		out.Resource = other.Resource
	}
	if other.TenantID != "" {
		out.TenantID = other.TenantID
	}
	if other.RequestID != "" {
		out.RequestID = other.RequestID
	}
	if other.TraceID != "" {
		out.TraceID = other.TraceID
	}
	if other.UserID != "" {
		out.UserID = other.UserID
	}
	if other.CallerService != "" {
		out.CallerService = other.CallerService
	}
	if other.TargetService != "" {
		out.TargetService = other.TargetService
	}
	if other.IsSystem {
		out.IsSystem = true
	}
	if other.IsInternal {
		out.IsInternal = true
	}
	if other.IsLongRunning {
		out.IsLongRunning = true
	}
	if len(other.Labels) > 0 {
		if out.Labels == nil {
			out.Labels = map[string]string{}
		}
		for k, v := range other.Labels {
			out.Labels[k] = v
		}
	}
	return out.Normalize()
}

// Resolver maps ctx/operation/req into request metadata. Code generators should
// implement this from proto access policy. Runtime middleware can chain multiple resolvers.
type Resolver func(ctx context.Context, operation string, req any) (Info, bool, error)

type contextKey struct{}

// NewContext attaches Info to ctx.
func NewContext(ctx context.Context, info Info) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, info.Normalize())
}

// FromContext returns Info from ctx.
func FromContext(ctx context.Context) (Info, bool) {
	if ctx == nil {
		return Info{}, false
	}
	info, ok := ctx.Value(contextKey{}).(Info)
	if !ok {
		return Info{}, false
	}
	return info.Normalize(), true
}

// FromContextOr returns Info from ctx or fallback.
func FromContextOr(ctx context.Context, fallback Info) Info {
	if info, ok := FromContext(ctx); ok {
		return info
	}
	return fallback.Normalize()
}

// EnrichFromContext copies canonical contextx fields into info when missing.
func EnrichFromContext(ctx context.Context, info Info) Info {
	if info.RequestID == "" {
		info.RequestID = contextx.RequestIDFromContext(ctx)
	}
	if info.TraceID == "" {
		info.TraceID = contextx.TraceIDFromContext(ctx)
	}
	if info.TenantID == "" {
		info.TenantID = contextx.TenantFromContext(ctx)
	}
	if info.UserID == "" {
		info.UserID = contextx.SubjectIDFromContext(ctx)
	}
	return info.Normalize()
}

// FromTransport builds base Info from transport context.
func FromTransport(ctx context.Context, direction Direction) (Info, bool) {
	var tr transport.Transporter
	var ok bool
	if direction == DirectionClient {
		tr, ok = transport.FromClientContext(ctx)
	} else {
		tr, ok = transport.FromServerContext(ctx)
	}
	if !ok || tr == nil {
		return Info{}, false
	}
	protocol := ProtocolUnknown
	switch tr.Kind() {
	case transport.KindGRPC:
		protocol = ProtocolGRPC
	case transport.KindHTTP:
		protocol = ProtocolHTTP
	}
	info := Info{Direction: direction, Protocol: protocol, Operation: tr.Operation(), Endpoint: tr.Endpoint()}.Normalize()
	return EnrichFromContext(ctx, info), true
}

// OperationKey returns a stable operation/method key for rate limit, metrics and logs.
func OperationKey(ctx context.Context) string {
	if info, ok := FromContext(ctx); ok {
		if info.Operation != "" {
			return info.Operation
		}
		if info.Service != "" && info.Method != "" {
			return info.Service + "/" + info.Method
		}
		if info.Method != "" {
			return info.Method
		}
	}
	if info, ok := FromTransport(ctx, DirectionClient); ok && info.Operation != "" {
		return info.Operation
	}
	if info, ok := FromTransport(ctx, DirectionServer); ok && info.Operation != "" {
		return info.Operation
	}
	return "_operation"
}

func splitOperation(op string) (string, string) {
	op = strings.TrimSpace(op)
	op = strings.TrimPrefix(op, "/")
	if op == "" {
		return "", ""
	}
	parts := strings.Split(op, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", parts[len(parts)-1]
}
