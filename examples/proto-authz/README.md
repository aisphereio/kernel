# proto-authz example

This example is a proto contract input for Kernel authz/audit/capability generator behavior.

It intentionally does not commit `.pb.go` or generated authz files. To exercise it, run the Kernel generator flow in a temporary output directory or through the repository validation targets.

Rules:

1. Treat `skill.proto` as source input, not a checked-in generated package.
2. Do not import this example from runtime code.
3. Keep generator outputs disposable unless a dedicated example runner documents how to build them.
