---
status: accepted
---

# Version and validate Project Configuration

Optional Project Configuration lives in phite.yaml and declares its Configuration Schema explicitly, beginning with schema 1. Paths are relative to the PHP Project, validation rejects unknown keys, and v0.1 configuration is limited to Document Root, Project Database, and Live Reload watch overrides; PHP selection will not appear until multiple Managed Runtime versions exist. Before 1.0, breaking CLI or Configuration Schema changes are limited to minor releases with migration notes, while patch releases preserve behavior contracts and Runtime Identity.
