---
id: R-071
title: Which typed fact may carry an accepted root claim from Epoch verification to Namespace materialization?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-071 — Typed Epoch claim winner

## Decision this unlocks

G2-F023 and F028 require one owner to verify the threshold-authenticated claim
close and one later materialization step to consume its result.  The current
`ApplyOrderedClaim` accepts both a raw proof and a caller-built lifecycle `Op`,
so it re-verifies the close and lets a later control shape participate in a
root claim whose admission was consumed at Epoch input time.

## Current contract

R-042/ADR-0017 freeze commit in Epoch `E`, reveal in `E+1`, the
threshold-authenticated close, and the lowest eligible ordinal per Name.  The
local R-045 proof authenticates the commitment at input ingestion; it is not a
second current-state control proof.  ADR-0020 makes a threshold Namespace
materialization the sole current-state publisher.  R-067 permits replacement
of the internal claim/control representation, and R-070/ADR-0023 apply to
ordinary Authority-signed successors, not to a late root-claim Gateway path.

## Hypotheses

- **H1:** `ClaimOrder.Verify` creates an opaque `ClaimWinner`; materialization
  consumes that fact once, derives the root transition only from the winner,
  installed predecessor, materialization time, and Namespace policy, and never
  receives a raw proof or caller-built `Op`.
- **H2:** keep passing the raw proof and `Op` through every materializer but
  try to cache verification internally.
- **H0:** retain root claim as a normal Gateway control with a new admission
  proof at materialization time.

## Evaluation criteria

The choice must preserve R-042's exact signed close and per-Name `32`-claim
bound without inventing a total-epoch or product-scale limit; ensure a winner
cannot be substituted for a different Name, Authority, or ordinal; make the
single verification observable in focused tests; and keep incomplete, forked,
or rejected evidence from mutating a Lease.  It must not introduce a
Network-State import into Namespace or silently promote a submitted control to
current state.

## Findings

- **Inspection (2026-08-23):** `control.claim` first invokes
  `ClaimOrder.Verify`, then calls `ApplyOrderedClaim`, which invokes it again
  and compares its result to a public `Op.ClaimOrdinal`.
- **Inspection (2026-08-23):** R-042 assigns the relevant anonymous admission
  to the commitment input, while this later `Op` includes an unrelated
  ordering-proof control shape.
- **Inference:** an opaque value with private Name, Authority, ordinal, and
  close identity is the smallest proof-carrying boundary.  It lets the
  verifier reject bad Epoch evidence once and prevents materialization callers
  from choosing a different winner.

## Options

| Option | Fit | Disposition |
|---|---|---|
| H1 opaque winner | Preserves the accepted close while removing raw proof and `Op` from the post-verification boundary. | choose |
| H2 cache behind raw API | Leaves the duplicate lifecycle authority and makes cache lifetime a new security boundary. | reject |
| H0 late Gateway control | Charges/validates admission at the wrong lifecycle point and contradicts R-042. | reject |

## Recommendation

Accept H1 with high confidence.  `OpenClaimWinner` is the sole conversion of
an authenticated R-042 close into a materializable fact.  Its materialization
method derives first-claim versus reclaim generation, Name Authority, and the
policy lease without a caller-supplied claim ordinal, Name, Authority, or
lease deadline.  It returns an unsigned candidate only as the existing Record
signature flow requires; threshold `Store.Commit` remains the sole current
publisher.  The remaining signing/install composition is part of the sealed
Namespace Interface and M5 lifecycle work, not an assertion that this fact is
already current.

The strongest counterargument is that the opaque fact is process-local rather
than a new public Epoch wire object.  That is intentional for this tracer:
R-042's already signed close remains the interoperable proof, while its
verified result is an in-process Namespace boundary.  A public Epoch input
wire belongs only with a later selected Network/Namespace composition contract.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** No new ADR is needed: this refines the implementation ownership
of ADR-0017 without changing its protocol, trust root, record signature, or
resource bounds.  M5 must replace `ApplyOrderedClaim`, route the historical C4
bridge through the new single-verification boundary, and add substitution,
reclaim, and no-second-verification behavior tests.
