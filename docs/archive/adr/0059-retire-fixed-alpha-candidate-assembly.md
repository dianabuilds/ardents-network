---
status: accepted
date: 2026-08-30
supersedes: ADR-0052
---

# ADR-0059 — Retire fixed historical candidate assembly

## Context

ADR-0052 deliberately implemented only the exact RC1 and RC2 static-input
ceremonies. Both operations have been exercised, their immutable artifacts and
receipts are retained, and neither operation can construct a new product
candidate. Keeping their H4-bound generator and terminal routes in maintained
code would make a completed planning-stage ceremony look like a current
release capability. It would also invite reuse of a custody record that is not
accepted as authority for a future release.

H4 is project-planning taxonomy, not product or protocol identity. Historical
evidence may retain its exact strings, but maintained runtime interfaces must
not depend on those strings.

## Decision

Retire `BuildAlphaInputs`, `BuildAlphaSuccessor`, their fixed request and static
directory implementation, and the `ardents-release-custody assemble` and
`assemble-successor` routes. The command keeps only `initialize` and `inspect`:
they create or authenticate the bounded encrypted seed record and expose only
its public receipt. They do not sign, assemble, publish, upload, or execute a
release.

The RC1/RC2 release artifacts, receipts, research records, exact identifiers,
and ADR-0052 remain immutable provenance. They are not regenerated or silently
renamed. A future candidate assembly operation requires a new decision and
must be bound to a product release contract rather than a delivery-horizon or
epic label.

## Consequences

- Current code no longer contains an executable route that can recreate the
  historical H4 candidate inputs.
- The local custody record remains inspectable without becoming a generic
  signer or key-export interface.
- Historical verification remains possible from the retained artifacts and
  records; source history preserves the retired implementation.
- No existing seed record is accepted automatically for a future release.

## Compliance

- [ADR-0015](0015-separate-release-decision-from-local-activation.md)
- [ADR-0050](0050-separate-local-release-seed-custody.md)
- [ADR-0052](0052-build-fixed-alpha-static-inputs.md)
- [R-119](../research/records/r-119-closed-alpha-release-signing-operation.md)
- [R-120](../research/records/r-120-bounded-alpha-input-signing.md)

