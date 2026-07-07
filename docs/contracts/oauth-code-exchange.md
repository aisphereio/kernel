# OAuth Authorization Code Exchange Boundary

## 1. Decision

Kernel does not provide a default idempotency/cache wrapper for OAuth authorization-code exchange.

Authorization codes are owned by the authorization server, for example Casdoor. They are short-lived, one-time grants. Kernel and IAM must not introduce a second default code-state authority.

## 2. Correct responsibility split

```text
Frontend
  - starts the login flow
  - receives code/state on the callback URL
  - calls IAM exchange once
  - clears code/state from the URL
  - restarts login if the code is stale, expired, or already used

IAM
  - acts as OAuth client / adapter
  - forwards code exchange to the configured identity provider
  - stays stateless for authorization-code lifetime and single-use semantics

Identity provider / authorization server
  - validates code, client_id, redirect_uri, and optional PKCE verifier
  - enforces expiration
  - enforces single use
```

## 3. What not to do by default

Do not add a default distributed lock, local cache, or successful exchange replay in Kernel/IAM for OAuth authorization codes.

That kind of state makes IAM look like a second authorization-code authority and creates unnecessary implementation choices for multi-replica deployments.

## 4. Handling duplicate callbacks

Duplicate callback attempts should be handled at the edge of the browser flow:

```text
- guard the callback component so it exchanges once per mounted flow
- remove code/state from the browser URL before exchange
- disable automatic retries for the exchange API
- when the provider reports code used/expired/invalid, restart login instead of retrying the same code
```

A repeated authorization-code exchange failure is not a backend consistency issue. It means the browser or caller reused a one-time grant.

## 5. Kernel contract

Kernel `authn.TokenService.ExchangeCode` remains a direct provider-neutral contract:

```go
ExchangeCode(ctx context.Context, req AuthCodeExchangeRequest) (TokenSet, Principal, error)
```

Provider adapters should translate provider errors into Kernel error semantics, but they should not cache or replay successful exchanges by default.
