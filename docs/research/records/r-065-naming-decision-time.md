---
id: R-065
title: Which decision-time rule makes signed Name recovery and lifecycle freshness replay-safe?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-065 — Naming decision time

## Decision this unlocks

DA-03 and M5 need one interpretation for Name lifecycle time. The current
Gateway validates control proof freshness at millisecond resolution, while the
Lease implementation accepts seconds and reconstructs a millisecond value by
multiplication. A valid recovery proof can therefore be rejected whenever the
Gateway clock is not exactly at the start of a second.

## Current contract

R-039 requires bounded, fail-closed Recovery Pending with no administrative
fallback. R-044 fixes a Recovery Authorization's signed `StartedAt` and
`CompletesAt` boundaries in epoch milliseconds, and R-045 bounds admission
proof lifetime to at most thirty seconds. Existing Record bytes already encode
Lease expiry/grace fields in epoch seconds and Recovery/Policy boundaries in
epoch milliseconds. R-047/ADR-0014 remain authoritative for the signed record
and control transcript bytes; this record selects their time interpretation,
not a new format or cryptographic suite.

## Hypotheses

- **H1:** one Gateway-owned decision `time.Time` is sampled for each accepted
  control operation. Lease expiry/grace comparisons use its Unix seconds;
  signed Record, Policy, and Recovery boundaries use its Unix milliseconds.
  A recovery initiation stores the verified signed start boundary, provided it
  is no later than decision time and is within the already accepted finite
  admission lifetime. Completion compares the same millisecond decision time
  to the signed completion boundary.
- **H2:** initiation must equal the Gateway's decision time to the millisecond.
- **H0:** the existing mixed representation cannot preserve the accepted
  recovery contract without a new signed-record/wire format.

## Evaluation criteria

The selected rule must preserve exact signature bindings, prevent a future
start/completion from being accepted early, reject a stale initiation outside
the finite admission lifetime, keep durable Recovery Pending boundaries
unchanged across restart, and never extend a Recovery Policy or Lease because
of wall-clock rollback. It must not change canonical record or transcript
bytes, introduce a clock/network dependency, or make current Name Authority
an alternative recovery authority.

## Evidence plan

### Primary sources

- R-039, R-044, R-045, and R-047, accessed 2026-08-23.
- `internal/nameauthority/control.go`, `control_operation.go`,
  `internal/namelease/{lease,recovery,lifecycle}.go`, and
  `internal/namerecovery/{contract,authorize,transcript}.go`, inspected
  2026-08-23.
- Stage 8 G2 naming delta review F027, accessed 2026-08-23.

### Experiment

Characterize a valid threshold recovery initiation at a non-zero millisecond
offset, a start in the future, a start exactly at and just beyond the accepted
admission lifetime, cancellation before/at completion, completion before/at
the signed boundary, restart-derived Recovery Pending, and a backward wall
clock observation. The result must preserve the same signed start/completion
bytes and produce no Lease/Policy mutation on rejection.

### Failure scenarios

- Gateway and signer clocks differ within the accepted control lifetime;
- replay of an old valid initiation, cancel, or completion;
- future-start or premature-completion proof;
- backward or forward wall-clock step at restart;
- seconds/milliseconds conversion changing a signed boundary; and
- a recovery transition modifying unrelated Lease, policy, authority, or
  current Target facts.

## Findings

- **Sourced fact:** R-044 signs recovery boundaries and policy delays in
  milliseconds; R-045 bounds admission lifetime to at most thirty seconds.
- **Inspection:** the Gateway samples `UnixMilli` for control admission and
  canonical recovery validation, then passes `Unix` to Lease. Lease compares
  an authorization start to `now * 1,000`; the equality is impossible for a
  valid non-zero millisecond offset.
- **Inspection:** durable Record fields intentionally already distinguish
  second-granularity Lease expiry/grace from millisecond-granularity recovery
  and policy boundaries.
- **Inference:** H1 preserves every signed byte and makes the existing
  two-granularity representation explicit. H2 converts ordinary scheduling
  jitter into denial of a valid recovery; H0 is not needed unless a new
  characterization shows H1 cannot bound replay with the accepted admission
  lifetime.

## Options

| Option | Fit | Consequence | Disposition |
|---|---|---|---|
| H1: typed decision time | Preserves signed recovery, bounded replay, and existing Record units. | M5 gives Name lifecycle one decision-time input and tests all boundaries; no wire migration. | Accepted. |
| H2: exact Gateway equality | Has a simple implementation. | Rejects valid signed proofs for ordinary sub-second skew and is not required by R-044. | Rejected. |
| H0: new format | Could use one unit everywhere. | Selects a format migration without evidence that existing encoded units are unsafe. | Fallback. |

## Recommendation

Accept H1. `nameauthority` owns one decision time per control transition;
`namelease` receives explicit seconds and milliseconds derived from that one
value rather than inferring milliseconds by arithmetic. A verified initiation
uses its signed start as the durable recovery boundary only when it is within
the accepted finite admission lifetime; completion and cancellation use the
same decision milliseconds against the recorded signed boundary. This is a
semantic repair, not a protocol or format change.

Confidence is high that H1 resolves the demonstrated unit conflict without
weakening freshness. The strongest objection is that a fixed admission window
can be too short for a distributed authority path; widening it would be a new
resource/replay decision and is deliberately not made here.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** DA-03 is closed for the existing naming profile. No ADR is
required because canonical bytes, cryptography, and field units are retained;
any change to them reopens DA-07 and requires the applicable authority.
