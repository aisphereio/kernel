# Kubernetes apiserver lessons for Kernel

Kernel borrows the **design pattern**, not the implementation, from Kubernetes apiserver:

```text
API contract
  -> normalized request metadata
  -> generic server/client chain
  -> authn / authz / flow control / audit
  -> admission mutation / validation
  -> business storage/service
  -> negotiated status/error
```

## Kernel mapping

| Kubernetes apiserver idea | Kernel equivalent |
|---|---|
| RequestInfo | `requestx.Info` |
| request filters | `middleware/autowire` |
| authentication chain | `middleware/authn` + providers |
| authorization attributes | `accessx.Check` + generated access resolver |
| priority and fairness / flow control | `ratelimitx` + client/server policies |
| audit policy/backend | `auditx` + access middleware |
| admission plugins | `admissionx` |
| generic server system routes | future `serverx/bootx` |
| status/error negotiation | `errorx` HTTP/gRPC adapters |

## Kernel rule

Every middleware that needs request metadata must read `requestx.Info` first. If the metadata is missing, fix the generated resolver or `middleware/requestinfo`; do not re-parse HTTP/gRPC operation strings in business code.

## Why this matters for AI agents

Multiple agents can build gateway, IAM, Hub, Runtime, and Sandbox independently only if the framework contract is explicit and checkable. The apiserver pattern gives us stable seams:

1. contract declarations;
2. lint and generation;
3. normalized request metadata;
4. ordered middleware chain;
5. admission hooks;
6. storage/service implementation.

Agents must extend Kernel at the right seam instead of inventing local plumbing.
