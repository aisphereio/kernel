# kernel latest code package

This archive contains source code only. It excludes offline Go proxy cache, Go toolchain archives, protoc/buf archives, compiled binaries, and sandbox demo logs.

Included major changes:

- `aisphere.access.v1.policy` proto contract.
- `buf-check-aisphere` access-policy checks.
- `protoc-gen-go-authz` generated access resolver support.
- `middleware/ctxinject`, `middleware/authn`, `middleware/access`, `middleware/autowire`.
- HTTP route static-priority fix.
- grpcx selector fallback fix.
- `layout/cmd/fullflow-smoke` full-chain demo.
- Agent development rules in `AGENTS.md`.
