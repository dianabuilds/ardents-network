---
status: accepted
date: 2026-08-24
supersedes: ADR-0011 (generic `tests/live/` location only)
---

# ADR-0031 — Retire the generic live-test tree

## Context

ADR-0011 correctly separates unit, end-to-end, and selected live evidence, but
its fixed `tests/live/` path made an inactive profile look like maintained test
surface. The Stage 5 live corpus is retired and no current Route/Node runtime
selects a live scenario.

## Decision

Keep the three evidence classes, their independent setup, and their explicit
entrypoints. Retire the generic `tests/live/` tree and its implied build tag.
A future selected live scenario creates one purpose-named boundary, registers
its profile and entrypoint, and owns complete setup, teardown, and environment
failure handling in the same change.

## Consequences

- an inactive live profile has no empty source tree or passing skip;
- Docker or VPS observations remain integration evidence, not live-profile
  selection; and
- ADR-0011 continues to govern independence of unit, e2e, and live evidence.
