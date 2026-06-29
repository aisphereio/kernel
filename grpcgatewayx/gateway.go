// Package grpcgatewayx provides small helpers for mounting grpc-gateway
// generated reverse-proxy handlers inside Kernel gateway/BFF services.
//
// It intentionally does not replace transportx/http or protoc-gen-go-http.
// Use it when a service is proto-first internally and wants to expose
// HTTP/JSON through grpc-gateway generated *.gw.pb.go files.
package grpcgatewayx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RegisterHandlerFromEndpoint matches grpc-gateway generated functions such as:
//
//	RegisterGreeterHandlerFromEndpoint(ctx, mux, endpoint, opts)
//
// The generated function is emitted by protoc-gen-grpc-gateway into *.gw.pb.go.
type RegisterHandlerFromEndpoint func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// RegisterHandlerFromConn matches grpc-gateway generated functions such as:
//
//	RegisterGreeterHandler(ctx, mux, conn)
//
// Use this when the caller owns the gRPC ClientConn lifecycle.
type RegisterHandlerFromConn func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

// HandlerConfig configures a grpc-gateway runtime mux.
type HandlerConfig struct {
	// Endpoint is the gRPC upstream address used by RegisterHandlerFromEndpoint,
	// for example "127.0.0.1:9000" or "skill-service:9000".
	Endpoint string

	// DialOptions are passed to grpc.DialContext by grpc-gateway. If empty,
	// WithInsecure defaults to true for local/internal development. Production
	// services should pass TLS/mTLS dial options explicitly.
	DialOptions []grpc.DialOption

	// MuxOptions are passed to runtime.NewServeMux.
	MuxOptions []runtime.ServeMuxOption

	// DialTimeout bounds grpc-gateway's initial connection attempt when using
	// RegisterHandlerFromEndpoint. Zero means no additional timeout is applied.
	DialTimeout time.Duration
}

// NewServeMux creates a grpc-gateway ServeMux. Exposed to keep generated
// registration explicit and testable.
func NewServeMux(opts ...runtime.ServeMuxOption) *runtime.ServeMux {
	return runtime.NewServeMux(opts...)
}

// NewHandlerFromEndpoint builds an http.Handler by registering grpc-gateway
// generated handlers against a gRPC endpoint.
func NewHandlerFromEndpoint(ctx context.Context, cfg HandlerConfig, register RegisterHandlerFromEndpoint) (http.Handler, error) {
	if register == nil {
		return nil, errors.New("grpcgatewayx: nil register function")
	}
	if cfg.Endpoint == "" {
		return nil, errors.New("grpcgatewayx: empty endpoint")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.DialTimeout)
		defer cancel()
	}
	mux := NewServeMux(DefaultMuxOptions(cfg.MuxOptions...)...)
	dialOptions := cfg.DialOptions
	if len(dialOptions) == 0 {
		dialOptions = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	if err := register(ctx, mux, cfg.Endpoint, dialOptions); err != nil {
		return nil, err
	}
	return mux, nil
}

// NewHandlerFromConn builds an http.Handler by registering grpc-gateway
// generated handlers against an existing gRPC ClientConn.
func NewHandlerFromConn(ctx context.Context, conn *grpc.ClientConn, opts []runtime.ServeMuxOption, register RegisterHandlerFromConn) (http.Handler, error) {
	if register == nil {
		return nil, errors.New("grpcgatewayx: nil register function")
	}
	if conn == nil {
		return nil, errors.New("grpcgatewayx: nil grpc conn")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	mux := NewServeMux(DefaultMuxOptions(opts...)...)
	if err := register(ctx, mux, conn); err != nil {
		return nil, err
	}
	return mux, nil
}
