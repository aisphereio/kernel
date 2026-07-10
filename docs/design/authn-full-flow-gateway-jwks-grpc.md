# AuthN Full Flow: Casdoor JWT -> Envoy Gateway JWKS -> Backend

## Decision

Casdoor is the only login/session/token issuer. Kernel does not issue platform JWTs.

The standard request path is:

```text
Browser / CLI
  -> Authorization: Bearer <Casdoor access_token JWT>
Envoy Gateway
  -> OIDC discovery + JWKS verification
  -> validate issuer / audience / expiry / owner / algorithm
  -> strip spoofable X-Aisphere-* headers
  -> inject trusted Principal headers
  -> forward to backend service
Backend service
  -> trust Envoy Gateway Principal (gateway_trusted mode)
  -> execute business logic
```

## JWKS and caching

JWKS contains public keys only. It is safe to expose because public keys can verify a signature but cannot sign a JWT. Casdoor keeps the private signing key.

Kernel provides two caches:

1. In-process JWKS cache in `authn/oidcx`. It caches public keys by `kid` and refreshes from `jwks_uri` after `jwks_cache_ttl_ns`.
2. Optional token verification-result cache via `authn.NewCachedAuthenticator`. It stores a normalized `Principal` keyed by `sha256(token)`, never by raw token. TTL is capped by `min(cache_ttl, token.exp-now)`, so an expired token is never served from cache.

Redis is optional. It is useful for high-QPS scenarios where repeated JWT verification becomes expensive. It is not required for correctness.

Do not cache raw access tokens, private keys, client secrets, or decrypted data. JWTs here are signed, not encrypted.

## Envoy Gateway SecurityPolicy config

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: myapp-oidc
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: myapp-route
  oidc:
    provider:
      issuer: https://casdoor.weagent.cc
    clientID: myapp-web
    clientSecret:
      name: casdoor-myapp-oidc
    redirectURL: https://myapp.weagent.cc/oauth2/callback
    logoutPath: /logout
    scopes: [openid, profile, email]
    refreshToken: true
    forwardAccessToken: true
    passThroughAuthHeader: true
  jwt:
    providers:
      - name: casdoor
        issuer: "https://casdoor.weagent.cc"
        audiences: ["myapp-web"]
        remoteJWKS:
          uri: "https://casdoor.weagent.cc/.well-known/jwks"
        claimToHeaders:
          - claim: sub
            header: x-aisphere-external-sub
          - claim: email
            header: x-aisphere-external-email
          - claim: name
            header: x-aisphere-external-name
          - claim: preferred_username
            header: x-aisphere-external-username
```

## Backend mode

Backend services use `gateway_trusted` mode:

```yaml
security:
  authn:
    mode: gateway_trusted
```

Trust Envoy Gateway-injected principal. This requires NetworkPolicy so clients cannot bypass Envoy Gateway.

## Validation checklist

Envoy Gateway must validate:

- JWT signature using JWKS public key selected by `kid`
- `alg` is in allowlist
- `iss` equals configured issuer
- `aud` contains configured audience
- `exp` is not expired
- `nbf` and `iat` are valid within clock skew

Envoy Gateway must remove inbound `X-Aisphere-*` headers before injecting trusted identity headers via `ClientTrafficPolicy`.
