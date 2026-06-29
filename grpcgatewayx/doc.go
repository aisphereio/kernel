// Package grpcgatewayx contains runtime helpers for grpc-gateway generated
// *.gw.pb.go handlers.
//
// Recommended Kernel pattern:
//
//   - Use proto + google.api.http as the API contract.
//   - Generate *.pb.go, *_grpc.pb.go and *.gw.pb.go.
//   - Mount the generated gateway handler behind restx/gatewayx filters.
//   - Keep business authz explicit in Gateway/BFF logic for complex routes.
//
// grpc-gateway is best for REST/JSON-to-gRPC protocol translation. It is not a
// replacement for Kernel authn/authz/audit/session/gateway policy layers.
package grpcgatewayx
