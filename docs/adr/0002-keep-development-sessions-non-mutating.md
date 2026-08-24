---
status: accepted
---

# Keep Development Sessions non-mutating

Starting a Development Session must not rewrite a PHP Project's source or version-controlled configuration; Phite may only write Local State. Explicit setup commands may propose Project Configuration or ignore-file changes, but must show those changes before applying them. This gives up automatic repair of application configuration in exchange for predictable startup and Developer trust.
