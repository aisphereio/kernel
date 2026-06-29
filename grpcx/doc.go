// Package grpcx provides Kernel's standard gRPC runtime layer.
//
// grpcx is a thin facade over gRPC interceptors and selected middleware from
// github.com/grpc-ecosystem/go-grpc-middleware/v2. Business services should use
// grpcx helpers instead of assembling raw interceptors by hand so request
// context propagation, internal/scoped token handling, protovalidate, metrics,
// logging and recovery stay consistent across services.
package grpcx
