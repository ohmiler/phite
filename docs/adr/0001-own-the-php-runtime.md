---
status: accepted
---

# Own the PHP runtime with FrankenPHP

Phite will supply and control PHP through official FrankenPHP distributions run as a Managed Runtime subprocess instead of requiring system PHP, a separately configured web server, Docker, or an embedded FrankenPHP library. The pure-Go Phite control process will download, verify, cache, start, and stop the appropriate platform distribution. This makes first use network-dependent and gives up literal one-executable distribution, but preserves zero-manual-runtime onboarding while decoupling runtime upgrades and avoiding a per-platform CGO/PHP build pipeline.
