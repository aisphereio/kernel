# contrib

`contrib/` is reserved for optional, non-core adapters and integrations.

Current rule:

1. Empty subdirectories under `contrib/` are placeholders only.
2. New business code must not assume a contrib package exists until it contains Go files and appears in `PACKAGE_INDEX.md` / `docs/contracts/package-status.md`.
3. Core Kernel runtime packages must not depend on contrib packages.
4. A contrib adapter must document its owner, status, replacement path and runtime dependency cost before it is treated as supported.

If a placeholder remains empty for multiple releases and has no planned owner, delete the placeholder instead of leaving a misleading package surface.
