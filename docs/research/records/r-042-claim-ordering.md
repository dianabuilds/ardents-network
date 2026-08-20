---
id: R-042
title: What authenticated ordering and inclusion proof makes a permissionless root claim first-valid without rewarding copied pending claims?
status: decided
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
- [RFC 9162](https://www.rfc-editor.org/rfc/rfc9162.html), accessed
  2026-08-20 — domain-separated Merkle trees, signed tree heads, inclusion
  proofs, and consistency proofs. These prove membership and append-only
  consistency; they do not by themselves prove that an operator accepted every
  eligible submission.
- R-029, accessed 2026-08-20 — the already accepted threshold-authenticated
  input-log/View/rejection model and its explicit captured-threshold,
  withholding, auditor, and fork limitations.

### Experiment

Build a disposable simulator under `experiments/r-042-claim-ordering/`. Run the
same signed claims through permutations of observation copying, reveal
withholding, flooding, partition, rollback, equivocation, and rule fork. Retain
the exact eligible-set proof and verify it independently. Define latency,
storage, verification-work, and false-accept thresholds before running it.

The frozen experiment caps one claim set at `64` commitments, its logical proof
encoding at `64 KiB`, and one verification at `10 ms` p95 on the declared weaker
`1 vCPU/512 MiB` Linux container. Every hostile scenario must have zero false
accepts, and all reveal-order permutations must return the same winner and loser
ordinals. These are experiment gates, not accepted production capacity.

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

## Decision-ready candidate O1b — epoch commit/reveal

The Product Owner accepted this candidate on 2026-08-20. It deliberately reuses
the existing Network Epoch trust root instead of inventing a second registrar
or consensus mechanism.

O1 used a `64`-claim per-Name conflict cap and failed its predeclared weaker-host
p95 verification gate. O1b changes only that per-Name cap to `32`; it does not
cap the complete epoch input log or total distinct names.

1. During commit epoch `E`, a claimant submits a canonical commitment over the
   network, rule version, canonical Name wire bytes, proposed Name Authority,
   and a fresh `32-byte` secret. The published commitment does not disclose the
   Name. A Node-local R-045 admission proof binds this commitment as its
   operation digest and accompanies the submission; it is not an ordering key.
2. The threshold-authenticated close of `E` commits the ordered input-log root,
   cutoff, accepted commitment root/length, and deterministic rejection
   root/length. Every input has exactly one accepted or rejected outcome.
3. During reveal epoch `E+1`, the claimant reveals the exact Name, secret, and
   signed claim. A reveal is eligible only when it opens one accepted `E`
   commitment and authenticates the same proposed Name Authority.
4. Claims for one Name are ordered only by their accepted commitment input
   ordinal. The lowest eligible ordinal is accepted; later eligible ordinals
   are deterministic `ordered-collision` losers. Reveal arrival order and claim
   digest never choose the winner.
5. A copied commitment cannot be opened without the secret and Authority key.
   A commitment made before observing the reveal is an independent competing
   claim and remains subject to admission cost and the authenticated ordinal.
6. Missing reveal, missing full-set evidence, withholding, stale cutoff, or an
   invalid proof mutates no Lease. Two threshold-authenticated roots for the
   same network/rule/epoch are `fork`; incomplete evidence is `unavailable`.

The proof presented to an ordinary verifier contains the exact Network Epoch,
commitment and reveal, their input ordinals, accepted/rejected materializations,
and Merkle paths. The independent S6.6 verifier receives the complete bounded
input corpora and recomputes both trees, every rejection, every reveal opening,
and the per-Name winner. This preserves the accepted distinction between local
materialization verification and global completeness auditing.

### Frozen proof fields

The threshold-authenticated epoch close contains exactly: `network[32]`,
`epoch uint64`, the ASCII rule identifier, monotonic cutoff offset, input-log
root and length, accepted-materialization root and length, rejection root and
length, and sorted distinct epoch-authority signatures. One committed input
contains `ordinal uint32`, `commitment[32]`, and the digest of its locally
accepted R-045 transcript. One rejection contains the same ordinal and
commitment plus a closed reason code. A reveal contains canonical Name wire
bytes, `secret[32]`, proposed Authority `key[32]`, commitment ordinal, and one
Ed25519 signature by that proposed Authority.

The ordinary proof contains the epoch close, one Name's at-most-`32` reveals in
strict commitment-ordinal order, the materialization inclusion path, and its
winner/loser ordinals. The development verifier additionally receives the
complete bounded epoch input and rejection corpora and recomputes every root,
admission binding, reveal opening, eligibility decision, and materialization.
Proof verification rejects duplicate or decreasing ordinals as non-canonical;
it never sorts or otherwise normalizes caller-supplied reveal order.
An ordinary proof can verify what the threshold published; only the complete
corpus can test whether the threshold followed the accepted rule. Canonical
artifact encoding is owned by S6E1 rather than this ordering record.

### Eight-scenario coverage map

| Scenario | Required outcome |
|---|---|
| two independent valid commitments | lowest eligible input ordinal wins; later ordinal is `ordered-collision` |
| copied commitment or copied reveal | cannot authenticate/open as a distinct eligible claim |
| reveal withholding | no mutation for the missing reveal; remaining complete set is deterministic |
| input withholding or partition | `unavailable`/`conflict`; no deterministic loser is named |
| more than 32 claims for one Name | bounded `unavailable`; no Lease mutation |
| prior-epoch rollback | `unavailable`; no old winner revival |
| two authenticated roots for one epoch/rule | `fork` |
| authenticated incompatible rule version | `fork` |

The honest limitation is explicit: a captured Network Epoch threshold can
censor inputs or fork the entire Namespace log. O1b makes that behavior
inspectable and fail-closed; it does not eliminate governance capture or prove
that a submission reached an honest signer.

## Measurements

- **Measurement:** the deterministic hostile matrix passed ten consecutive test
  runs with zero false accepts. Copied reveal, incomplete set, cap overflow,
  mixed names, duplicate signer, rollback, equivocation, and incompatible rule
  version all produced their predeclared outcomes; reveal permutations selected
  the same ordinal.
- **Measurement:** on the Windows `amd64` endpoint with Go `1.26.6`, O1 verified
  `64` claims in `3.082100 ms` p95 over `1,000` iterations and retained `11,468`
  logical proof bytes.
- **Measurement:** in the pinned Ubuntu image, offline/read-only, capped to
  `1 vCPU`, `512 MiB`, `64` PIDs, no capabilities, and no network, O1 took
  `13.842151 ms` p95 and failed the frozen `10 ms` gate.
- **Measurement:** under the identical Linux constraints, O1b verified `32`
  claims in `1.640826 ms` p95 (`1.115748 ms` p50, `9.941401 ms` maximum) over
  `1,000` iterations and retained `5,932` logical proof bytes. O1b passed.
- **Inference:** the threshold-authenticated set and commit/reveal semantics are
  viable for the bounded Stage 6 tracer at a `32`-claim per-Name cap. This does
  not measure distributed log availability, signer independence, or censorship.

## Recommendation

Choose O1b. Confidence is high that the previous
digest-only option is insufficient and moderate that O1b meets the bounded V1
semantic contract. The strongest counterargument is that the Network Epoch
threshold becomes the visible claim-log inclusion authority and can still
censor or fork the Namespace.

## Disposition

- State: `decided`; the Product Owner accepted O1b and ADR-0017 on 2026-08-20.
  The former digest-only option and measured O1/64 profile are rejected.
- S6.5 implementation is authorized through `ClaimOrder.Verify`; no local
  arrival, digest-priority, registrar, or silent branch selection is permitted.
- The frozen field inventory and eight-scenario map are S6.5/S6.6 acceptance
  tests.
