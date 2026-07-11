# Security Architecture

Aisphere Kernel implements a **three-tier security model**: edge authentication via Envoy Gateway + Casdoor OIDC, backend principal reconstruction from trusted headers, and resource-level authorization via SpiceDB.

## Security Tiers

| Tier | Access Policy | Behavior |
|---|---|---|
| `AUTH_NONE` / PUBLIC | `aisphere.access.v1.policy.auth_mode = AUTH_NONE` | No principal required, no authn check |
| `AUTHENTICATED` / `AUTHN_ONLY` | `aisphere.access.v1.policy.auth_mode = AUTHENTICATED` | Principal required; no authz check |
| `AUTHORIZED` / `AUTHZ` | `aisphere.access.v1.policy.auth_mode = AUTHORIZED` | Principal required + accessx authz check |
| `INTERNAL` | `aisphere.access.v1.policy.auth_mode = INTERNAL` | Service-to-service only; internal token required |

## Authentication Flow

```
Browser/CLI
  → Bearer JWT
  → Envoy Gateway
    → OIDC Discovery + JWKS verification
    → Strip spoofable headers (x-aisphere-*, x-internal-jwt)
    → Inject trusted headers (x-aisphere-external-*)
    → Inject internal service token (x-aisphere-internal-token)
  → Backend Service
    → Verify internal service token
    → Restore Principal from trusted headers (authn/trusted_headers.go)
    → ContextWithPrincipal → contextx.WithAuthnPrincipal
    → middleware/access → accessx guard
    → auditx → business handler
```

### Key Components

| Component | Role | Source |
|---|---|---|
| `authn.TrustedHeaderAuthenticator` | Reconstructs Principal from Envoy-injected headers | `/authn/trusted_authenticator.go` |
| `authn.trusted_headers.go` | Parses `x-aisphere-external-*` headers into Principal | `/authn/trusted_headers.go` |
| `authn.InternalServiceTokenAuthenticator` | Verifies internal service JWT | `/authn/internal_service_token.go` |
| `authn/oidcx` | OIDC provider client with JWKS caching | `/authn/oidcx/` |
| `authn/casdoor` | Casdoor-specific authn provider | `/authn/casdoor/` |
| `contextx.WithAuthnPrincipal` | Lossless mirror of authn.Principal into context | `/contextx/authn.go` |

### Header Specification

**Envoy Gateway injects** (from Casdoor JWT claims):
- `x-aisphere-external-sub` — Subject ID
- `x-aisphere-external-iss` — Issuer
- `x-aisphere-external-email` — Email
- `x-aisphere-external-name` — Display name
- `x-aisphere-external-preferred-username` — Username
- `x-aisphere-external-owner` — Casdoor owner
- `x-aisphere-external-id` — Casdoor user ID

**Kernel internal projection** (from backend processing):
- `x-aisphere-principal` — Serialized Principal
- `x-aisphere-user-id` — Resolved user ID
- `x-aisphere-org-id` — Resolved org ID

**All `x-aisphere-*` and `x-internal-jwt` headers must be stripped from client input** before Gateway injects trusted values.

## Authorization (authz)

The `authz` package defines provider-neutral authorization contracts:

| Interface | Purpose |
|---|---|
| `Authorizer` | Check permission (`Check(ctx, *Request) (*Response, error)`) |
| `BatchAuthorizer` | Batch permission checks |
| `RelationshipWriter` | Write ReBAC relationships |
| `RelationshipReader` | Query ReBAC relationships |
| `RelationshipStore` | Combined read/write |
| `ResourceLookup` | Look up resources by type/ID |
| `SubjectLookup` | Look up subjects by type/ID |

Default provider: **SpiceDB** (ReBAC/ABAC) via `/authz/spicedb/`.

## Access Orchestration (accessx)

`accessx` is the **request-time access guard** that orchestrates authn + authz + audit:

```go
// accessx.Guard orchestrates the full access check
type Guard struct {
    authn   authn.Authenticator
    authz   authz.Authorizer
    audit   auditx.Recorder
    skip    SkipPolicyResolver
}
```

| Concept | Purpose |
|---|---|
| `SkipPolicy` | Controls which checks to skip: `SkipDefault` (full), `SkipAuthz` (authn+audit only), `SkipAll` (public) |
| `AuthnMode` | Authentication mode (gateway_trusted, direct, etc.) |
| `ShortCircuitMode` | Whether to stop on first failure |
| `AccessConfig` | Configuration for access guard |

## Audit (auditx)

`auditx` provides **append-only security record** storage:

| Interface | Purpose |
|---|---|
| `Recorder` | Record audit events |
| `Queryer` | Query audit records |
| `Store` | Combined recorder + queryer |
| `Record` | Single audit event |
| `QueryFilter` | Filter for audit queries |

Audit is separate from `logx` — it records security-relevant events durably.

## IAM Control Plane (iamx)

`iamx` provides a provider-neutral view of users, organizations, groups, and memberships:

| Interface | Purpose |
|---|---|
| `Directory` | User/org/group/membership queries |
| `Service` | Higher-level IAM operations |

**Important boundary**: `iamx` is NOT a primary store. It's a view interface backed by Casdoor (`iamx/casdoor`), memory (`iamx/memory`), or database (`iamx/db`). The identity source of truth is always Casdoor.

## Resource Model (resourcex)

`resourcex` defines the **control-plane resource model**:

| Type | Purpose |
|---|---|
| `Resource` | A resource instance |
| `ResourceType` | Resource type definition |
| `Grant` | Control-plane grant (for audit/approval/display) |
| `ResourceRef` | Resource reference (type + ID) |

**Key boundary**: `resourcex.Grant` is the control-plane fact; `authz.Relationship` is the ReBAC query projection. Writes go to `resourcex` first, then project to `authz`.

## Configuration

Security configuration is defined in `securityx`:

```yaml
# configs/security.example.yaml
authn:
  mode: gateway_trusted
  providers:
    casdoor:
      endpoint: https://casdoor.example.com
      client_id: ...
      client_secret: ...
      certificate: ...
  internal_token:
    secret: ...
    issuer: kernel
```

## Related Documentation

- [Architecture Overview](overview.md)
- [Gateway Architecture](gateway.md)
- [docs/security/envoy-casdoor-oidc-gateway.md](../../docs/security/envoy-casdoor-oidc-gateway.md)
- [docs/contracts/authn-gateway-generation.md](../../docs/contracts/authn-gateway-generation.md)
- [docs/design/authn-full-flow-gateway-jwks-grpc.md](../../docs/design/authn-full-flow-gateway-jwks-grpc.md)
- [docs/design/authn-gateway-internal-service-token.md](../../docs/design/authn-gateway-internal-service-token.md)
- [docs/design/authn-auto-wiring-gateway-trusted.md](../../docs/design/authn-auto-wiring-gateway-trusted.md)
- [docs/design/security-authn-authz-auditx.md](../../docs/design/security-authn-authz-auditx.md)
- [docs/design/authn-casdoor-first-directory-session.md](../../docs/design/authn-casdoor-first-directory-session.md)