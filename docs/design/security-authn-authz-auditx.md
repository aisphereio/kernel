# security: authn + authz + auditx boundary

## Position

Kernel security is split into three packages:

```text
authn  -> who is the caller and how identity-provider objects are provisioned
authz  -> can this subject perform this permission on this resource
auditx -> durable evidence of authn/authz/business security events
```

Casdoor should implement the `authn.IdentityAdmin` surface: OAuth code exchange,
token verification, user lookup, organization sync, application sync and
organization-tree/group sync. SpiceDB should implement the `authz.Service`
surface: check, lookup resources, lookup subjects, relationship writes and
schema management.

## Fact source rule

```text
Casdoor         identity source: login accounts, OIDC/OAuth, identity UI
Kernel IAM DB   platform source: org/project/resource metadata and outbox
SpiceDB         authorization graph projection: relationships and checks
auditx store    append-only security evidence
```

Do not let business handlers call Casdoor SDK, SpiceDB SDK or audit database
APIs directly. They should depend on `authn.Principal`, `authz.Authorizer` and
`auditx.Recorder`.

## Organization creation flow

```text
POST /iam/orgs
  -> Kernel IAM DB inserts organization + owner membership + outbox event
  -> authn.IdentityAdmin.CreateOrganization syncs Casdoor projection
  -> authz.RelationshipWriter writes organization:org#owner@user:u
  -> auditx.Recorder records iam.organization.create
```

Early development can write synchronously. Production should use transactional
outbox + idempotent relationship writes to avoid dual-write drift.

## Minimal business usage

```go
principal, _ := authn.MustPrincipal(ctx)
decision, err := authorizer.Check(ctx, authz.CheckRequest{
    Subject:    authz.SubjectRef{Type: authz.SubjectTypeUser, ID: principal.SubjectID},
    Resource:   authz.ObjectRef{Type: "agent", ID: agentID},
    Permission: "edit",
    OrgID:      principal.OrgID,
})
if err != nil { return err }
if !decision.Allowed { return authz.ErrPermissionDenied(decision.Reason) }
```
