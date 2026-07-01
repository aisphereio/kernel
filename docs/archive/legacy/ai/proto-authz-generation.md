# Proto authz generation

`protoc-gen-go-authz` reads Aisphere method options from proto descriptors and emits Go authz metadata plus a secure client wrapper.

## Options

Defined in `api/aisphere/options/v1/authz.proto`:

```proto
extend google.protobuf.MethodOptions {
  AuthzRule authz = 51001;
  AuditRule audit = 51002;
  CapabilityRule capability = 51003;
}
```

A typical external RPC should look like this:

```proto
rpc DownloadSkillPackage(DownloadSkillPackageRequest) returns (DownloadSkillPackageReply) {
  option (google.api.http) = {
    get: "/api/v1/skills/{skill_id}/package"
  };
  option (aisphere.options.v1.authz) = {
    action: "skill.download"
    resource: "skill:{skill_id}"
    audience: "skill-service"
    mode: SCOPED_TOKEN
  };
  option (aisphere.options.v1.audit) = {
    event: "skill.package.download"
    risk: "medium"
  };
  option (aisphere.options.v1.capability) = {
    group: "skill"
    name: "download"
  };
}
```

## Generated Go

For a service named `SkillService`, the plugin generates:

```go
var SkillServiceAuthzRules = authz.Rules{...}
const SkillServiceAuthzManifestJSON = `...`
type SkillServiceSecureClient struct { ... }
func NewSkillServiceSecureClient(raw SkillServiceClient, guard authz.Guard) *SkillServiceSecureClient
```

The plugin intentionally does not bind to SpiceDB, OpenFGA, OPA, Casdoor or any other concrete backend. It only converts proto contract rules into Kernel runtime objects. The concrete IAM service implements `authz.Guard`.

## Rule mode guidance

| Mode | Use when | Runtime behavior |
|---|---|---|
| `CHECK_ONLY` | Internal service calls where a decision token is enough | Generated client calls `Guard.Require` before invoking RPC. |
| `SCOPED_TOKEN` | Gateway/BFF calls downstream services and needs delegated proof | Generated client calls `Guard.RequireScopedToken` and propagates scoped token. |
| `SELF_CHECK` | High-risk resource-owner operations | Generated client does not pre-check; target service must check at boundary. |
| `UNSPECIFIED` | Never use | `buf-check-aisphere` reports it as a contract violation. |

Resource templates support request field references, including nested paths:

```proto
resource: "skill:{skill_id}"
resource: "skill:{owner.id}:{skill_id}"
```

`authz.RuleResolver` and `buf-check-aisphere` now validate/resolve these paths consistently.

## Contract check before generation

Run the checker against a descriptor set:

```bash
buf build -o - | buf-check-aisphere
```

The checker enforces:

1. `google.api.http` external methods must declare `aisphere.authz`.
2. authz `action/resource/audience/mode` must not be empty or unspecified.
3. high-risk actions must declare `aisphere.audit`.
4. resource template fields must exist on the request message.

## Why custom unknown-option parsing?

The generator and checker parse encoded `google.protobuf.MethodOptions` extension payloads directly by extension field number. This keeps them independent from generated Go code for the options proto during bootstrapping. Once `make api` runs, normal generated option code can still be produced for consumers.

The parsing logic is centralized in `internal/protooptions` so `protoc-gen-go-authz` and `buf-check-aisphere` cannot drift apart.
