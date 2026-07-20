---
name: ardents-architecture-guard
description: Guardrail skill for architecture-aligned changes in the Ardents Network repo. Use when changing domain boundaries, repo structure, local API shape, product surfaces, or when deciding whether logic from aim-core should be reused, rewritten, or retired.
---

# Ardents Architecture Guard

Use this skill to keep changes aligned with the active `v1` architecture.

## Read First

Read only the documents needed for the task, starting with:

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/development-contract.md`
- `docs/engineering-constraints.md`

Then read:

- the specific domain documents involved in the change;
- `docs/reference-invariants.md` if the change touches network/discovery/messaging/publication;
- `docs/canonical-network-foundation.md` if the change touches the network plane.

## Working Rules

- Treat `docs/` as the architectural source of truth.
- Treat `aim-core` as reference source for invariants and proven mechanisms, not as package-layout template.
- Preserve the fixed domain set from `system-frame.md`.
- Preserve `local control surface` as the canonical management surface, not as a product domain.
- Do not reopen the question of canonical network foundation unless documents explicitly change.
- Do not accept prototype-first domain shapes.

## Decision Workflow

For any non-trivial change:

1. State which bounded context owns the change.
2. State whether the change affects product surface, domain logic, workflow, or cross-cutting enforcement.
3. Check whether the change fits the current domain map.
4. If it does not fit, update architecture docs before editing code.
5. If legacy code is relevant, classify it as `Keep As Is Temporarily`, `Extract And Reuse`, `Rewrite Under New Boundary`, or `Retire`.
6. Check whether the change introduces fake foundations, temporary substitutes, or deferred critical behavior; if yes, stop and redesign before coding.

## Prohibited Moves

- Do not recreate parallel external contract systems by default.
- Do not introduce runtime facade layers as central ownership.
- Do not multiply domains into `domain/usecase/service/contracts/sdk` unless the task clearly requires it and documents justify it.
- Do not move product semantics into protocol plumbing packages.
- Do not invent a neutral transport core that bypasses `Waku` for `v1`.

## Expected Output

When using this skill, produce:

- the owning domain or slice;
- the architectural fit check;
- the reuse or rewrite decision if legacy is involved;
- the minimal code path that implements the change without reopening legacy structure.
