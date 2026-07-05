// Package authn defines Kernel's authentication and identity-management boundary.
//
// The package is intentionally provider-neutral while the platform defaults to a
// Casdoor-first deployment. Casdoor/OIDC/JWKS and trusted gateway headers adapt
// into these contracts. Business code should depend on Principal, Authenticator
// and the small capability interfaces here; it should not import concrete
// provider SDKs.
//
// Request authentication flow:
//
//  1. HTTP/gRPC middleware extracts a credential or trusted gateway metadata.
//  2. Authenticator verifies it and returns Principal.
//  3. Principal is stored in context.
//  4. Business code uses authz.Authorizer for permissions.
//
// Provider/token flow:
//
//  1. LoginService builds a hosted Casdoor/OIDC login URL.
//  2. TokenService is a thin wrapper around the provider OAuth/OIDC flow when a
//     service must perform callback exchange or token verification. Kernel does
//     not become a token issuer or token store.
//  3. ProfileService can enrich Principal with user/group/application data.
//
// Identity provisioning flow:
//
//  1. Kernel IAM DB creates the platform organization/application/group.
//  2. UserAdmin/GroupAdmin/IdentityAdmin sync identity-provider projections,
//     e.g. Casdoor users and groups.
//  3. authz.RelationshipWriter projects owner/member relationships to SpiceDB.
//
// Casdoor-first identity flow:
//
//  1. User/password, MFA, refresh tokens, logout and browser sessions remain in
//     Casdoor, including Casdoor's local username/password provider.
//  2. UserAdmin/GroupAdmin call Casdoor APIs through the adapter; Kernel does not
//     store users as source of truth or manage password hashes.
//  3. SessionDirectory reads Casdoor sessions for display, such as online-user
//     dashboards. It does not create or validate sessions.
package authn
