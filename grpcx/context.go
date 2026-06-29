package grpcx

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/contextx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryContextClientInterceptor propagates trusted request context values from
// contextx into outgoing gRPC metadata. It never forwards browser cookies; if a
// scoped/internal token exists in contextx, it is sent as Authorization.
func UnaryContextClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = OutgoingContext(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamContextClientInterceptor is the streaming counterpart of
// UnaryContextClientInterceptor.
func StreamContextClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = OutgoingContext(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// UnaryContextServerInterceptor restores contextx values from incoming gRPC
// metadata. Authentication tokens are only copied into contextx as opaque
// scoped/internal token values; verification belongs to auth interceptors.
func UnaryContextServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = IncomingContext(ctx)
		return handler(ctx, req)
	}
}

// StreamContextServerInterceptor restores contextx values for streaming RPCs.
func StreamContextServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := IncomingContext(stream.Context())
		return handler(srv, &serverStream{ServerStream: stream, ctx: ctx})
	}
}

// OutgoingContext copies contextx values to outgoing metadata.
func OutgoingContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	pairs := make([]string, 0, 8)
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			pairs = append(pairs, k, v)
		}
	}
	add(HeaderRequestID, contextx.RequestIDFromContext(ctx))
	add(HeaderTraceID, contextx.TraceIDFromContext(ctx))
	add(HeaderAuthzDecisionID, contextx.AuthzDecisionIDFromContext(ctx))
	if token := firstNonEmpty(contextx.ScopedTokenFromContext(ctx), contextx.InternalTokenFromContext(ctx)); token != "" {
		add(HeaderAuthorization, bearer(token))
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// IncomingContext copies selected incoming metadata into contextx.
func IncomingContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	if v := first(md.Get(HeaderRequestID)); v != "" {
		ctx = contextx.WithRequestID(ctx, v)
	}
	if v := firstNonEmpty(first(md.Get(HeaderTraceID)), traceIDFromTraceparent(first(md.Get(HeaderTraceparent)))); v != "" {
		ctx = contextx.WithTraceID(ctx, v)
	}
	if v := first(md.Get(HeaderAuthzDecisionID)); v != "" {
		ctx = contextx.WithAuthzDecisionID(ctx, v)
	}
	if v := first(md.Get(HeaderAuthorization)); v != "" {
		token := trimBearer(v)
		ctx = contextx.WithScopedToken(ctx, token)
	}
	return ctx
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func bearer(token string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(token)), "bearer ") {
		return strings.TrimSpace(token)
	}
	return "Bearer " + strings.TrimSpace(token)
}

func trimBearer(header string) string {
	header = strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func traceIDFromTraceparent(v string) string {
	parts := strings.Split(v, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStream) Context() context.Context { return s.ctx }
