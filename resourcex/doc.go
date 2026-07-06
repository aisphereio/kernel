// Package resourcex defines Kernel's provider-neutral resource control-plane
// contracts.
//
// The package is intentionally business-neutral. It does not know about
// Aisphere-specific resources such as skills, repositories, agents or
// sandboxes. Applications and platform services implement these interfaces in
// their own control-plane service, while shared middleware and generated code
// can depend on the stable contracts here.
//
// Boundary with authz:
//
//   - resourcex stores durable control-plane facts: resource catalog entries,
//     role templates and grants. These records are used for audit, UI,
//     approval, revocation and reconciliation.
//   - authz stores or queries the authorization-engine projection, e.g.
//     Zanzibar/ReBAC relationships in SpiceDB/OpenFGA.
//
// The preferred write path is: update resourcex control-plane facts in the
// domain transaction, emit an outbox event, then project to authz relationships.
package resourcex
