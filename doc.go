// Package kernel provides the lifecycle root for Aisphere Kernel.
//
// Kernel is a specification-driven microservice foundation. Business projects
// declare contracts in proto; Kernel tools check and generate runtime glue; and
// Kernel runtime packages provide fixed infrastructure for service assembly,
// request metadata, access control, traffic governance, observability and errors.
//
// Main runtime surfaces:
//
//   - configx: configuration loading and scanning
//   - logx: structured logging
//   - metricsx: metrics manager
//   - errorx: cross-protocol business errors
//   - serverx: service assembly and runtime autowire
//   - transportx/http and transportx/grpc: HTTP and gRPC transports
//   - requestx: request metadata shared by middleware and business logic
//   - accessx, authn, authz and auditx: identity, authorization and audit
//   - gatewayx: route manifests, route registries and gateway dispatch
//   - admissionx: mutating and validating admission chains
//   - ratelimitx and clientpolicyx: traffic governance contracts
//   - dbx, cachex, objectstorex and dtmx: data and transaction integrations
//
// Development tools live under cmd/. Business code must not import cmd/kernel,
// cmd/protoc-gen-* or cmd/buf-check-* packages directly.
//
// Scenario validation code was intentionally removed from the runtime tree. Add
// future scenario checks in a dedicated repository, behind explicit build tags,
// or as generated-layout tests rather than as framework runtime packages.
package kernel
