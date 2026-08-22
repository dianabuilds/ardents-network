---
id: R-068
title: How must a Target Record validity limit become durable and resolution-enforced?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-068 — Name Record validity migration

## Decision this unlocks

G2-F026 blocks M5: an authenticated `RecordNotAfter` is checked only at
control ingress, then discarded. The Namespace cannot prove or enforce a
Target binding's expiry after materialization or restart.

## Current contract

R-041 retains canonical Name V1. R-047 and ADR-0014 select ordinary Ed25519,
the `ardents-name-record-v1` transcript domain, and the signed-record
container. R-057 and ADR-0020 require threshold-authenticated current
materialization and compact proof verification. R-067 retains those profile
facts, while allowing an explicitly authorized compatibility migration for a
demonstrated security defect.

## Hypotheses

- **H1:** add signed `RecordNotAfter` in milliseconds to canonical Record V4;
  require every newly published or recovery-resumed Target to fit within its
  effective own/parent Lease. Decode existing V3 only as a migration input and
  make any V3 Target unavailable until it is replaced by V4.
- **H2:** retain V3 and derive the Target expiry from Lease alone.
- **H0:** change the Ed25519 suite, transcript domain, or signed container.

## Evaluation criteria

The result must bind expiry under the existing Authority signature, survive
durable reopen, make proof verification fail at the exact boundary, compose
with the minimum parent lifetime, reject expiry after Lease, and never make a
legacy Record more available than it was proved to be. It must preserve the
selected Ed25519 algorithm, transcript domain, and outer signed-container
framing.

## Evidence plan

### Primary sources

- R-041, R-047, R-057, R-067, ADR-0014, and ADR-0020, accessed 2026-08-23.
- G2-F026 and M5 in the Stage 8 workbook, inspected 2026-08-23.
- Current `record_wire`, `record_signature`, materialization, and control
  paths, inspected 2026-08-23.

### Experiment

Characterize Record V4 signature mutation, durable reopen, exact expiry,
expiry after own Lease, parent-minimum expiry, rotation, and recovery-resume.
Also install a valid V3 signed Record and confirm that it fails closed for a
Target lookup instead of inheriting Lease lifetime.

### Failure scenarios

- a Gateway validates expiry but a later proof ignores it;
- a parent or child Lease is shortened below a Target validity limit;
- a recovery successor revives an old Target without a new validity limit;
- a legacy V3 materialization remains resolvable indefinitely; and
- a format change silently changes the Ed25519 domain, network binding, or
  outer signed-container parsing.

## Findings

- **Inspection:** `RecordNotAfter` exists in canonical control JSON but is
  absent from `Record`, its signed bytes, materialization leaves, and `Verify`.
- **Inspection:** current proof expiry derives only from Lease and parent
  Lease; a valid Target can consequently outlive its authenticated control
  boundary.
- **Inference:** a signed canonical field plus the materialized leaf's
  minimum-expiry calculation repairs the complete persistence/proof path.
- **Inference:** treating V3 Target records as unavailable is the only safe
  migration without inventing an expiry that was never signed.

## Options

| Option | Fit | Disposition |
|---|---|---|
| H1: Record V4, fail-closed V3 Target migration | Makes the exact signed expiry durable and observable while retaining selected signature mechanics. | Accepted. |
| H2: Lease-derived expiry | Preserves the defect: the accepted Record lifetime is still absent. | Rejected. |
| H0: replace crypto/container | Expands the migration beyond the demonstrated defect. | Rejected. |

## Recommendation

Accept H1. Record V4 appends `RecordNotAfter` as an unsigned big-endian epoch
millisecond value to the canonical Record bytes and is therefore covered by the
existing `ardents-name-record-v1` Ed25519 transcript. New publish and
recovery-resume transitions must carry the same signed boundary and reject a
non-positive, elapsed, or Lease/parent-exceeding value. Record V3 remains
decodable solely for durable migration; because it has no signed Target expiry,
its non-empty Target is unavailable in any new materialization/proof.

Confidence is high: the migration adds the missing fact without changing the
selected cryptographic primitive or ownership boundary. The strongest
objection is short-term availability loss for V3 Target records; accepting them
would fabricate a security fact, so replacement by a V4 signed record is
required instead.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** ADR-0022 supersedes only the Record-version aspect of R-047 and
ADR-0014. M5 must implement the migration with signature, materialization,
proof, restart, and exact-boundary tests; no external observer is claimed.
