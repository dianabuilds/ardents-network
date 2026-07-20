---
name: ardents-acceptance-gate
description: Final validation workflow for Ardents slices and substantial code changes. Use when implementation is believed to be complete and you need to verify acceptance: tests, diagnostics visibility, document alignment, code-size limits, and absence of deferred critical behavior.
---

# Ardents Acceptance Gate

Use this skill only near the end of a slice or substantial change.

This skill is not for architecture design, dependency selection, or domain implementation.
It is the final gate that decides whether the change is actually acceptable.

## Read First

- `docs/development-contract.md`
- `docs/engineering-constraints.md`
- the relevant domain document
- `skills/ardents-code-size-guard/SKILL.md` when handwritten Go code changed

## Distinct Role

- `ardents-development-guard` stops prototype-first work before or during implementation.
- `ardents-architecture-guard` checks architectural fit.
- `ardents-v1-slice-delivery` guides implementation.
- `ardents-acceptance-gate` runs after that work and answers one question: "is this slice acceptable right now?"

## Acceptance Workflow

1. State the owning domain and the intended capability.
2. Verify the change against the domain document's required properties.
3. Verify there is no deferred critical behavior.
4. Run the relevant tests.
5. Run code-size checks if handwritten Go code changed.
6. Verify diagnostics and/or local control surface can explain the new behavior.
7. Verify document alignment if behavior or boundaries changed.
8. Accept or reject the slice.

## Required Checks

- tests pass for the changed slice
- at least one failure or degraded path is covered
- no fake foundation or temporary substitute remains in the delivered path
- diagnostics or status surface explains the outcome
- code size limits are not breached, or breaches have an explicit refactor plan
- documents still match the code

## Reject If

- the slice only proves happy-path behavior
- mandatory domain properties are still deferred
- diagnostics cannot explain the result
- the code contradicts the current documents
- the change passed locally only because the critical path is still fake or disabled

## Output

When using this skill, produce:

- capability being validated
- acceptance checks performed
- pass/fail decision
- exact blockers if the slice is rejected
