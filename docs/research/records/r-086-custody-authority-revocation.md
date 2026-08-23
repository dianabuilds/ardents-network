---
id: R-086
title: How can Authority Custody observe Name Authority replacement safely?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-086 — Custody-aware Name Authority revocation

## Decision this unlocks

Decide whether an active Authority Vault record can be durably demoted when an
R-044 threshold recovery replaces its Name Authority, and if so which opaque
Namespace proof and D08 format migration are required.

## Current contract

ADR-0021 fixes D08 to opaque environment, network, root, and Authority
commitments and forbids plaintext Name state. R-044/ADR-0018 make a completed
threshold recovery replace the effective Namespace Authority. The current
control signer rejects the former key for that recovered Record, but a Vault
cannot select or authenticate the replacement because it does not know a Name.
Broker Grant revocation is a separate local-admission authority.

## Hypotheses

- **H1:** an opaque, current Namespace replacement witness can bind the old
  Authority commitment to its successor and permit fail-closed local demotion.
- **H2:** Namespace-level rejection is sufficient for the selected custody
  scope; active Vault demotion would add an unjustified format/protocol surface.
- **H0:** neither design preserves D08 privacy, authority separation, and
  monotonic recovery requirements.

## Evaluation criteria

The selected outcome must reject stale, forked, wrong-network, wrong-root,
wrong-Authority, replayed, and rollback witnesses; never expose a plaintext
Name or root; preserve active/quarantine exclusivity and floors; and define
old/new reader, writer, rollback, and export/import behavior before changing
D08.

## Evidence plan

### Primary sources

- ADR-0021 and the Stage 7 Authority Custody specification, accessed
  2026-08-23.
- R-044/ADR-0018 and the current Namespace recovery/record implementation,
  accessed 2026-08-23.

### Experiment

Freeze current, stale, equal, wrong-binding, forked, and interrupted demotion
vectors. If a candidate needs new bytes, independently decode old/new Vaults
and prove that no old reader silently activates a demoted record.

### Failure scenarios

An old Vault signs after a valid replacement; a forged/forked witness demotes
a live Authority; a recovery leaks Name text; a rollback reactivates a demoted
record; or a local Grant revocation is mistaken for a Name Authority transition.

## Findings

- **Sourced fact:** R-044 binds a threshold recovery successor and its delay.
- **Measurement:** the maintained Namespace control rejects the former key
  after a completed recovery Record replaces it.
- **Inference:** D08 lacks the Name-scoped linkage needed for a Vault to locate
  that event, so a local demotion operation cannot be safely invented.
- **Inspection:** the implemented `NameAuthorityReconciliation` authenticates
  that one Authority key is current and strictly newer for recovered-Bundle
  activation. It does not commit an old Authority key to a different successor;
  extending it for active-Vault demotion would add the prohibited new D08 and
  Namespace format surface.

## Options

1. Add an opaque replacement witness plus an explicitly migrated D08 binding.
2. Retain Namespace-level effective revocation only and document the local
   stale-signature limitation.
3. Add an owner-local kill switch, which is not proof of network revocation.

## Recommendation

Evaluate option 1 against option 2 before changing D08 or adding a Vault
operation. Confidence is high that a generic current-state claim is unsafe;
the strongest counterargument is that an additional witness may make ordinary
custody operation too complex for the one-to-one maintained product.

## Disposition

**Accepted H2, 2026-08-23, under the Product Owner's standing Stage 8
delegation.** Namespace-level rejection is the effective Name Authority
revocation for the selected custody scope. An active local Vault remains
locally usable until an operation reaches Namespace, where the former key is
rejected; it is not evidence that the former Authority remains effective.

No local kill switch, D08 field, Namespace wire, or Vault demotion operation is
added. A future requirement for preflight local demotion must introduce a
Name-scoped, predecessor-to-successor opaque proof; it requires a new format
decision, old/new reader and migration analysis, and the adversarial vectors
listed above. Broker Grant revocation remains a separate local transition.
