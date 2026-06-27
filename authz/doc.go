// Package authz defines Kernel's authorization boundary.
//
// The package owns the question "can this subject do this action on this
// resource?". It supports RBAC as relationships, ReBAC as Zanzibar tuples and
// ABAC through request attributes/caveats, while keeping business code unaware
// of SpiceDB, OpenFGA, OPA or Casbin.
//
// Business code should depend on Authorizer only. Provisioning code may also
// use RelationshipWriter and SchemaManager.
package authz
