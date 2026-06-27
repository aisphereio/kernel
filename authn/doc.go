// Package authn defines Kernel's authentication and identity-management boundary.
//
// The package is intentionally provider-neutral. A Casdoor adapter should map
// OAuth code exchange, JWT parsing, users, organizations, applications and
// group trees into these contracts, but business code should only see Principal.
//
// Typical flow:
//
//  1. HTTP/gRPC middleware extracts a bearer token.
//  2. Authenticator verifies it and returns Principal.
//  3. Principal is stored in context.
//  4. Business code uses authz.Authorizer for permissions.
//
// Identity provisioning flow:
//
//  1. Kernel IAM DB creates the platform organization/application/group.
//  2. IdentityAdmin syncs the identity-provider projection, e.g. Casdoor.
//  3. authz.RelationshipWriter projects owner/member relationships to SpiceDB.
package authn
