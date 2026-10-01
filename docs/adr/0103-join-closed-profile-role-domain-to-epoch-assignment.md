---
status: accepted
date: 2026-09-26
---

# ADR-0103 — Join the closed-profile Role Domain to the authenticated Epoch assignment

## Context

The signed closed profile v3 (`ARDCPR03`) carries a numeric Role Domain per
Node entry — Initiator=1, Rendezvous=2, Responder=3, Introduction=4 — and the
protected-route contract requires each entry's identity, digest, assignment
and duty to match the current Node Record. The accepted Epoch determines each
record's assignment: `assignedDomain(epoch, record.family)` selects one role
domain name per family through the epoch assignment digest, and State
materializes that name as the record's `Snapshot.Assignment`.

Before this decision, `matchesClosedProfileCandidates` joined only NodeID,
record generation, record digest and closed Carrier eligibility. The
profile's numeric Role Domain was structurally validated
(`validClosedProfileDuty`) but never compared with the Epoch assignment: a
correctly signed profile could name a domain the record's family was never
assigned, and the State owner would durably accept and serve it (F-45).
Node's `closedRouteReceiver` checks
`ClosedPurposePermitsDuty(purpose, RoleDomain, Subrole)` against that view,
so the mismatched domain would have driven listeners as authenticated duty.

## Decision

1. **Contract-fixed mapping.** The numeric domains join to the Epoch
   assignment names as initiator=1, rendezvous=2, responder=3,
   introduction=4. The `transit-issuance` assignment has no closed-v3
   domain; a closed issuer is the Rendezvous-domain node at subrole 6.
2. **State owner enforces the join.** For every profile entry with a NodeID
   equal to an accepted record, State derives
   `assignedDomain(epoch, record.family)` from the verified Epoch and maps it
   through the fixed contract mapping. An unknown assignment or a domain
   unequal to the entry's numeric Role Domain refuses the profile. The same
   join runs at durable acceptance (`AcceptClosedProfile`) and at every
   read-back (`CurrentClosedProfile`/`CurrentClosedRoute`), so a profile
   accepted under one Epoch cannot survive a successor as a usable route.
3. **Node consumes the joined view.** The Node side holds no second mapping;
   `closedRouteReceiver` keeps selecting the entry by NodeID from the
   State-provided view.
4. **Signed bytes are unchanged.** The profile grammar, signing domain and
   numeric domain values are untouched. Whether an Epoch may carry
   additional unused domains remains a separate question (the F-50 closed
   Epoch-envelope rule).

## Consequences

- Refusal evidence: the installed provisioning E2E adds a full-Epoch,
  valid-signature mismatch case proving refusal before listener start, and a
  synthetic State join test proves refusal and acceptance on the read-back
  path. Test-topology families are hash-searched so each Node's family lands
  in the domain its topology role requires — the same search a real closed
  operator performs, because the assignment is hash-determined.
- Fixtures that used arbitrary non-role domain names migrate to the
  canonical role names. The issuer's materialized assignment becomes
  `rendezvous`, so its duty carrier selection and local retention class
  follow the route-rendezvous contract; that is the contract-correct outcome
  previously masked by the unjoined fixture domains.
- F-07 (the State-to-Node checked duty value) is unblocked: the role join
  its field freeze awaited is settled.
