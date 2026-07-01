package gatewayx

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/errorx"
	transport "github.com/aisphereio/kernel/transportx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BindFunc converts a matched Gateway request into a strongly typed upstream
// request. Generated code should own this binding so business handlers do not
// parse HTTP paths, query strings or bodies by hand.
type BindFunc[Req any] func(DispatchRequest, RouteMatch) (Req, error)

// OperationInvoker invokes one generated upstream operation.
type OperationInvoker interface {
	InvokeOperation(ctx context.Context, match RouteMatch, req DispatchRequest) (DispatchResponse, error)
}

// UnaryHandler is the in-process shape used by validation tests and local
// demos. It mirrors generated gRPC unary clients without requiring a network
// listener.
type UnaryHandler[Req any, Reply any] func(context.Context, Req) (Reply, error)

// UnaryRPC is the generated gRPC client method shape, for example
// SkillServiceClient.GetSkill(ctx, req, opts...).
type UnaryRPC[Req any, Reply any] func(context.Context, Req, ...grpc.CallOption) (Reply, error)

// UnaryInvoker adapts a generated bind function and an in-process service call
// into a Gateway upstream operation invoker.
func UnaryInvoker[Req any, Reply any](bind BindFunc[Req], call UnaryHandler[Req, Reply]) OperationInvoker {
	return unaryInvoker[Req, Reply]{bind: bind, call: call}
}

// GRPCUnaryInvoker adapts a generated bind function and a generated gRPC client
// method into a Gateway upstream operation invoker.
func GRPCUnaryInvoker[Req any, Reply any](bind BindFunc[Req], call UnaryRPC[Req, Reply], opts ...grpc.CallOption) OperationInvoker {
	return grpcUnaryInvoker[Req, Reply]{bind: bind, call: call, opts: opts}
}

type unaryInvoker[Req any, Reply any] struct {
	bind BindFunc[Req]
	call UnaryHandler[Req, Reply]
}

func (i unaryInvoker[Req, Reply]) InvokeOperation(ctx context.Context, match RouteMatch, req DispatchRequest) (DispatchResponse, error) {
	if i.bind == nil || i.call == nil {
		return DispatchResponse{}, fmt.Errorf("gatewayx: nil unary invoker")
	}
	in, err := i.bind(req, match)
	if err != nil {
		return DispatchResponse{Status: errorx.HTTPStatusOf(err)}, err
	}
	out, err := i.call(GRPCServerContextFromDispatch(ctx, req, match), in)
	if err != nil {
		return DispatchResponse{Status: errorx.HTTPStatusOf(err)}, err
	}
	return DispatchResponse{Status: 200, Body: out}, nil
}

type grpcUnaryInvoker[Req any, Reply any] struct {
	bind BindFunc[Req]
	call UnaryRPC[Req, Reply]
	opts []grpc.CallOption
}

func (i grpcUnaryInvoker[Req, Reply]) InvokeOperation(ctx context.Context, match RouteMatch, req DispatchRequest) (DispatchResponse, error) {
	if i.bind == nil || i.call == nil {
		return DispatchResponse{}, fmt.Errorf("gatewayx: nil grpc unary invoker")
	}
	in, err := i.bind(req, match)
	if err != nil {
		return DispatchResponse{Status: errorx.HTTPStatusOf(err)}, err
	}
	out, err := i.call(GRPCClientContextFromDispatch(ctx, req, match), in, i.opts...)
	if err != nil {
		return DispatchResponse{Status: errorx.HTTPStatusOf(err)}, err
	}
	return DispatchResponse{Status: 200, Body: out}, nil
}

// InvokerRegistry routes matched Gateway operations to generated operation
// invokers. It implements UpstreamInvoker so Dispatcher can use it directly.
type InvokerRegistry struct{ operations map[string]OperationInvoker }

func NewInvokerRegistry() *InvokerRegistry {
	return &InvokerRegistry{operations: map[string]OperationInvoker{}}
}

func (r *InvokerRegistry) Register(operation string, invoker OperationInvoker) error {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return fmt.Errorf("gatewayx: operation is required")
	}
	if invoker == nil {
		return fmt.Errorf("gatewayx: invoker is required for %s", operation)
	}
	if r.operations == nil {
		r.operations = map[string]OperationInvoker{}
	}
	r.operations[operation] = invoker
	return nil
}

func (r *InvokerRegistry) Invoke(ctx context.Context, match RouteMatch, req DispatchRequest, target string) (DispatchResponse, error) {
	_ = target // target is used by HTTP reverse-proxy implementations; generated gRPC dispatch uses operation.
	if r == nil {
		return DispatchResponse{}, fmt.Errorf("gatewayx: nil invoker registry")
	}
	operation := strings.TrimSpace(match.Route.Upstream.Operation)
	invoker := r.operations[operation]
	if invoker == nil {
		return DispatchResponse{Status: 502}, errorx.Unavailable("UPSTREAM_INVOKER_NOT_FOUND", fmt.Sprintf("no generated invoker for %s", operation))
	}
	return invoker.InvokeOperation(ctx, match, req)
}

// GRPCClientContextFromDispatch copies Gateway request headers into outgoing
// gRPC metadata before calling a generated upstream client. Real backend gRPC
// servers then reconstruct Kernel transport context from incoming metadata.
func GRPCClientContextFromDispatch(parent context.Context, req DispatchRequest, match RouteMatch) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	md := metadata.MD{}
	for k, v := range req.Headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		md.Set(strings.ToLower(k), v)
	}
	md.Set("x-kernel-operation", match.Route.Upstream.Operation)
	return metadata.NewOutgoingContext(parent, md)
}

// GRPCServerContextFromIncomingMetadata reconstructs Kernel's transport context
// from incoming gRPC metadata. Lightweight/generated validation servers can use
// this when they do not run through Kernel's managed transportx/grpc.Server.
func GRPCServerContextFromIncomingMetadata(parent context.Context, operation string) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	md, _ := metadata.FromIncomingContext(parent)
	header := dispatchHeader{}
	for k, values := range md {
		for _, v := range values {
			header.Add(k, v)
		}
	}
	if operation == "" {
		operation = header.Get("x-kernel-operation")
	}
	return transport.NewServerContext(parent, dispatchTransport{header: header, operation: operation})
}

// GRPCServerContextFromDispatch builds the downstream server context used by
// generated in-process invokers. It preserves authorization, request id and
// tracing headers so the backend Kernel server chain performs authoritative
// authn/authz/audit exactly as a real gRPC call would.
func GRPCServerContextFromDispatch(parent context.Context, req DispatchRequest, match RouteMatch) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	h := dispatchHeader{}
	for k, v := range req.Headers {
		h.Set(k, v)
	}
	return transport.NewServerContext(parent, dispatchTransport{header: h, operation: match.Route.Upstream.Operation})
}

type dispatchTransport struct {
	header    dispatchHeader
	operation string
}

func (dispatchTransport) Kind() transport.Kind              { return transport.KindGRPC }
func (dispatchTransport) Endpoint() string                  { return "gatewayx://generated" }
func (t dispatchTransport) Operation() string               { return t.operation }
func (t dispatchTransport) RequestHeader() transport.Header { return t.header }
func (dispatchTransport) ReplyHeader() transport.Header     { return dispatchHeader{} }

type dispatchHeader map[string][]string

func (h dispatchHeader) Get(key string) string {
	values := h.Values(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func (h dispatchHeader) Set(key, value string) { h[key] = []string{value} }
func (h dispatchHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h dispatchHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h dispatchHeader) Values(key string) []string {
	if v, ok := h[key]; ok {
		return v
	}
	for k, v := range h {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return nil
}
