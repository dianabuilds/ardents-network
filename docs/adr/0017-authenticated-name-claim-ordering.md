---
status: accepted
date: 2026-08-20
---

# ADR-0017 — Order root-name claims through authenticated epoch input

## Context

Permissionless root claims need one verifiable winner without local arrival
time, digest grinding, or a hidden registrar. A digest can order only a locally
known set and cannot prove inclusion, completeness, withholding, or a rule
fork. R-042 measured a bounded commit/reveal candidate against copying,
withholding, rollback, equivocation, and incompatible rules.

## Decision

Accept R-042 O1b. A claim commits during Network Epoch `E` and reveals during
`E+1`. The accepted threshold-authenticated epoch close commits the ordered
input log, accepted materializations, and deterministic rejections. For one
Name, the lowest eligible commitment input ordinal wins; reveal arrival and
claim digest never choose priority. At most 32 eligible claims for one Name are
materialized in the bounded V1 proof.

Incomplete evidence or withholding is `unavailable` and mutates no Lease. Two
authenticated roots or an incompatible authenticated rule is `fork`. The
ordinary proof verifies the threshold-published materialization; the Stage 6
verifier additionally recomputes it from the complete bounded input and
rejection corpora.

## Consequences

- S6.5 may implement `ClaimOrder.Verify` and the R-042 hostile matrix.
- Claim latency spans two epochs and depends on Network Epoch availability.
- A captured epoch threshold can censor or fork the Namespace. The design makes
  this visible and fail-closed; it does not remove governance capture.
- This decision reuses ADR-0004 and selects no second registrar, consensus
  system, storage engine, or public wire protocol.

## Compliance

- [R-042](../research/records/r-042-claim-ordering.md) freezes fields, bounds,
  measurements, and the eight-scenario map.
- [ADR-0004](0004-authenticated-epochs-and-separated-control-roots.md) owns the
  Network Epoch trust root and captured-threshold limitation.
- [R-055](../research/records/r-055-stage-6-evidence-serialization.md) owns
  canonical development-artifact encoding.
