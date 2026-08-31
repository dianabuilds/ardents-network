---
status: accepted
date: 2026-08-29
supersedes: none
partially-superseded-by: ADR-0060 (maintained generator and command consequence only)
---

# ADR-0056 — Simulate H4-6D controlled project-control transitions

## Context

H4-6C selected the Product Owner-and-Codex mechanics simulation. Its receipt
does not by itself demonstrate whether an unsafe successor is refused without a
fallback or local repair.

## Decision

H4-6D is a separate local project-control transition simulation. Its accepted
evidence is the behavior test matrix and one versioned JSON receipt from
`ardents-control simulate-public-control-transitions --source-revision LOWERCASE_40_HEX_COMMIT`,
retained outside the repository. The matrix accepts only continuous overlap and
requires exact stop/unavailable outcomes for expiry, revocation, incompatible
generation, rollback, distribution outage, and a live disable-only emergency.
It rejects missing continuity, emergency scope escalation, and emergency expiry.

The simulator has no network, persistent keys, authority, Endpoint root, or
fallback. It is always `simulation: true` and `qualified: false`.

## Consequences

- The Product Owner and Codex can reproduce the exact H4-6D transition boundary
  in one short local run.
- A stopped or unavailable result can never justify an alternate source, older
  generation, Route, Namespace, or Endpoint action.
- This decision makes no claim about public operation, independent control,
  availability, or Public Beta; any such claim remains a separately selected
  future programme.

## Compliance

- ADR-0004, ADR-0054, and ADR-0055
- [R-125](../research/records/r-125-controlled-project-control-transitions.md)
- [R-125 controlled-transition evidence](../research/records/r-125-controlled-project-control-transitions.md)
