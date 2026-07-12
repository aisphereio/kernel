# Kernel Access Policy Contract

Kernel follows **contract-first + policy-as-metadata + runtime autowire**.
Business code must not invent request wiring, auth checks, or ad-hoc runtime
limits. API contracts declare intent in proto; code generators create glue; the
runtime autowires providers and middleware in a fixed order.

## Required proto policy

Every externally exposed RPC, especially every RPC with `google.api.http`, must
explicitly declare `aisphere.access.v1.policy`.

This rule does **not** mean every endpoint needs resource authorization. It means
every exposed endpoint must choose one access class deliberately:

| exposure | Meaning | Authn | Authz | Typical endpoints |
|---|---|---:|---:|---|
| `PUBLIC` | Internet-facing anonymous endpoint | no | no | login, register, public callback |
| `AUTHENTICATED` | Valid user/service identity required | yes | no resource check | `/me`, logout, token refresh |
| `AUTHORIZED` | Identity plus resource/action permission | yes | yes | CRUD, run workflow, publish skill |
| `INTERNAL` | Service boundary only | service auth/mTLS | optional | scheduler trigger, worker callback |
| `SYSTEM` | Framework/system endpoint | framework-owned | framework-owned | healthz, readyz, metrics, pprof |

Public and system endpoints are not silent bypasses. They are explicit policies.

## Example: resource endpoint

```proto
rpc UpdateTodo(UpdateTodoRequest) returns (Todo) {
  option (google.api.http) = {
    put: "/v1/todos/update"
    body: "todo"
  };
  option (aisphere.access.v1.policy) = {
    exposure: AUTHORIZED
    authz: {
      action: "update"
      resource: "todo:{todo.id}"
      audience: "todo-service"
      mode: CHECK_ONLY
    }
    audit: {
      enabled: true
      event: "todo.access"
      risk: "medium"
    }
    rate_limit: {
      enabled: true
      key: "principal"
      qps: 50
      burst: 100
      backend: MEMORY
    }
  };
}
```

## Example: login endpoint

```proto
rpc Login(LoginRequest) returns (LoginReply) {
  option (google.api.http) = {
    post: "/v1/auth/login"
    body: "*"
  };
  option (aisphere.access.v1.policy) = {
    exposure: PUBLIC
    audit: {
      enabled: true
      event: "auth.login"
      risk: "medium"
    }
    rate_limit: {
      enabled: true
      key: "ip"
      qps: 5
      burst: 10
      backend: REDIS
    }
  };
}
```

## Health, readiness, metrics

Prefer framework-owned system routes configured by `bootx/serverx` instead of
business proto methods:

```yaml
server:
  system_routes:
    healthz:
      enabled: true
      exposure: SYSTEM
    readyz:
      enabled: true
      exposure: SYSTEM
    metrics:
      enabled: true
      exposure: INTERNAL
```

If a system endpoint is intentionally declared in proto, it must still declare:

```proto
option (aisphere.access.v1.policy) = {
  exposure: SYSTEM
  reason: "framework health check"
};
```

## Rate limit backend policy

Rate-limit policy has two levels:

1. **Contract intent** in proto: key, qps, burst, backend.
2. **Runtime provider** selected by boot/autowire based on deployment mode.

| Deployment shape | Allowed backend | Reason |
|---|---|---|
| local dev / single process | `MEMORY` | cheap, deterministic, no external dependency |
| one Kubernetes replica | `MEMORY` allowed | state is process-local but there is only one process |
| multiple Kubernetes replicas | `REDIS` or `EXTERNAL` required | limit state must be shared across pods |
| gateway-level global limit | `EXTERNAL` | delegated to Envoy/APISIX/cloud gateway |

The current Kernel `ratelimitx` implementation supports both process-local and
distributed backends. Use `backend: MEMORY` for local demos and single-replica
services. Multi-replica production must use `backend: REDIS` or `backend: EXTERNAL`.
Boot-time validation should reject `backend: MEMORY` when configured replicas are
greater than one.

## Enforcement layers

| Layer | Tool/package | Responsibility |
|---|---|---|
| Contract lint | `buf-check-aisphere` | Auto access policy and validate required fields |
| Codegen | `protoc-gen-go-authz` | Convert AUTHORIZED policy into access resolver |
| Runtime | `middleware/autowire` | Fixed ctx/authn/ratelimit/authz/audit chain |
| Runtime | `accessx.SkipPolicyResolver` | Config-driven skip policies (SkipAuthz/SkipAll) checked before resolver |
| Boot | `bootx/serverx` | Select config and reject invalid deployment policies |
| Agent guide | `AGENTS.md` | Keep AI agents inside the Kernel development paradigm |

## Agent rule

Agents must fix the Kernel abstraction when it is insufficient. They must not
bypass Kernel by hand-rolling auth, rate limits, request context, tracing,
errors, or transport glue in business code.
