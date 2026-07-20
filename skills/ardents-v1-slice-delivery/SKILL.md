---
name: ardents-v1-slice-delivery
description: Delivery workflow for implementing or expanding a v1 slice in the Ardents Network root codebase. Use when adding functionality to internal packages, cmd/ardd, or local control surfaces while preserving the reduced domain map and avoiding aim-core-style over-architecture.
---

# Ardents V1 Slice Delivery

Use this skill when implementing real functionality in the root codebase.

## Start With Scope

Define the slice in one sentence:

- what user-visible or operator-visible capability is being added;
- which root package owns it;
- which documents define its acceptance.

## Read Only What You Need

Start with:

- `docs/development-contract.md`
- `docs/engineering-constraints.md`
- `docs/system-frame.md`
- the specific domain doc for the slice

Then inspect:

- the owning root package and its tests;
- `docs/reference-invariants.md` and `docs/canonical-network-foundation.md` if the slice touches network/discovery/messaging/publication.

## Delivery Workflow

1. Confirm the owning root package.
2. Confirm the slice will be product-grade and not a temporary prototype.
3. Define the smallest vertical slice that creates real behavior.
4. Prefer extending current root packages over creating new top-level layers.
5. Implement the slice behind the existing local control surface where applicable.
6. Add tests that prove behavior, not just structure.
7. Run `go test ./...` or the narrowest relevant package tests first, then broaden if needed.

## Design Bias

- Prefer one obvious path over extensibility scaffolding.
- Prefer explicit state and reasons over generic abstractions.
- Prefer explainable degraded behavior over hidden partial failure.
- Add interfaces only where they enable a real boundary such as workload execution or transport adapters.
- If the slice touches a critical plane, implement its mandatory product properties in the same slice instead of deferring them.

## Do Not Do

- Do not introduce a second canonical API surface.
- Do not add placeholder layers for hypothetical future adapters.
- Do not add generic builders or facades when direct assembly is enough.
- Do not mark a slice complete if it only stores config without enforcing runtime behavior.
- Do not ship fake executors, fake network foundations, metadata-only data planes, or other prototype substitutes as normal product paths.

## Done Means

A slice is done only when:

- it changes actual runtime behavior;
- tests cover success and at least one failure or degraded path;
- diagnostics or status surfaces explain the outcome;
- the implementation still fits the current `v1` system documents;
- the slice does not leave critical domain properties deferred as "future work".
