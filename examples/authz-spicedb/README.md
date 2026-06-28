# AuthZ SpiceDB integration example

This example exercises Kernel `authz` against a real local SpiceDB instance. It
uses only Kernel interfaces after bootstrapping the adapter; application code
should not call the AuthZed/SpiceDB SDK directly.

## 1. Start SpiceDB locally

For a quick development instance:

```powershell
spicedb serve `
  --grpc-preshared-key dev-token `
  --datastore-engine memory `
  --grpc-addr :50051
```

Docker alternative:

```powershell
docker run --rm -p 50051:50051 authzed/spicedb:latest serve `
  --grpc-preshared-key dev-token `
  --datastore-engine memory `
  --grpc-addr :50051
```

## 2. Prepare config

```powershell
Copy-Item .\examples\authz-spicedb\config.example.yaml .\examples\authz-spicedb\config.local.yaml
```

Set `authz_example.subject_id` to the `subject` printed by the authn Casdoor
example callback result.

## 3. Run the example

```powershell
go run .\examples\authz-spicedb -config .\examples\authz-spicedb\config.local.yaml
```

The example performs:

1. Read/write SpiceDB schema through `authz.SchemaManager`.
2. Write organization, group, application, project and resource relationships.
3. Run positive and negative `authz.Authorizer.Check` calls.
4. Run `authz.ResourceLookup` and `authz.SubjectLookup` calls.
5. Read relationships back through `authz.RelationshipStore`.
6. Exercise `auditx` through `authz.NewAuditedAuthorizer`.

The default schema is intentionally a Kernel demo schema, not final product
policy. Once this is green, move product-specific resource names such as
`agent`, `workflow`, `dataset`, `mcp_server`, etc. into the production schema.
