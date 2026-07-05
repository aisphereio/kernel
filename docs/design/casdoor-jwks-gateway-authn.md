# Casdoor JWKS + Gateway AuthN Boundary

## Decision

Kernel standardizes the following model:

```text
Casdoor signs OAuth/OIDC access tokens as JWTs.
Gateway verifies those JWTs locally with OIDC Discovery + JWKS.
Gateway injects trusted Principal headers for internal services.
Internal services default to gateway_trusted mode.
IAM does not sign a second platform JWT.
```

## Why JWKS is public

JWKS contains public keys only. Public keys verify signatures; they do not sign tokens.

```text
Casdoor private key -> sign JWT
JWKS public key     -> verify JWT
```

The secret is the private key stored by Casdoor. Publishing JWKS is the standard OIDC mechanism that lets resource servers validate JWTs and handle key rotation.

## Verification requirements

A valid Gateway verifier must check:

- JWT signature against a configured Casdoor JWKS.
- `alg` is in an allow-list.
- `iss` equals the configured trusted issuer.
- `aud` contains an expected client/application ID.
- `exp`, `nbf`, `iat` are valid within clock skew.
- optional Casdoor `owner` is in `allowed_owners`.

Never derive the JWKS URL from an untrusted token at request time. Discovery/JWKS endpoints must come from config.

## Header trust rule

Gateway must strip inbound `X-Aisphere-*` identity headers before injection. After JWT verification it may inject:

```text
X-Aisphere-Auth-Verified
X-Aisphere-Subject
X-Aisphere-Subject-Type
X-Aisphere-Provider
X-Aisphere-External-ID
X-Aisphere-Owner
X-Aisphere-Username
X-Aisphere-Email
X-Aisphere-Groups
X-Aisphere-Roles
X-Aisphere-Scopes
```

Services may use `authn.TrustedHeaderAuthenticator` only when traffic cannot bypass Gateway.

## Kernel packages

- `authn/casdoor`: supports static cert fallback and OIDC/JWKS verification.
- `authn`: defines trusted identity header constants and mapping helpers.
- `middleware/authn`: provides `TrustedHeaderExtractor` for internal services.
- `gatewayx`: verifies external JWTs when configured with `WithAuthenticator`, strips spoofed trusted headers, and injects verified Principal headers.
