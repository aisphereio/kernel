package gatewayx

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	httpx "github.com/aisphereio/kernel/transportx/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// InternalJWTHTTPMiddleware verifies Gateway-signed internal JWTs on internal
// HTTP endpoints and injects the actor into authn context. It is intended for
// backend services that accept Gateway -> service HTTP calls during migration.
func InternalJWTHTTPMiddleware(jwt *InternalJWT, audience string) httpx.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := verifyBearer(r.Header.Get("Authorization"), jwt, audience)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := authn.ContextWithPrincipal(r.Context(), claims.Actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// InternalJWTUnaryServerInterceptor verifies Gateway-signed internal JWTs for
// backend gRPC services.
func InternalJWTUnaryServerInterceptor(jwt *InternalJWT, audience string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		claims, ok := verifyIncomingGRPC(ctx, jwt, audience)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid internal token")
		}
		ctx = authn.ContextWithPrincipal(ctx, claims.Actor)
		return handler(ctx, req)
	}
}

// InternalJWTStreamServerInterceptor verifies Gateway-signed internal JWTs for
// backend streaming gRPC services.
func InternalJWTStreamServerInterceptor(jwt *InternalJWT, audience string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		claims, ok := verifyIncomingGRPC(stream.Context(), jwt, audience)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing or invalid internal token")
		}
		wrapped := &serverStreamWithContext{ServerStream: stream, ctx: authn.ContextWithPrincipal(stream.Context(), claims.Actor)}
		return handler(srv, wrapped)
	}
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context { return s.ctx }

func verifyIncomingGRPC(ctx context.Context, jwt *InternalJWT, audience string) (InternalClaims, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return InternalClaims{}, false
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return InternalClaims{}, false
	}
	return verifyBearer(vals[0], jwt, audience)
}

func verifyBearer(header string, jwt *InternalJWT, audience string) (InternalClaims, bool) {
	if jwt == nil {
		return InternalClaims{}, false
	}
	token := strings.TrimSpace(header)
	if token == "" {
		return InternalClaims{}, false
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	claims, err := jwt.Verify(token, audience, time.Now())
	return claims, err == nil
}
