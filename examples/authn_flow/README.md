# AuthN Full Flow Demo

This demo exercises the target path:

```text
Client -> Gateway -> gRPC backend
```

- The client sends a Casdoor/OIDC JWT in `Authorization: Bearer`.
- Gateway loads OIDC discovery + JWKS, verifies the JWT locally, and validates `iss`, `aud`, `exp`, `nbf`, `iat`, `alg`, and `owner`.
- Gateway strips client-supplied `X-Aisphere-*` identity headers and injects trusted principal headers.
- The backend can also reuse the same Kernel `authn/oidcx` verifier to validate the JWT again.
- AuthZ is intentionally out of scope in this demo and can be short-circuited while testing authn.

Run:

```bash
go run ./examples/authn_flow/demo.go
```
