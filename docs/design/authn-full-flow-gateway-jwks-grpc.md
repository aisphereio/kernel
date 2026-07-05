# AuthN Full Flow: Casdoor JWT -> Gateway JWKS -> gRPC Backend

## Decision

Casdoor is the only login/session/token issuer. Kernel does not issue platform JWTs.

The standard request path is:

```text
Browser / CLI
  -> Authorization: Bearer <Casdoor access_token JWT>
Gateway
  -> OIDC discovery + JWKS verification
  -> validate issuer / audience / expiry / owner / algorithm
  -> strip spoofable X-Aisphere-* headers
  -> inject trusted Principal headers / gRPC metadata
  -> forward to backend gRPC service
Backend service
  -> either trust Gateway Principal or re-verify the same JWT with Kernel oidcx
  -> execute business logic
```

For the current full-flow authn test, authorization can be short-circuited with `dev_allow_all` or route-level SkipAuthz. The goal is to verify that authentication is correct from edge to backend.

## JWKS and caching

JWKS contains public keys only. It is safe to expose because public keys can verify a signature but cannot sign a JWT. Casdoor keeps the private signing key.

Kernel provides two caches:

1. In-process JWKS cache in `authn/oidcx`. It caches public keys by `kid` and refreshes from `jwks_uri` after `jwks_cache_ttl_ns`.
2. Optional token verification-result cache via `authn.NewCachedAuthenticator`. It stores a normalized `Principal` keyed by `sha256(token)`, never by raw token. TTL is capped by `min(cache_ttl, token.exp-now)`, so an expired token is never served from cache.

Redis is optional. It is useful for high-QPS Gateways where repeated JWT verification becomes expensive. It is not required for correctness.

Do not cache raw access tokens, private keys, client secrets, or decrypted data. JWTs here are signed, not encrypted.

## Gateway config

```yaml
security:
  authn:
    enabled: true
    provider: casdoor
    cache_ttl_ns: 300000000000
    oidc:
      provider: casdoor
      issuer: https://casdoor.example.com
      discovery_url: https://casdoor.example.com/.well-known/openid-configuration
      jwks_url: https://casdoor.example.com/.well-known/jwks
      audience: [aisphere-web]
      allowed_owners: [aisphere]
      allowed_algs: [RS256]
      jwks_cache_ttl_ns: 600000000000
      clock_skew_ns: 60000000000
```

## Backend mode

Backend services can choose one of two modes:

```yaml
security:
  authn:
    mode: gateway_trusted
```

Trust Gateway-injected principal. This requires network policy / mTLS so clients cannot bypass Gateway.

```yaml
security:
  authn:
    mode: casdoor_jwt
```

Re-verify the same Casdoor JWT locally with Kernel `authn/oidcx`. This is useful for high-sensitivity services or during the full-flow authn test.

## Validation checklist

Gateway must validate:

- JWT signature using JWKS public key selected by `kid`
- `alg` is in allowlist
- `iss` equals configured issuer
- `aud` contains configured audience
- `exp` is not expired
- `nbf` and `iat` are valid within clock skew
- Casdoor `owner` is in `allowed_owners`, when configured

Gateway must remove inbound `X-Aisphere-*` headers before injecting trusted identity headers.
