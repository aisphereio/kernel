// Package securityx provides a small shared configuration shape for Kernel
// security components.
//
// Recommended responsibility split:
//   - authn/casdoor verifies users and provisions the identity-side projection
//     of organizations, applications and groups.
//   - authz/spicedb owns ReBAC relationships and permission checks.
//   - auditx records security-relevant decisions.
//
// Keep business facts in Kernel IAM DB. Sync Casdoor and SpiceDB as projections.
package securityx
