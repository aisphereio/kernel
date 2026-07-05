# AuthN Full Flow: Casdoor JWT, Gateway JWKS, Internal Service Token

## Final decision

Aisphere uses Casdoor as the only login/session/token issuer. Kernel does not issue its own JWT, does not store passwords, and does not store user sessions.

The runtime request flow is:

```text
Client
  -> Gateway: Authorization: Bearer <Casdoor access_token JWT>
Gateway
  -> OIDC discovery/JWKS: fetch and cache public signing keys
  -> verify JWT locally: signature, iss, aud, exp, nbf, iat, alg, owner
  -> strip client-supplied X-Aisphere-* and X-Aisphere-Internal-Token
  -> inject trusted X-Aisphere-* Principal headers
  -> inject X-Aisphere-Internal-Token
Backend service
  -> verify X-Aisphere-Internal-Token
  -> trust Gateway Principal headers
  -> execute business logic
```

## JWKS is not secret

JWKS publishes public keys. Public keys can verify tokens but cannot sign or forge tokens. Casdoor keeps the private signing key. Gateway must never discover JWKS from untrusted token input; it must use a configured issuer/discovery URL allowlist.

## Kernel packages

- `authn/oidcx`: provider-neutral OIDC discovery + JWKS JWT verifier.
- `authn`: Principal, trusted headers, `InternalServiceTokenConfig`.
- `middleware/authn`: Bearer and gateway-trusted credential extractors.
- `gatewayx`: route dispatch, JWT verification, trusted header/internal-token injection.

## Gateway behavior

Gateway must call:

```go
gatewayx.NewDispatcher(
  routes,
  hosts,
  invoker,
  gatewayx.WithAuthenticator(oidcVerifier),
  gatewayx.WithInternalServiceToken(authn.InternalServiceTokenConfig{...}),
)
```

Before forwarding, Dispatcher calls:

```go
authn.StripGatewayControlledHeaders(req.Headers, internalTokenHeader)
authn.InjectTrustedHeaders(req.Headers, principal)
authn.InjectInternalServiceToken(req.Headers, internalTokenConfig)
```

## Backend behavior

Backend services use `gateway_trusted` mode by default:

```yaml
security:
  authn:
    mode: gateway_trusted
  internal_call:
    enabled: true
    header: X-Aisphere-Internal-Token
    token: ${GATEWAY_TO_BACKEND_INTERNAL_TOKEN}
```

The authenticator is:

```go
authn.TrustedHeaderAuthenticator{InternalToken: cfg.Security.InternalCall}
```

This means a forged `X-Aisphere-Subject` is not enough. The backend also requires the internal service token, and production deployments should use NetworkPolicy so only Gateway can reach backend pods.

## Redis cache guidance

Redis is optional. Correctness does not require Redis.

- JWKS public keys: cache in process.
- token hash -> Principal: optional optimization.
- raw tokens, private keys, refresh tokens: never cache.

If token-result caching is enabled, key by `sha256(raw_token)` and set TTL to `min(configured_ttl, token_exp-now)`. Never return cached principals after token expiration.

## AuthZ during testing

For the first authn end-to-end test, set authz to `dev_allow_all` or AccessX `SkipAuthz`. After the authentication chain is stable, switch authz back to IAM AuthzService/SpiceDB.
