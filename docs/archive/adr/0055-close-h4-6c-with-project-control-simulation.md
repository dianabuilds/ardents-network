---
status: accepted
date: 2026-08-29
supersedes: ADR-0054 H4-6C consequence only
partially-superseded-by: ADR-0060 (maintained generator and command consequence only)
---

# ADR-0055 — Close H4-6C with project-control simulation

## Context

The active project team is the Product Owner and Codex. The Product Owner has
explicitly decided that these are the only actors relevant to H4-6C. Requiring
unavailable outside people as a completion condition would turn the horizon
into an unbounded recruitment task rather than a checkable engineering slice.

## Decision

H4-6C is a project-controlled shared-control mechanics simulation. Its accepted
evidence is one versioned JSON receipt from `ardents-control
simulate-public-control --source-revision LOWERCASE_40_HEX_COMMIT`, retained outside the repository,
its test matrix, and the R-124 contract. The receipt records a caller-declared
source revision, contract version, pass/fail cell lists, and a receipt digest.
It simulates five custody roles with
`3-of-5` routine and expiring disable-only `4-of-5` emergency actions,
bidirectional rotation/recovery, complete Candidate View reconstruction, two
builder roles, two auditor roles, and all defined failure outcomes.

The simulation creates no persistent authority, does not select a real public
candidate, and makes no Public Beta or independent-operation claim. A future
public claim, if the Product Owner ever requests one, needs a new decision and
its own evidence; it is not a remaining H4-6C requirement.

## Consequences

- The Product Owner and Codex may complete H4-6C without recruiting or
  impersonating outside custodians, builders, or auditors.
- `external-evidence-required` remains a reader result for a future public
  claim, not a failure of this simulation.
- ADR-0054's statement that H4-6C requires real independent actors is
  superseded; its H4-6B transition separation remains unchanged.

## Compliance

- ADR-0004, ADR-0038, ADR-0054
- [R-124](../research/records/r-124-public-control-candidate-evidence.md)
- [R-124 project-control evidence](../research/records/r-124-public-control-candidate-evidence.md)
