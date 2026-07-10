# AuthN Casdoor-first Boundary: Users, Groups, Sessions

## Decision

Aisphere uses Casdoor as the only identity-system source of truth.

Kernel and IAM must not implement a second local account system. In particular:

- local username/password accounts are Casdoor local users, not Kernel users;
- password hashes, MFA settings, refresh tokens, logout and browser sessions stay in Casdoor;
- Kernel does not create, validate, persist or revoke sessions;
- Kernel does not become an access-token or refresh-token management service;
- IAM user/group/session APIs are controlled proxies to Casdoor APIs, guarded by Aisphere authorization.

## Runtime authentication

Envoy Gateway verifies Casdoor JWTs with Casdoor OIDC Discovery/JWKS. After a token is verified, Envoy Gateway injects a trusted `authn.Principal` into HTTP headers via `claimToHeaders`. Internal services normally run in `gateway_trusted` mode and do not perform login/session handling.

```text
Client -> Envoy Gateway OIDC/JWT -> Casdoor JWT verify -> Principal
Envoy Gateway -> Backend service (gateway_trusted)
```

## Identity management

IAM exposes Aisphere APIs such as:

```text
/v1/identity/users
/v1/identity/groups
/v1/identity/groups/{group}/members
```

The implementation is:

```text
IAM API
  -> verify Gateway Principal
  -> Check SpiceDB permission
  -> call Kernel authn.UserAdmin / GroupAdmin
  -> Casdoor SDK/API
```

Casdoor remains the source of truth for users and groups. IAM may keep resource grants, audits and SpiceDB relationship projections, but it should not store identity data as authoritative account state.

## Session display

Casdoor owns sessions. Kernel only exposes a read model:

```go
type SessionDirectory interface {
    GetSession(ctx context.Context, orgID, sessionID string) (Session, error)
    ListSessions(ctx context.Context, filter SessionFilter) ([]Session, error)
    SummarizeSessions(ctx context.Context, filter SessionFilter) (SessionSummary, error)
}
```

This supports UI/ops scenarios:

- show online users;
- count active sessions;
- list sessions by application/user/organization;
- show "currently online: N users".

It intentionally does not expose:

- `CreateSession`;
- `ValidateSession`;
- `TouchSession`;
- local opaque session token creation;
- refresh-token rotation;
- password hash verification.

Casdoor's public API exposes session management endpoints such as `GET /api/get-sessions` and the Go SDK exposes `GetSessions()` / `GetSession()`. Kernel's Casdoor adapter maps those objects into provider-neutral `authn.Session` read models.

## Token management

For now, Aisphere does not build a separate token management subsystem.

- Casdoor issues access tokens and refresh tokens.
- Gateway verifies Casdoor JWTs locally.
- High-risk online token active checks are a future route-level option, not part of this interface pass.
- IAM does not persist access tokens or refresh tokens.

## Removed scope

The previous experimental `authn/local` implementation has been removed from the production tree. It created local users, stored bcrypt password hashes and managed in-memory sessions, which conflicts with the Casdoor-first boundary.

For tests, use small in-test fakes/mocks instead of a production-looking local identity provider.
