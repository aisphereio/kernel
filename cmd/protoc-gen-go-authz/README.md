# protoc-gen-go-authz

`protoc-gen-go-authz` turns Aisphere proto method options into provider-neutral Kernel authorization helpers.

It reads these options from `api/aisphere/options/v1/authz.proto`:

```proto
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
```

Generated helpers:

```go
var SkillServiceAuthzRules authz.Rules
const SkillServiceAuthzManifestJSON = `...`
type SkillServiceSecureClient struct { ... }
func NewSkillServiceSecureClient(raw SkillServiceClient, guard authz.Guard) *SkillServiceSecureClient
```

Runtime rule: the generator does not depend on SpiceDB, OpenFGA, OPA, Casdoor, or any concrete IAM backend. It only converts proto contracts into `github.com/aisphereio/kernel/authz` rules. Deployment code supplies an `authz.Guard` implementation.

## Rule modes

| mode | Generated client behavior |
|---|---|
| `CHECK_ONLY` | Calls `Guard.Require` before the target RPC and propagates the decision token in `contextx`. |
| `SCOPED_TOKEN` | Calls `Guard.RequireScopedToken`, then propagates the scoped token and decision token in `contextx`. |
| `SELF_CHECK` | Does not pre-check on the client; the resource service must enforce authorization at its own boundary. |
| `UNSPECIFIED` | Treated as an invalid generated contract. `buf-check-aisphere` should catch this before generation. |

Resource templates support top-level and nested request fields, for example `skill:{skill_id}` and `skill:{owner.id}:{skill_id}`.

## Build integration

```yaml
plugins:
  - local: protoc-gen-go-authz
    out: .
    opt:
      - paths=source_relative
```

Before generation, run `buf-check-aisphere` against the descriptor set to fail fast on missing authz/audit contracts.
