---
status: accepted
---

# Run Project commands through the Managed Runtime

Phite v0.1 will expose Project-scoped PHP and Composer execution as Runtime Commands so a Developer does not need system PHP after installing Phite. Composer is a pinned, verified runtime artifact. Starting a Development Session will validate dependencies but will not install them automatically; when dependencies are absent, Phite will direct the Developer to the explicit Composer Runtime Command. This adds two CLI surfaces but preserves non-mutating Development Sessions while making the Managed Runtime useful for normal PHP workflows.
