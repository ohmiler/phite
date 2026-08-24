---
status: accepted
---

# Pin immutable runtime artifacts

Each Phite release will select Managed Runtimes through a Runtime Manifest and immutable mirrored artifacts rather than resolving mutable upstream release assets during a Development Session. Runtime Identity includes the actual FrankenPHP and PHP versions, platform, extension set, and checksum; verified artifacts are shared through the Runtime Cache. The v0.1 runtime supports PHP 8.5 and its pinned extensions only, failing clearly when Composer declares incompatible PHP or ext-* requirements. This adds release storage and maintenance work but makes runtime behavior reproducible and supply-chain verification possible.
