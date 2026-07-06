# IAMService validation contract

`validation/iamservice` is a scenario-validation input for Kernel proto/generator checks. It is not a runtime API package and business repositories must not import it.

This directory proves the IAM service can follow the Kernel development flow:

1. `kernel new iam-service --repo ./layout`
2. Write `api/iam/v1/iam.proto`
3. Run `make api` / validation generation into a temporary output directory
4. Wire business code through Kernel `authn`, `authz`, `accessx` and `serverx`
5. Replace fake providers with `authn/casdoor` and `authz/spicedb` in production

Rules:

1. Keep `.proto` contracts here small and focused on Kernel generator behavior.
2. Do not commit `.pb.go`, grpc stubs or gateway stubs under this directory.
3. Validation jobs should generate into a temporary output directory.
4. New proto imports must use canonical Kernel proto paths, for example `api/aisphere/access/v1/access.proto`.

The short wrapper import `aisphere/access/v1/access.proto` is retained only for compatibility with older generated projects.
