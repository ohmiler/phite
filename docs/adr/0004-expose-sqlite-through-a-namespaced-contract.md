---
status: accepted
---

# Expose SQLite through a namespaced contract

Phite v0.1 will create a Project Database as Local State and expose it to the PHP process through PHITE_DATABASE_DRIVER, PHITE_DATABASE_PATH, and PHITE_DATABASE_URL. It will not override generic DATABASE_URL or framework-specific variables such as DB_*, so existing application behavior remains unchanged unless the PHP Project explicitly consumes the Database Contract. This gives up automatic database wiring for arbitrary frameworks in exchange for a safe, framework-neutral contract and non-mutating Development Sessions.
