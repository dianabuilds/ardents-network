---
id: R-042
title: What authenticated ordering and inclusion proof makes a permissionless root claim first-valid without rewarding copied pending claims?
status: open
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-042 — Stage 6 claim ordering and inclusion

## Decision this unlocks

Select the mechanism and proof that let every honest verifier distinguish an
ordered losing claim from Conflict, partition, withholding, equivocation, or a
rule Fork. S6.5 cannot implement root-claim acceptance until this is decided.

## Current contract

R-003 and R-039 require first-valid, deterministic, permissionless root claims.
Local arrival time, wall clock, one resolver's view, project preference, and
manual dispute resolution are forbidden. A copied pending claim must not win
merely by changing bytes or obtaining more favorable propagation. R-029 supplies
authenticated Network Epoch identity and time confidence, but it does not prove
that an observer saw the complete eligible claim set for an epoch.

The earlier candidate `(network_id, epoch_number, SHA-256(claim))` only sorts
claims already known to a verifier. It neither proves inclusion/completeness nor
prevents digest grinding, copying, withholding, or different partitions from
ordering different sets. It is therefore not an accepted ordering mechanism.

## Hypotheses

- **H1:** a bounded commit/reveal protocol plus authenticated epoch inclusion
  can make copied claims ineligible and produce a globally verifiable loser.
- **H2:** an authenticated ordered log or quorum inclusion proof can establish
  one eligible set without a separate reveal phase.
- **H0:** no candidate fits the privacy, availability, governance, and modest
  hardware bounds; permissionless root claims must then be removed or reopened.

## Evaluation criteria

1. The proof authenticates the rule version, network, epoch, complete canonical
   claim, eligibility window, and inclusion position.
2. Copying an observed pending claim or grinding irrelevant bytes cannot improve
   priority.
3. Withholding, partition, rollback, and equivocation produce explicit
   `conflict`, `partition`, `unavailable`, or `fork` outcomes with no Lease
   mutation.
4. Different honest verifiers with the same authenticated evidence reach the
   same result without local arrival time.
5. The mechanism introduces no hidden registrar, coordinator, payment system,
   stable User identity, or unmeasured global infrastructure.
6. Claim latency, retained bytes, verification work, and recovery behavior fit
   the accepted R-023 reference-host budget.

## Evidence plan

### Primary sources

- R-003 and R-039, accessed 2026-08-20 — product claim and failure contract.
- R-005 and R-029, accessed 2026-08-20 — Time Confidence and authenticated
  Network Epoch boundary.
- R-002, accessed 2026-08-20 — Connection Result taxonomy.
- Candidate protocol specifications and maintained source code must be added
  before comparing an ordered log, commit/reveal, or another mechanism.

### Experiment

Build a disposable simulator under `experiments/r-042-claim-ordering/`. Run the
same signed claims through permutations of observation copying, reveal
withholding, flooding, partition, rollback, equivocation, and rule fork. Retain
the exact eligible-set proof and verify it independently. Define latency,
storage, verification-work, and false-accept thresholds before running it.

### Failure scenarios

- A copied or digest-ground claim outranks the original commitment.
- Two honest partitions accept different controllers for one complete name.
- A verifier names a loser without proving the eligible claim set.
- Missing reveal or inclusion evidence mutates a Lease.
- Rollback or a different rule version is classified as an ordered collision.
- One coordinator becomes the unrecorded source of ordering truth.

## Findings

- **Sourced fact:** R-003 leaves commitment, reveal, and shared-state mechanism
  selection open and explicitly requires front-running analysis.
- **Sourced fact:** R-029 authenticates an epoch but does not attest that every
  eligible claim was included or observed.
- **Inference:** hashing a claim is a deterministic tie-break only after an
  authenticated eligible set exists.
- **Assumption:** a bounded multi-step claim flow may be acceptable if its cost
  and recovery behavior are measured against the Developer journey.

## Options

1. **Epoch plus claim digest only.** Rejected: sortable but grindable and has no
   inclusion/completeness proof.
2. **Commit/reveal plus authenticated inclusion.** Candidate: can bind priority
   before payload disclosure, but adds latency, withholding, cleanup, and state.
3. **Authenticated ordered log or quorum proof.** Candidate: may produce one
   eligible order, but adds governance, availability, and fork dependencies.
4. **No permissionless root claim in V1.** Fallback if no candidate meets the
   accepted contract.

## Recommendation

Run the named experiment before choosing. Confidence is high that the previous
digest-only option is insufficient and low that any replacement fits without a
meaningful governance or availability cost. The strongest counterargument is
that commit/reveal may still reward denial and add too much latency.

## Disposition

- State: `open`; the former Option A is withdrawn.
- S6.5 claim ordering and every evidence predicate that depends on it remain
  blocked.
- R-041 canonical encoding may be used as experiment input; no production claim
  mechanism or Lease mutation is authorized.
- A decision must include the promised eight-scenario coverage map and exact
  independent proof schema.
