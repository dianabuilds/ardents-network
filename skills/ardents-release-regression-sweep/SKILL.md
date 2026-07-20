---
name: ardents-release-regression-sweep
description: Final cross-cutting release sweep for Ardents. Use after implementation and focused reviews when you need one prepared flow that checks the release as a product: startup, recovery, diagnostics, local API truth, network truth, publication truth, tests, and document alignment across the changed domains.
---

# Ardents Release Regression Sweep

Use this skill near the end of a release once focused reviews are done.

This is the product-level sweep that ties together:
- `ardents-release-code-review`
- `ardents-release-bug-hunt`
- `ardents-release-vulnerability-review`
- `ardents-release-error-handling-review`
- `ardents-acceptance-gate`

## Read First

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/development-contract.md`
- the relevant domain documents

## Sweep Goal

Answer one question:

"Does this release still behave like Ardents, or did the changed slices quietly break product truth?"

## Workflow

1. State the release scope.
2. List the domains and runtime flows touched by the release.
3. Re-check startup, shutdown, restart, diagnostics, local API truth, and persistence truth across those flows.
4. Re-check network truth, discovery truth, trust impact, and service publication truth if the release touches them.
5. Re-check tests and known review findings.
6. Re-check docs and release claims against the actual behaviour.
7. Decide whether the release is shippable.

## Mandatory Cross-Cutting Checks

- startup, stop, and restart still work
- diagnostics still explain degraded and failed states
- local API still reflects runtime truth
- no changed domain now depends on a fake or symbolic foundation
- service publication still follows runtime truth
- network/discovery behaviour still respects canonical foundation when touched
- tests cover changed behaviour and one representative broken-path outcome
- release notes or claims do not overstate what the code actually does

## Reject If

- all focused reviews passed but the product story is still inconsistent
- changed slices work in isolation but violate system truths together
- release behaviour depends on local-only approximations in a critical plane
- the release is only "green" because the dangerous path is untested or disabled

## Output

When using this skill, produce:

- release scope
- cross-cutting checks performed
- residual regression risks
- shippable / blocked decision
