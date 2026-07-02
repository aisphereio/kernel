# Gateway Route Publication Policy

This document defines how proto services decide whether an RPC is published to a Gateway route registry.

## Problem

`google.api.http` only says that an RPC has an HTTP binding. It does **not** necessarily mean the route should be published through a public gateway.

Examples that may have HTTP handlers but should not be public Gateway APIs:

- `/internal/dtm/*` branch and compensation endpoints
- `/healthz`, `/readyz`, `/metrics`, `/debug/*`
- IAM token verification / permission check APIs used by internal services
- repair, migration, bootstrap, and other operator-only endpoints

## Rule

A route is published into a generated `GatewayManifest` only when both are true:

1. The RPC has `google.api.http`.
2. The RPC explicitly declares `aisphere.access.v1.policy`.

If the policy is absent, the service may still register its own HTTP route, but `protoc-gen-go-gateway` will not publish it to Gateway.

## Proto contract

```proto
option (aisphere.access.v1.policy) = {
  exposure: AUTHORIZED
  authz: {
    action: "update"
    resource: "aihub:skill:{name}"
    audience: "aihub-service"
    mode: CHECK_ONLY
  }
  gateway: {
    profiles: "public"
    profiles: "internal"
    tags: "skill"
  }
};
```

### Disable Gateway publishing entirely

Use `gateway.publish: DISABLED` when an RPC must never enter Gateway manifests, even if it has `google.api.http` for direct service HTTP use.

```proto
option (aisphere.access.v1.policy) = {
  exposure: INTERNAL
  gateway: { publish: DISABLED tags: "dtm" }
};
```

`GatewayPublish` is tri-state:

- `GATEWAY_PUBLISH_UNSPECIFIED`: generators include the route and deployment filters decide the target profile.
- `ENABLED`: explicitly include the route.
- `DISABLED`: do not generate the route into `GatewayManifest`.

## Gateway profiles

Gateway publication has two layers:

1. **Generation-time**: `gateway.publish: DISABLED` removes a route from generated manifests.
2. **Registration-time**: `gatewayx.RouteFilter` chooses which manifest routes are written to a concrete registry.

Recommended filters:

```go
serverx.RegisterServiceGatewayRoutesWithFilter(ctx, registry,
    gatewayx.PublicRouteFilter(),
    myv1.MyServiceKernelModule(),
)
```

`gatewayx.PublicRouteFilter()` includes:

- `PUBLIC`
- `AUTHENTICATED`
- `AUTHORIZED`

and excludes:

- `INTERNAL`
- `SYSTEM`
- `/internal/*`
- `/debug/*`
- `/metrics`
- `/healthz`
- `/readyz`

`gatewayx.InternalRouteFilter()` can publish service-to-service APIs but still excludes debug/ops-only paths.

## Decision table

| Route type | Public Gateway | Internal Gateway | Direct/Ops |
|---|---:|---:|---:|
| Product business API | yes | optional | no |
| Login / OAuth entry | yes, usually IAM | optional | no |
| Token verify / permission check | no | yes | optional |
| Service-to-service APIs | no | yes | no |
| DTM branch / compensate | no | usually no | direct only |
| healthz / readyz | no | no | yes |
| metrics | no | no | yes |
| debug / pprof | no | no | temporary ops only |
| repair / migrate / bootstrap | no | no | ops + audit only |

## Service guidance

- Do not rely on `google.api.http` alone for Gateway exposure.
- Put stable product APIs behind proto `access.policy`.
- Put direct-only internal HTTP routes outside proto or set `gateway.publish: DISABLED`.
- Public gateway registries should always use `PublicRouteFilter`.
- Internal gateway registries should use `InternalRouteFilter` and still avoid debug/ops APIs.
