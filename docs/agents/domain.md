# Domain Docs

This repository uses a single-context domain documentation layout.

## Before exploring

Read these resources when they exist:

- `CONTEXT.md` for canonical domain terminology
- Relevant ADRs under `docs/adr/`

Proceed silently when they do not exist. The `domain-modeling` skill creates them lazily when terminology or architectural decisions are resolved.

## Use canonical vocabulary

Use terminology defined in `CONTEXT.md` in issue titles, specifications, tests, and implementation. Avoid introducing synonyms for concepts already defined there.

When a required concept is missing, reconsider whether it is needed or note the gap for `domain-modeling`.

## Respect architectural decisions

Surface any conflict with an existing ADR explicitly instead of silently overriding it.
