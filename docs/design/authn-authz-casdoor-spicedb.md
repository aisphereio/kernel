# Authn/Authz/Audit Boundary: Casdoor + SpiceDB

Kernel treats authentication, authorization and auditing as separate concerns.

- `authn`: who is the caller? Casdoor/OIDC maps tokens into `authn.Principal`.
- `authz`: can the caller perform this permission on this resource? SpiceDB owns ReBAC relationships.
- `auditx`: what happened? The decision and context are recorded for traceability.

## Casdoor responsibility

Casdoor is the identity provider and identity-side admin projection:

- users
- organizations
- applications/OAuth clients
- organization groups
- OAuth/OIDC code exchange and JWT verification

Kernel IAM DB remains the business source of truth for platform organizations,
projects and resource metadata. Casdoor is synchronized from IAM when identity
provider state is needed.

## Casdoor applications and credentials

Use two credential sets:

1. Login/OIDC application
   - Used by user-facing services for OAuth code exchange and token verification.
   - Example: `aisphere-hub` under organization `aisphere`.
   - Config path: `security.authn.casdoor.client_id`, `client_secret`, `certificate`.

2. Management application
   - Used by Kernel IAM provisioning code to create or update users,
     organizations, applications and groups.
   - Example: `kernel-iam-admin` under `built-in` or an operations organization.
   - Config path: `security.authn.casdoor.admin.*`.

For local development the adapter can fall back to the login application if the
admin block is omitted. Production should use a dedicated management application
with the smallest Casdoor permissions needed for provisioning.

Do not create a new Casdoor organization just to let Kernel connect to Casdoor.
Organizations represent identity tenants or business organizations. Applications
represent protected services and OAuth clients.

## Groups, not departments

Casdoor's organization tree is modeled as groups. Groups belong to an
organization and can have a parent group. Casdoor group types are `Physical` and
`Virtual`: a user can belong to only one Physical group but multiple Virtual
groups.

Kernel therefore exposes `authn.GroupAdmin`, not `DepartmentAdmin`.

## SpiceDB responsibility

SpiceDB stores the authorization graph projection:

```zed
group:eng#member@user:u1
project:p1#editor@group:eng#member
resource:r1#viewer@user:u2
```

Business services should not call the Casdoor SDK or SpiceDB SDK directly. They
should depend on:

- `authn.Principal`
- `authz.Authorizer`
- `auditx.Recorder`
- optionally `accessx.Guard` at transport boundaries

## External YAML

Example config lives in `configs/security.example.yaml`.

Recommended production shape:

```yaml
security:
  authn:
    provider: casdoor
    casdoor:
      endpoint: "https://door.example.com"
      organization_name: "aisphere"
      application_name: "aisphere-hub"
      client_id: "${CASDOOR_CLIENT_ID}"
      client_secret: "${CASDOOR_CLIENT_SECRET}"
      certificate: "${CASDOOR_CERTIFICATE}"
      admin:
        enabled: true
        organization_name: "built-in"
        application_name: "kernel-iam-admin"
        client_id: "${CASDOOR_ADMIN_CLIENT_ID}"
        client_secret: "${CASDOOR_ADMIN_CLIENT_SECRET}"
        certificate: "${CASDOOR_ADMIN_CERTIFICATE}"
  authz:
    provider: spicedb
    spicedb:
      endpoint: "spicedb:50051"
      token: "${SPICEDB_TOKEN}"
      insecure: false
```
