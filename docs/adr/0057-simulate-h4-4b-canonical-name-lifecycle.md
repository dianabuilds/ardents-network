---
status: accepted
date: 2026-08-29
supersedes: ADR-0022 target-validity consequence for H4-4B; ADR-0043 H4-4B completion consequence only
---

# ADR-0057 — Simulate H4-4B canonical Name lifecycle

## Context

ADR-0043 selects proof-derived Grace but deliberately leaves H4-4B without one
complete lifecycle evidence path. The project team is the Product Owner and
Codex; public operators or a public Namespace are not prerequisites for the
selected simulation.

## Decision

H4-4B is completed as a bounded local simulation through `ardents-control
simulate-namespace-lifecycle --source-revision LOWERCASE_40_HEX_COMMIT`. It creates a fresh
temporary Store, appends immutable signed successors, and makes each state
current only through `EpochInstallation.Commit` with a `2-of-3` threshold
attestation. It proves publication/update, Active-to-Grace warning,
materialized Released unavailability, reclaim only as generation two, restart,
and rejection of stale replay, stale fork attempt, and old-generation reclaim.

For H4-4B only, a published Target may remain valid through its signed Grace
boundary. This narrowly supersedes ADR-0022's earlier Lease-bound consequence:
the target remains under the same Record/parent validity and then fails closed
at Grace expiry; it does not authorize a later renewal or a public claim.

An alpha corpus is never input to this simulation. The receipt is explicitly
`simulation: true` and `qualified: false`; temporary keys and store are removed
after the run.

## Consequences

- Published Targets remain usable through the signed Grace boundary, then fail
  once a Released state is materialized.
- Current state is a threshold-attested Namespace Epoch, never a caller-built
  corpus, local clock transition, or alpha corpus.
- This closes only project-controlled H4-4B evidence. Public Epoch operation,
  governance, canonical Namespace availability, and Public Beta remain
  unselected.

## Compliance

- ADR-0020, ADR-0022, ADR-0023, ADR-0043, ADR-0055, and ADR-0056
- [R-126](../research/records/r-126-project-control-canonical-name-lifecycle.md)
