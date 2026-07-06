// Package accessx provides Kernel's request-time access guard.
//
// accessx is the runtime orchestration layer for authn + authz + audit. In the
// normal Kernel server path, middleware/authn authenticates first and injects an
// authn.Principal into context. accessx then consumes that principal, applies
// authorization policy, resolves SkipPolicy, and records audit. A legacy
// authenticator hook remains on Guard only for direct callers that still pass raw
// credentials in Check.
//
// # accessx.Guard vs authz.Authorizer
//
// Use accessx.Guard when handling a request and you need the whole access chain:
// principal resolution, optional authn fallback, authz decision, SkipPolicy and
// audit.
//
// Use authz.Authorizer when you only need to ask the authorization backend for a
// provider-neutral decision. authz must not grow a second request-time guard.
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
//     endpoints like health checks, login, token exchange, etc. Guard.Require
//     still records audit with an anonymous actor.
//
// # AccessConfig
//
// AccessConfig controls per-operation access policies via YAML configuration.
// It supports three operation lists evaluated in priority order:
//
//  1. PublicOperations — operations that skip both authn and authz but still audit (SkipAll).
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
