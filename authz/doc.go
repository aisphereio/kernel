// Package authz defines Kernel's authorization-provider boundary.
//
// The package owns only the question "can this subject do this action on this
// resource?". It supports RBAC as relationships, ReBAC as Zanzibar tuples and
// ABAC through request attributes/caveats, while keeping business code unaware
// of SpiceDB, OpenFGA, OPA or Casbin.
//
// Boundary rules:
//
//   - Business hot paths that only need a permission decision should depend on
//     Authorizer.
//   - Provisioning/control-plane paths may also use RelationshipWriter,
//     RelationshipReader and SchemaManager.
//   - Request-time orchestration of authentication, authorization, SkipPolicy
//     and audit belongs to accessx.Guard, not this package.
//   - AuditedAuthorizer is a low-level decorator for non-request-time checks;
//     normal HTTP/gRPC server paths should audit through accessx.Guard.
//
// Do not introduce a second runtime guard in authz. Generated policy contracts
// should resolve to accessx.Check/accessx.Guard or to the provider-neutral
// Authorizer interface.
package authz
