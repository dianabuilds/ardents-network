---
name: ardents-release-code-review
description: Release-focused code review workflow for Ardents. Use when preparing a release or reviewing a release-sized change set and you need to find logic bugs, behavioural regressions, unsafe assumptions, document drift, missing tests, or hidden blockers before shipping.
---

# Ardents Release Code Review

Use this skill for release review, not for design or implementation.

This skill complements:
- `ardents-acceptance-gate` for final slice acceptance
- `ardents-release-bug-hunt` for deeper failure-oriented bug search
- `ardents-release-error-handling-review` for error-path discipline

## Read First

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/development-contract.md`
- `docs/engineering-constraints.md`
- the relevant domain document
- `docs/reference-invariants.md` and `docs/canonical-network-foundation.md` if the release touches network/discovery/messaging/publication

## Review Goal

Decide whether the code about to ship is trustworthy enough for release.

Prioritise:
- real bugs
- behavioural regressions
- document drift
- missing failure handling
- missing tests for risky paths
- false product claims

## Workflow

1. State the release surface under review: files, domain, and intended capability.
2. Identify the code paths most likely to hide regressions: changed hot paths, startup/shutdown, persistence, network flows, service publication, policy/security checks.
3. Compare the code against the system documents and domain requirements.
4. Review for correctness first, not style.
5. Check whether failure and degraded paths are covered by code and tests.
6. Check whether the change introduces new claims the product cannot actually satisfy.
7. Produce findings ordered by severity.

## Mandatory Checks

- no contradiction with current docs
- no fake foundation or symbolic critical-path behavior
- no silent behaviour change in startup, shutdown, persistence, network presence, discovery, trust, workload execution, or publication
- no hidden dependency on disabled or untested paths
- no release-sized change without tests in the risky path
- no stale comments, names, or diagnostics that lie about runtime behavior

## Reject If

- the release depends on a path that is still heuristic-only in a critical plane
- the code claims product behaviour that is not operationally real
- risky paths changed without test coverage
- the release changes semantics but leaves docs and diagnostics behind
- the review can only conclude "probably works"

## Output

When using this skill, produce:

- reviewed release surface
- highest-severity findings first
- release decision: ready or blocked
- exact blockers and missing checks if blocked
