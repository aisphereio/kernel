# OpenAPI contract governance

Kernel services use protobuf definitions as the source of truth and publish a normalized, tracked Swagger 2.0 document for frontend and external consumers.

## Contract pipeline

1. Define HTTP and message contracts in protobuf.
2. Run `buf lint` and the repository breaking-change check.
3. Generate Swagger with `protoc-gen-openapiv2`.
4. Normalize and validate it with `openapi-contract`.
5. Commit the normalized document and reject generation drift in CI.
6. Pin the producing backend commit and contract SHA-256 in each consumer repository before generating an SDK.

The normalized contract is a build artifact with source control history. A Swagger UI may render it, but the UI is not the source of truth.

## Command

Build the reusable command with `make tools`, or invoke it directly:

```text
go run ./cmd/openapi-contract \
  --input docs/openapi/generated.swagger.json \
  --output docs/openapi/service.swagger.json \
  --title "Example Service API" \
  --version "<source commit>"
```

The command requires operation IDs and tags, makes the output deterministic, and normalizes every default operation error to `#/definitions/KernelErrorResponse`.

`KernelErrorResponse` is aligned with Kernel HTTP error transport and exposes stable `code` and `message` fields plus optional `request_id`, `trace_id`, and structured `metadata`. Services must not publish framework-specific error schemas as their public default response.

## CI requirements

A service repository should run these checks on the pull request HEAD with read-only repository permissions:

- protobuf lint and breaking-change checks;
- generator package tests and normalized OpenAPI generation in a temporary workspace;
- deterministic repeat-generation checks and `git diff --exit-code` over the repository;
- backend tests and compilation;
- consumer lock verification and SDK generation drift checks.

Delivery workflows may publish immutable images and rendered deployment YAML. They must not rewrite source branches or deploy to a cluster. Release metadata should connect the image digest, source commit, normalized contract, and consumer lock.
