# Casdoor JWKS + Envoy Gateway AuthN Boundary

## Decision

Kernel standardizes the following model:

```text
Casdoor signs OAuth/OIDC access tokens as JWTs.
Envoy Gateway verifies those JWTs locally with OIDC Discovery + JWKS.
Envoy Gateway injects trusted Principal headers for internal services.
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

A valid Envoy Gateway verifier must check:

- JWT signature against a configured Casdoor JWKS.
- `alg` is in an allow-list.
- `iss` equals the configured trusted issuer.
- `aud` contains an expected client/application ID.
- `exp`, `nbf`, `iat` are valid within clock skew.
- optional Casdoor `owner` is in `allowed_owners`.

Never derive the JWKS URL from an untrusted token at request time. Discovery/JWKS endpoints must come from config.

## Header trust rule

Envoy Gateway must strip inbound `X-Aisphere-*` identity headers before injection via `ClientTrafficPolicy`. After JWT verification it injects via `claimToHeaders`:

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
x-aisphere-external-issuer
x-aisphere-external-owner
x-aisphere-external-id
```

Services use `authn.TrustedHeaderAuthenticator` only when traffic cannot bypass Envoy Gateway.

## Kernel packages

- `authn/casdoor`: supports static cert fallback and OIDC/JWKS verification.
- `authn`: defines trusted identity header constants and mapping helpers.
- `middleware/authn`: provides `TrustedHeaderExtractor` for internal services.
- `gatewayx`: route manifest generation and route registry structures.