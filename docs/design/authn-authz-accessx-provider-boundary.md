# Kernel AuthN / AuthZ / AccessX Provider Boundary

## 1. Positioning

Kernel does not own a concrete identity provider or authorization backend. It owns the stable contracts used by Aisphere services:

```text
authn  = who is calling
authz  = can this subject do this action on this resource
accessx = transport-independent authn + authz + audit orchestration
auditx = durable security event recording
gatewayx = route declaration / route registry structures
```

Concrete providers are adapters:

```text
Casdoor adapter -> authn.IdentityAdmin / TokenService / ProfileService
SpiceDB adapter -> authz.AdminProvider / Authorizer / RelationshipWriter
```

Business services must depend on Kernel interfaces, not provider SDKs.

---

## 2. Casdoor boundary

Casdoor is the identity source of truth for users and groups. Management calls must be made by the IAM control plane through a dedicated service application using M2M/client_credentials semantics.

Do not configure normal runtime management calls with `built-in/admin`.

Recommended shape:

```yaml
security:
  authn:
    casdoor:
      application_name: aisphere       # browser/OIDC login app
      admin:
        enabled: true
        application_name: iam-service  # backend M2M management app
```

---

## 3. AccessX short-circuit rules

AccessX supports three runtime modes:

| Mode | Meaning |
|---|---|
| `SkipDefault` | Authenticate and authorize normally. |
| `SkipAuthz` | Authenticate and audit, but skip the authz backend check. Use for self/bootstrap/internal operations. |
| `SkipAll` | Public endpoint; skip authn and authz. Use only for health/login/public callback style endpoints. |

`middleware/access.Server` now treats `SkipAuthz` as an explicit Guard path. This avoids a dangerous failure mode where an operation listed in config could bypass both authn and authz when no generated resolver existed.

---

## 4. Declarative route/access rules

`accessx.AccessRule` is the provider-neutral declaration format that can be generated from protobuf, stored in gateway route registry, or declared by services:

```go
type AccessRule struct {
    ID           string
    Authn        AuthnMode
    Permission   string
    Resource     ResourceResolverSpec
    Subject      SubjectResolverSpec
    ShortCircuit ShortCircuitSpec
    Audit        AuditSpec
}
```

Concrete middleware converts this into `accessx.Check` and calls `Guard.Require`.

---

## 5. Provider neutrality requirements

Allowed imports in business services:

```text
github.com/aisphereio/kernel/authn
github.com/aisphereio/kernel/authz
github.com/aisphereio/kernel/accessx
github.com/aisphereio/kernel/auditx
github.com/aisphereio/kernel/gatewayx
```

Forbidden imports in business services:

```text
github.com/casdoor/casdoor-go-sdk/...
github.com/authzed/authzed-go/...
```

Provider SDKs belong in IAM/provider packages or Kernel adapter packages only.
