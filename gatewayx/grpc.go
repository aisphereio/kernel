package gatewayx

import (
	"context"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/contextx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// InternalJWTUnaryClientInterceptor injects a Gateway-signed internal JWT into
// outgoing gRPC calls. Use it for Gateway/BFF -> internal service calls when
// the backend service should trust the gateway rather than browser cookies.
func InternalJWTUnaryClientInterceptor(jwt *InternalJWT, audience string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = contextWithInternalJWT(ctx, jwt, audience)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// InternalJWTStreamClientInterceptor is the streaming counterpart of
// InternalJWTUnaryClientInterceptor.
func InternalJWTStreamClientInterceptor(jwt *InternalJWT, audience string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = contextWithInternalJWT(ctx, jwt, audience)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func contextWithInternalJWT(ctx context.Context, jwt *InternalJWT, audience string) context.Context {
	if ctx == nil || jwt == nil {
		return ctx
	}
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok {
		return ctx
	}
	reqID := contextx.RequestIDFromContext(ctx)
	decisionID := contextx.FromString(ctx, "authz_decision_id")
	token, err := jwt.Sign(audience, principal, reqID, decisionID, time.Now())
	if err != nil {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}
