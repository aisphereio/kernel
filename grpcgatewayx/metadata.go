package grpcgatewayx

import (
	"context"
	"net/http"
	"strings"

	"github.com/aisphereio/kernel/contextx"
	"google.golang.org/grpc/metadata"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

const (
	HeaderRequestID       = "X-Request-Id"
	HeaderTraceID         = "X-Trace-Id"
	HeaderAuthzDecisionID = "X-Authz-Decision-Id"
	HeaderTraceparent     = "Traceparent"
	HeaderBaggage         = "Baggage"
)

// DefaultMuxOptions returns gateway mux options for Kernel Gateway/BFF services.
// It propagates trusted contextx values into the outgoing gRPC context. It does
// not forward external Authorization or external authz-decision headers by
// default; Gateway code should create scoped/internal tokens explicitly.
func DefaultMuxOptions(extra ...runtime.ServeMuxOption) []runtime.ServeMuxOption {
	opts := []runtime.ServeMuxOption{
		runtime.WithMetadata(kernelMetadata),
		runtime.WithIncomingHeaderMatcher(kernelHeaderMatcher),
	}
	return append(opts, extra...)
}

func kernelMetadata(ctx context.Context, r *http.Request) metadata.MD {
	pairs := []string{}
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			pairs = append(pairs, strings.ToLower(k), v)
		}
	}
	add(HeaderRequestID, firstNonEmpty(contextx.RequestIDFromContext(ctx), r.Header.Get(HeaderRequestID)))
	add(HeaderTraceID, firstNonEmpty(contextx.TraceIDFromContext(ctx), r.Header.Get(HeaderTraceID)))
	add(HeaderTraceparent, r.Header.Get(HeaderTraceparent))
	add(HeaderBaggage, r.Header.Get(HeaderBaggage))
	add(HeaderAuthzDecisionID, contextx.AuthzDecisionIDFromContext(ctx))
	if token := firstNonEmpty(contextx.ScopedTokenFromContext(ctx), contextx.InternalTokenFromContext(ctx)); token != "" {
		add("authorization", bearer(token))
	}
	return metadata.Pairs(pairs...)
}

func kernelHeaderMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case strings.ToLower(HeaderRequestID), strings.ToLower(HeaderTraceID), strings.ToLower(HeaderTraceparent), strings.ToLower(HeaderBaggage):
		return strings.ToLower(key), true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
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
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}
