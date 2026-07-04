// Package accessx provides a small authn + authz + audit facade for handlers.
//
// It is intentionally a thin orchestration layer. Business code may depend on
// Guard for endpoint protection, while lower-level services should still depend
// directly on authn.Authenticator, authz.Authorizer, and auditx.Recorder.
//
// # SkipPolicy
//
// SkipPolicy controls whether the Guard performs authentication and/or
// authorization checks. It replaces the deprecated AllowAll + AuthzMode
// combination with a single, clearer policy:
//
//   - SkipDefault — performs the full authentication + authorization check.
//   - SkipAuthz — skips the SpiceDB authorization check but still requires
//     authentication and records audit. Use for operations like GetMe or
//     CreateOrganization where the target resource does not yet exist in
//     the authorization graph.
//   - SkipAll — skips both authentication AND authorization. Use for public
//     endpoints like health checks, login, token exchange, etc.
//
// # AccessConfig
//
// AccessConfig controls per-operation access policies via YAML configuration.
// It supports three operation lists evaluated in priority order:
//
//  1. PublicOperations — operations that skip both authn and authz (SkipAll).
//  2. SkipOperations — operations that skip authz but keep authn (SkipAuthz).
//  3. AllowAllOperations (deprecated) — legacy, use SkipOperations instead.
//
// Example YAML:
//
//	security:
//	  access:
//	    skip_operations:
//	      - GetMe
//	      - CreateOrganization
//	    public_operations:
//	      - Login
//	      - Exchange
//
// # SkipPolicyResolver
//
// NewSkipPolicyResolver creates a SkipPolicyResolver from an AccessConfig.
// It supports matching against short method names, full gRPC method names,
// HTTP URL paths, and wildcard/prefix patterns (e.g. "/v1/authn/*").
// The resolver is used by middleware/access to short-circuit requests before
// calling the main Resolver or Guard.Require.
//
// # Backward Compatibility
//
// The deprecated AuthzMode (AuthzModeDefault / AuthzModeNone) and AllowAll
// fields on Check are still supported. When SkipPolicy is set, it takes
// priority over the deprecated fields. New code should use SkipPolicy.
package accessx
