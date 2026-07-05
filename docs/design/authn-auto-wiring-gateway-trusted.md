# AuthN Auto Wiring: Casdoor JWT at Gateway, Gateway-Trusted Backends

## Final boundary

The platform uses one clear AuthN chain:

```text
Client Casdoor JWT
  -> Gateway verifies JWT with OIDC Discovery/JWKS
  -> Gateway strips spoofable identity/internal headers
  -> Gateway injects X-Aisphere-* Principal headers
  -> Gateway injects X-Aisphere-Internal-Token
  -> Backend service verifies internal token
  -> Backend restores Principal from trusted headers
  -> middleware/access consumes Principal for authz/audit
  -> Business handler reads authn.PrincipalFromContext(ctx)
```

Casdoor remains the only login/session/token issuer. Kernel does not issue JWTs,
store passwords, or store sessions.

## Framework API

Services should not manually construct these pieces:

- `TrustedHeaderAuthenticator`
- `GatewayTrustedExtractor`
- `oidcx.Verifier`
- internal-service-token header checks
- authn/access middleware order

Instead they use the canonical `security` subtree and build a provider-neutral
runtime:

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

`securityx` builds runtime values from config. `serverx/autowire` remains the
only middleware assembly layer and owns the middleware order.

## Modes

### `casdoor_jwt`

Used by Gateway and optionally by high-security services.

```yaml
security:
  authn:
    enabled: true
    mode: casdoor_jwt
    provider: casdoor
    oidc:
      issuer: "https://casdoor.example.com"
      discovery_url: "https://casdoor.example.com/.well-known/openid-configuration"
      jwks_url: "https://casdoor.example.com/.well-known/jwks"
      audience: ["hub-web"]
      allowed_owners: ["aisphere"]
      allowed_algs: ["RS256"]
```

The verifier validates signature, issuer, audience, `exp`, `nbf`, `iat`,
algorithm and optional Casdoor owner.

### `gateway_trusted`

Default backend mode.

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

The backend validates the internal token, requires
`X-Aisphere-Auth-Verified=true`, restores the trusted Principal and puts it into
`context.Context`.

### `hybrid`

Migration/debug mode. It first accepts bearer JWTs, otherwise falls back to
Gateway trusted headers. Do not use it as the long-term production default.

## Gateway behavior

`gatewayx.Dispatcher` owns the edge behavior:

1. Strip all incoming `X-Aisphere-*` trusted headers.
2. Strip incoming `X-Aisphere-Internal-Token`.
3. Verify the external Casdoor/OIDC bearer JWT for protected routes.
4. Inject trusted Principal headers.
5. Inject Gateway-to-backend internal token.
6. Forward to the selected upstream.

Business Gateway code only supplies the configured `Authenticator` and
`InternalServiceTokenConfig` to the dispatcher.

## Access behavior

`accessx` does not own the normal authentication flow. The normal request chain
is:

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

`SkipAuthz` still requires an authenticated Principal and records audit.
`SkipAll` skips authn/authz for public endpoints but still records audit with an
anonymous actor.

## Business usage

Handlers only need:

```go
principal, ok := authn.PrincipalFromContext(ctx)
if !ok || !principal.IsAuthenticated() {
    return nil, errors.Unauthorized("AUTHN_REQUIRED", "missing principal")
}
```

Handlers must not parse Authorization headers, trusted headers, JWKS, or
internal-service-token values directly.
