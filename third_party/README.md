# third_party

`third_party/` contains vendored proto dependencies that are not authored by Kernel.

Rules:

1. Do not place Kernel-owned proto files here.
2. Do not add compatibility copies unless a current proto imports them.
3. Keep `third_party/buf.yaml` isolated from the root `buf.yaml`; it may stay on an older Buf module format when the vendored tree requires it.
4. Remove orphan proto files instead of keeping stale generated-plugin copies.

The former `third_party/errors/errors.proto` copy was removed because no Kernel proto imports it. Kernel-owned error contracts should use the current `errorx` / generator path, not a vendored orphan proto.
