# AuthN Full Flow: Casdoor JWT, Envoy Gateway JWKS, Internal Service Token

## Final decision

Aisphere uses Casdoor as the only login/session/token issuer. Kernel does not issue its own JWT, does not store passwords, and does not store user sessions.

The runtime request flow is:

```text
Client
  -> Envoy Gateway: Authorization: Bearer <Casdoor access_token JWT>
Envoy Gateway
  -> OIDC discovery/JWKS: fetch and cache public signing keys
  -> verify JWT locally: signature, iss, aud, exp, nbf, iat, alg, owner
  -> strip client-supplied X-Aisphere-* headers via ClientTrafficPolicy
  -> inject trusted X-Aisphere-* Principal headers via claimToHeaders
  -> inject X-Aisphere-Internal-Token
Backend service
  -> verify X-Aisphere-Internal-Token
  -> trust Envoy Gateway Principal headers
  -> execute business logic
```

## JWKS is not secret

JWKS publishes public keys. Public keys can verify tokens but cannot sign or forge tokens. Casdoor keeps the private signing key. Envoy Gateway must never discover JWKS from untrusted token input; it must use a configured issuer/discovery URL allowlist.

## Kernel packages

- `authn/oidcx`: provider-neutral OIDC discovery + JWKS JWT verifier.
- `authn`: Principal, trusted headers, `InternalServiceTokenConfig`.
- `middleware/authn`: Bearer and gateway-trusted credential extractors.

## Envoy Gateway behavior

Envoy Gateway uses `SecurityPolicy` for OIDC/JWT and `ClientTrafficPolicy` for header sanitize:

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

This means a forged `X-Aisphere-Subject` is not enough. The backend also requires the internal service token, and production deployments should use NetworkPolicy so only Envoy Gateway can reach backend pods.

## Redis cache guidance

Redis is optional. Correctness does not require Redis.

- JWKS public keys: cache in process.
- token hash -> Principal: optional optimization.
- raw tokens, private keys, refresh tokens: never cache.

If token-result caching is enabled, key by `sha256(raw_token)` and set TTL to `min(configured_ttl, token_exp-now)`. Never return cached principals after token expiration.

## AuthZ during testing

For the first authn end-to-end test, set authz to `dev_allow_all` or AccessX `SkipAuthz`. After the authentication chain is stable, switch authz back to IAM AuthzService/SpiceDB.