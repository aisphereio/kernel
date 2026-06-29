package grpcx

import (
	"context"
	"net/http"

	"buf.build/go/protovalidate"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// NewValidator creates a protovalidate validator for services that want to
// validate protobuf request messages at HTTP, gRPC and BFF boundaries.
func NewValidator(opts ...protovalidate.ValidatorOption) (protovalidate.Validator, error) {
	return protovalidate.New(opts...)
}

// UnaryProtovalidateServerInterceptor validates inbound unary protobuf
// requests with Buf Protovalidate annotations.
func UnaryProtovalidateServerInterceptor(v protovalidate.Validator) grpc.UnaryServerInterceptor {
	if v == nil {
		return passthroughUnary()
	}
	return protovalidate_middleware.UnaryServerInterceptor(v)
}

// StreamProtovalidateServerInterceptor validates inbound streaming protobuf
// requests with Buf Protovalidate annotations.
func StreamProtovalidateServerInterceptor(v protovalidate.Validator) grpc.StreamServerInterceptor {
	if v == nil {
		return passthroughStream()
	}
	return protovalidate_middleware.StreamServerInterceptor(v)
}

// ValidateProto validates a single protobuf message. Use this from generated
// go-http handlers or BFF logic after decoding the request and before authz or
// business logic.
func ValidateProto(v protovalidate.Validator, msg proto.Message) error {
	if v == nil || msg == nil {
		return nil
	}
	return v.Validate(msg)
}

// HTTPValidate is a tiny adapter for BFF/go-http handlers that decode protobuf
// request messages before calling business logic.
func HTTPValidate(v protovalidate.Validator, msg proto.Message, w http.ResponseWriter) bool {
	if err := ValidateProto(v, msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func passthroughUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}

func passthroughStream() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, stream)
	}
}
