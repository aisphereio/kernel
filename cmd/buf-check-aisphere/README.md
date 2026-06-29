# buf-check-aisphere

`buf-check-aisphere` is a descriptor-level contract checker for Aisphere proto APIs. It is designed to run after `buf build -o -` and before code generation or CI merge.

It currently checks:

1. RPC methods exposed through `google.api.http` must declare `aisphere.authz`.
2. `aisphere.authz.action`, `resource`, `audience`, and `mode` must be set.
3. High-risk actions such as `delete`, `publish`, `grant`, `share`, `transfer`, `owner`, `admin`, `remove`, and `revoke` must declare `aisphere.audit`.
4. High-risk audited methods should set `aisphere.audit.risk`.
5. Resource templates such as `skill:{skill_id}` and `skill:{owner.id}` must reference existing request fields.

Example:

```bash
buf build -o - | buf-check-aisphere
```

Override high-risk action tokens:

```bash
buf build -o - | buf-check-aisphere --high-risk-actions=delete,publish,grant
```

The checker parses encoded unknown option payloads by extension number. This keeps it independent from generated Go code for the options proto and aligns it with `protoc-gen-go-authz`.
