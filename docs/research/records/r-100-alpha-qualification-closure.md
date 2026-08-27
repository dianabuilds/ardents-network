---
id: R-100
title: Usable-alpha qualification and repository closure
status: open
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-100 — What exact usable-alpha release profile, live/soak matrix, remediation rule, and closure inventory can yield reproducible evidence and a maintainable repository without mislabeling the result Public Beta?

## Decision this unlocks

Select H4-8A's first release-readiness matrix and H4-8D's closure procedure
once a usable H4-1–H4-3 profile exists.

## Current contract

- Qualification applies to one exact build, topology, platform, workload and
  claim boundary; it is not a final smoke-test label.
- Live and soak profiles are inactive until their selected scenario has a named
  purpose, environment, oracle, duration and lifecycle.
- Current documentation ownership requires promotion of unique facts before
  retiring stage material; Git is provenance, not a second specification.

## Hypotheses

- **H1:** a finite alpha matrix can combine deterministic/process/platform/live
  and soak evidence with a closure inventory without pretending to be Public
  Beta qualification.
- **H2:** the alpha should omit soak or some host cells until a narrower exact
  profile can make their results meaningful.
- **H0:** no profile is coherent enough to qualify; H4-8 must return work to
  its owning epic rather than create an unbounded test campaign.

## Evaluation criteria

- exact candidate/profile/topology/claims/non-claims and attempt denominator;
- normal, failure, recovery, overload, update, removal and cleanup oracles;
- live host prerequisites, duration/load/fault contract, artifact retention and
  invalid-environment handling;
- defect disposition and requalification validity after any change;
- closure inventory: current owner, unique fact, references/tests, provenance,
  and retention/promotion/removal decision for each candidate artifact.

## Evidence plan

### Primary sources

- Current testing, documentation-ownership, package-map, H4-8, R-023 and
  Qualification Evidence contracts.

### Experiment

After selecting a usable profile, run only its predeclared matrix on the named
environment. Capture raw observations and failures; inject at least one failure
from each selected class. Independently audit the closure inventory against the
current tree and inbound references before deletion.

### Failure scenarios

- A required host/tool/privilege is missing yet reported green.
- A changed build/profile reuses invalid evidence.
- A flaky retry hides failure or a soak has no defined load/oracle.
- Cleanup deletes the sole current fact or leaves retired code as a live path.

## Findings

- **Current-contract fact:** the Product Owner selected the Ubuntu Portable,
  TCP/TLS-only, Target-Link/loopback first-alpha directions, but no exact
  maintained H4-1–H4-3 build/topology implements that complete profile yet. A
  live or soak matrix cannot therefore name its candidate artifact, resource
  ceilings, workload, claim denominator, or valid failure oracle.
- **Inference:** starting a generic "final testing" suite now would create
  misleading green checks and stale process documents—the exact H4-8 closure
  failure this record is meant to prevent. The only present H4-8 work is to
  retain the matrix and closure-inventory shape as a future gate.

## Options

1. Small exact alpha matrix and inventory-led closure.
2. Broad undifferentiated “final testing” campaign.
3. Promote a demo without a qualification matrix.

## Recommendation

Preselect option 1 as the only research direction, but defer its exact matrix
until H4-1–H4-3 identify the first usable release profile. Options 2 and 3
conflict with current testing/claim discipline.

## Disposition

Open, with its missing profile precondition recorded. Promotion may create
H4-8A/B/D artifacts for one selected alpha profile; Public Beta remains
separately gated on real independent evidence.
