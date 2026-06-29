# Proto authz generation

`protoc-gen-go-authz` reads Aisphere method options from proto descriptors and
emits Go authz metadata plus a secure client wrapper.

## Options

Defined in `api/aisphere/options/v1/authz.proto`:

```proto
extend google.protobuf.MethodOptions {
  AuthzRule authz = 51001;
  AuditRule audit = 51002;
  CapabilityRule capability = 51003;
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

The plugin intentionally does not bind to SpiceDB, OpenFGA, OPA, Casdoor or any
other concrete backend. It only converts proto contract rules into Kernel
runtime objects. The concrete IAM service implements `authz.Guard`.

## Why custom unknown-option parsing?

The generator parses the encoded `google.protobuf.MethodOptions` extension
payloads directly by extension field number. This keeps the generator independent
from generated Go code for the options proto during bootstrapping. Once `make
api` runs, normal generated option code can still be produced for consumers.
