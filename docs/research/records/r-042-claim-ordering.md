---
id: R-042
title: What authenticated order key, proof structure, and classification contract freeze S6.0 deterministic claim ordering and ordered-collision proof?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-19
---

# R-042 — Stage 6 claim ordering and classification

## Decision this unlocks

Freeze the authenticated order key, the claim proof structure, the
five-state classification contract, and the coverage map that the
S6.5 (concurrency/fork/abuse) implementation will use to name a
deterministic loser in an ordered collision and to fail-closed on every
other unresolved state. Without this freeze, S6.5 would fall back to
in-memory race timing or local best effort, which the brief explicitly
rejects, and the verifier could not test the ordering against an
external time source.

## Current contract

R-039 § Fixed product contract fixes:

- root canonical names use first-valid deterministic permissionless
  claims that create renewable time-bounded Name Leases for Name
  Authority;
- a parent may issue names only inside its subtree;
- bounded Anonymous Cost protects naming work without money, a global
  account, identity document, IP reputation, stable identity, wallet,
  token, or registrar and makes no personhood or fairness claim.

R-005 fixes Time Confidence and the loss boundary. R-029 freezes the
authenticated Network Epoch mechanism that the order key reuses.
R-002 fixes the Connection Result taxonomy used for `claim-loses-
ordered`, `state-conflict`, `fork-unresolved`, and `state-unavailable`
classifications. R-046 freezes the role set the order proof must
reference. R-041 freezes the canonical encoding the order proof consumes.

What remains open before S6.5 can start is the exact order key, the
proof structure, the five-state classification contract, and the
mapping from the eight brief scenarios to those five states.

## Hypotheses

- **H1:** a single global order key
  `(network_id, epoch_number, claim_digest)` with `claim_digest =
  SHA-256(canonical claim payload)` is sufficient to name the loser in
  any ordered collision and is globally verifiable without new
  infrastructure.
- **H2:** a monotonic clock from a single coordinator is also
  sufficient.
- **H0:** no global order key can be constructed without weakening the
  R-029 Epoch contract or introducing a new infrastructure root.

## Evaluation criteria

1. **Global determinism:** the order key is independent of any one
   observer's local clock or arrival time.
2. **Authenticated source:** the order key reuses an already accepted,
   audited mechanism (R-029 Network Epoch) and does not invent a new
   trust root.
3. **Collision resistance:** two distinct valid claim payloads cannot
   share the same order key.
4. **Forward monotonicity:** `epoch_number` increases monotonically;
   rollbacks do not produce a valid earlier order key.
5. **Five-state contract:** every ordering outcome is one of
   `ordered`, `conflict`, `fork`, `unavailable`, `partition`, each with
   an explicit Connection Result and an explicit Lease-mutation rule.
6. **Coverage:** all eight brief scenarios
   (observation copying, front-running, withholding, flooding,
   partition, rollback, equivocation, rule fork) map cleanly into the
   five states.
7. **No identity coupling:** the order key and proof do not require
   accounts, IP reputation, KYC, or stable cross-context identity.

## Evidence plan

Primary sources, accessed 2026-08-19:

- R-039 — H3 private naming lifecycle (accepted 2026-08-17).
- `horizon-3-stage-6-brief.md` S6.5.
- `stage-6-readiness-checklist.md` §B.2.
- R-029 — authenticated Node lifecycle (Network Epoch).
- R-005 — hostile bootstrap and Time Confidence.
- R-002 — Service Connection Connection Result taxonomy.
- R-041 — canonical name limits and `schema_version`.
- R-046 — role matrix.

The order key and classifier are implemented in S6.5 against this
contract; no new experiment is required for R-042.

## Failure scenarios

- The loser is named without a `claim_digest` over
  `network_id + epoch + payload`.
- Two distinct valid claim payloads produce the same `claim_digest`
  (collision resistance broken).
- The `conflict` state mutates a Lease.
- The `fork` state returns a successful resolution.
- The `unavailable` state (no authenticated Epoch) issues a Lease.
- A front-runner wins despite the original claimant having a smaller
  order key.
- A rollback returns a previous epoch as valid without monotonicity
  check.
- Equivocation (same identity, different `network_id`) is not classified
  as `fork`.
- The order key uses wall clock or local arrival time.

## Options and recommendation

1. **Option A — `(network_id, epoch_number, claim_digest)` with
   `claim_digest = SHA-256(canonical claim payload)` (recommended).**
   Reuses the R-029 Network Epoch as the authenticated time source and
   collision-resistant hash as the tie-break. Globally deterministic
   and forward-monotonic. No new trust root.
2. **Option B — `(network_id, monotonic_clock, claim_digest)`.** A
   network-coordinator monotonic clock replaces the Epoch. Rejected:
   requires a new trust root that does not exist in the H3 contract
   and diverges from R-029; observers in different partitions can
   disagree.
3. **Option C — local arrival time.** Rejected: explicitly forbidden by
   the brief ("never select a canonical branch from in-memory race
   timing or local best effort").

Recommendation: **Option A**, accepted by the Product Owner on
2026-08-19.

## Disposition

- R-042 becomes `decided`. The open row in `docs/research/questions.md`
  is updated to point at this record and the frozen contract.
- §B.2 of `stage-6-readiness-checklist.md` is checked.
- S6.5 (concurrency/fork/abuse) may implement the order-key check, the
  five-state classifier, and the coverage map. The verifier may test
  every state transition against the manifest.
- This freeze does not authorize code; the Stage 6 coding gate remains
  closed until §B.3 through §B.5 of the readiness checklist are also
  checked and the corrected brief, plan, and evidence contract are
  accepted.
- No ADR is required: this is a contract freeze that reuses the
  already-decided R-029 Epoch mechanism.
