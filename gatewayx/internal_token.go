package gatewayx

import (
	"context"
	"net/http"
	"strings"

	"github.com/aisphereio/kernel/authn"
	httpx "github.com/aisphereio/kernel/transportx/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// InternalTokenHTTPMiddleware verifies the static Gateway -> backend internal
// token and then trusts the Gateway-injected X-Aisphere-* Principal headers.
// It is intentionally simple for first-phase deployments; add NetworkPolicy and
// mTLS later for stronger service identity.
func InternalTokenHTTPMiddleware(cfg authn.InternalServiceTokenConfig) httpx.FilterFunc {
	cfg = cfg.Normalized()
	authenticator := authn.TrustedHeaderAuthenticator{InternalToken: cfg}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			md := map[string]string{}
			for _, key := range authn.TrustedHeaderNames() {
				if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
					md[key] = value
				}
			}
			if value := strings.TrimSpace(r.Header.Get(cfg.Header())); value != "" {
				md[cfg.Header()] = value
			}
			principal, err := authenticator.Authenticate(r.Context(), authn.Credential{Scheme: authn.CredentialGatewayTrusted, Metadata: md})
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(authn.ContextWithPrincipal(r.Context(), principal)))
		})
	}
}

// InternalTokenUnaryServerInterceptor verifies static Gateway -> backend token
// for gRPC and injects the trusted Principal into context.
func InternalTokenUnaryServerInterceptor(cfg authn.InternalServiceTokenConfig) grpc.UnaryServerInterceptor {
	cfg = cfg.Normalized()
	authenticator := authn.TrustedHeaderAuthenticator{InternalToken: cfg}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		principal, err := principalFromIncomingMetadata(ctx, authenticator, cfg.Header())
		if err != nil {
			return nil, err
		}
		return handler(authn.ContextWithPrincipal(ctx, principal), req)
	}
}

// InternalTokenStreamServerInterceptor is the streaming counterpart of
// InternalTokenUnaryServerInterceptor.
func InternalTokenStreamServerInterceptor(cfg authn.InternalServiceTokenConfig) grpc.StreamServerInterceptor {
	cfg = cfg.Normalized()
	authenticator := authn.TrustedHeaderAuthenticator{InternalToken: cfg}
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		principal, err := principalFromIncomingMetadata(stream.Context(), authenticator, cfg.Header())
		if err != nil {
			return err
		}
		wrapped := &serverStreamWithContext{ServerStream: stream, ctx: authn.ContextWithPrincipal(stream.Context(), principal)}
		return handler(srv, wrapped)
	}
}

func principalFromIncomingMetadata(ctx context.Context, authenticator authn.TrustedHeaderAuthenticator, internalHeader string) (authn.Principal, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	meta := map[string]string{}
	for _, key := range authn.TrustedHeaderNames() {
		if values := md.Get(strings.ToLower(key)); len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			meta[key] = values[0]
		}
	}
	if values := md.Get(strings.ToLower(internalHeader)); len(values) > 0 && strings.TrimSpace(values[0]) != "" {
		meta[internalHeader] = values[0]
	}
	if values := md.Get(strings.ToLower(authn.InternalServiceTokenHeader)); len(values) > 0 && strings.TrimSpace(values[0]) != "" {
		meta[authn.InternalServiceTokenHeader] = values[0]
	}
	return authenticator.Authenticate(ctx, authn.Credential{Scheme: authn.CredentialGatewayTrusted, Metadata: meta})
}
