# AuthN Auto Wiring: Envoy Gateway OIDC, Gateway-Trusted Backends

## Final boundary

The platform uses one clear AuthN chain:

```text
Client Casdoor JWT
  -> Envoy Gateway verifies JWT with OIDC Discovery/JWKS
  -> Envoy Gateway strips spoofable identity/internal headers
  -> Envoy Gateway injects X-Aisphere-* Principal headers
  -> Envoy Gateway injects X-Aisphere-Internal-Token
  -> Backend service verifies internal token
  -> Backend restores Principal from trusted headers
  -> middleware/access consumes Principal for authz/audit
  -> Business handler reads authn.PrincipalFromContext(ctx)
```

Casdoor remains the only login/session/token issuer. Kernel does not issue JWTs, store passwords, or store sessions.

## Framework API

Services should not manually construct these pieces:

- `TrustedHeaderAuthenticator`
- `GatewayTrustedExtractor`
- `oidcx.Verifier`
- internal-service-token header checks
- authn/access middleware order

Instead they use the canonical `security` subtree and build a provider-neutral runtime:

```go
securityRuntime, err := securityx.NewRuntime(ctx, securityx.Config{
    Authn: securityx.AuthnBoundaryConfig{
        Enabled:        true,
        Mode:           securityx.AuthnModeGatewayTrusted,
        AllowAnonymous: true,
    },
    InternalCall: authn.InternalServiceTokenConfig{
        Enabled: true,
        HeaderName: "X-Aisphere-Internal-Token",
        Token: os.Getenv("GATEWAY_TO_APP_INTERNAL_TOKEN"),
    },
    Access: accessx.AccessConfig{
        PublicOperations: []string{"/healthz", "/readyz"},
        SkipOperations:   []string{"GetMe", "CreateOrganization"},
    },
}, cache)
```

Then pass it to the single server assembly entrypoint:

```go
middlewares := serverx.ServerMiddlewareFromProviders(ctx, serverx.RuntimeProviders{
    Security:            securityRuntime,
    AccessGuard:         &resources.Access,
    RequestInfoResolver: generatedRequestInfoResolver,
    AccessResolver:      generatedAccessResolver,
})
```

`securityx` builds runtime values from config. `serverx/autowire` remains the only middleware assembly layer and owns the middleware order.

## Mode: gateway_trusted (default)

Default backend mode for all services behind Envoy Gateway.

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
  internal_call:
    enabled: true
    header: X-Aisphere-Internal-Token
    token: "${GATEWAY_TO_APP_INTERNAL_TOKEN}"
  access:
    public_operations:
      - /healthz
      - /readyz
    skip_operations:
      - GetMe
      - CreateOrganization
```

The backend validates the internal token, requires `X-Aisphere-Auth-Verified=true`, restores the trusted Principal and puts it into `context.Context`.

## Envoy Gateway behavior

Envoy Gateway owns the edge behavior:

1. Strip all incoming `X-Aisphere-*` trusted headers via `ClientTrafficPolicy`.
2. Verify the external Casdoor/OIDC bearer JWT for protected routes via `SecurityPolicy`.
3. Inject trusted Principal headers via `claimToHeaders`.
4. Inject Gateway-to-backend internal token.
5. Forward to the selected upstream.

## Access behavior

`accessx` does not own the normal authentication flow. The normal request chain is:

```text
middleware/authn
  -> inject authn.Principal into context
middleware/access
  -> read Principal from context
  -> apply security.access policy
  -> authz.Check
  -> audit.Record
business handler
```

`SkipAuthz` still requires an authenticated Principal and records audit. `SkipAll` skips authn/authz for public endpoints but still records audit with an anonymous actor.

## Business usage

Handlers only need:

```go
principal, ok := authn.PrincipalFromContext(ctx)
if !ok || !principal.IsAuthenticated() {
    return nil, errors.Unauthorized("AUTHN_REQUIRED", "missing principal")
}
```

Handlers must not parse Authorization headers, trusted headers, JWKS, or internal-service-token values directly.