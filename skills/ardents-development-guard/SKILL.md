---
name: ardents-development-guard
description: Guardrail skill for enforcing the Ardents development contract. Use when starting, reviewing, or expanding any substantial code change so the work stays product-grade, avoids prototype-first development, and does not introduce fake foundations or deferred critical behavior.
---

# Ardents Development Guard

Use this skill before any substantial implementation or review in the root product.

## Read First

- `docs/development-contract.md`
- `docs/engineering-constraints.md`
- `docs/system-frame.md`
- the specific domain document affected by the change
- `docs/reference-invariants.md` and `docs/canonical-network-foundation.md` if the change touches network/discovery/messaging/publication

## What This Skill Enforces

- No prototype-first development in critical domains.
- No fake foundation in network, data, workload execution, or other critical planes.
- No "we will finish the mandatory properties later" slices.
- No silent drift away from the fixed system documents.

## Workflow

1. State the owning domain.
2. State the mandatory product properties of that domain.
3. Check whether the planned change implements those properties in a real way.
4. Reject the change if it introduces a temporary substitute that looks like a foundation.
5. Reject the change if it defers mandatory domain properties to a later unspecified step.
6. Allow the change only when the slice is product-grade for the properties it touches.

## Red Flags

- "Temporary transport core"
- "Fake executor for now"
- "Metadata-only storage first"
- "Neutral network abstraction until we decide substrate"
- "Policy stub, enforcement later"
- "We will add diagnostics once behavior stabilizes"

If the change sounds like one of these, stop and redesign before coding.

## Acceptance Gate

Before approving a change, confirm:

- the owning domain is explicit;
- the change does not violate `development-contract.md`;
- the change does not violate `engineering-constraints.md`;
- the slice leaves no mandatory product property deferred;
- the result will be observable through diagnostics and/or local control surface.
