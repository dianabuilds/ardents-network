---
id: R-100
title: Usable-alpha qualification and repository closure
status: concluded
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-29
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

- **Current-contract fact:** the Product Owner selected and published exact
  immutable candidate `h4-alpha-1-rc-1`, source revision
  `70bf425eec937edcc22e8f0534db992aa2002a16`. H4-8A now binds artifact,
  platform, workload, Carrier, control inputs, topology, retained evidence, and
  claim limits to that identity.
- **Measurement:** A1-A10 are green, including immutable publication, the
  Product Owner's authenticated first-enrollment walkthrough, concrete H4-6A
  observations, the exact H4-3 workload/browser cells, and the same candidate
  on the selected TCP/TLS Carrier without fallback. The matrix retains failed
  attempts and their dispositions rather than hiding them behind successful
  reruns.
- **Measurement:** official A11 attempt 14 accepted all six cells and ten exact
  Go invocations in 2,462,217 ms. It bound immutable RC2 source/tag/archive,
  used the declared Windows/Ubuntu topology and 1 vCPU/1 GiB/128 PID remote
  envelope, retained every observer stream, and met the fixed 125-minute
  deadline. Evidence is retained outside Git at
  `C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-rc-2-h4-8-a11-attempt-14`.
- **Inference:** A11 and the finite A12 inventory now close the selected
  functional-alpha H4-3/H4-8 profile. They do not constitute capacity,
  availability, hostile-network, independent-operation, or Public Beta proof.

## Options

1. Small exact alpha matrix and inventory-led closure.
2. Broad undifferentiated “final testing” campaign.
3. Promote a demo without a qualification matrix.

## Recommendation

Continue option 1 using the exact H4-alpha-1 matrix. Do not convert A1-A10 into
a generic release label or infer A11 from elapsed wall-clock time. Options 2
and 3 remain rejected.

## Disposition

Concluded. Immutable `h4-alpha-1-rc-2` has green A1-A12 for the selected
functional-alpha profile. A12 retains the failed A11 attempt 13 and its two
harness defects (fixture transit ceiling and remote-evidence section parsing)
as recorded dispositions; it never overwrites prior evidence. RC2 release bytes
did not change, so no successor candidate or dependent release-byte gate was
required. Public Beta remains separately gated on real independent evidence.
