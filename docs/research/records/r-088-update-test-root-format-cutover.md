---
id: R-088
title: Which format cutover is valid for an Update root with no maintained bootstrap owner?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-088 — Update test-root format cutover

## Decision this unlocks

Retire the bounded tracer's V1 `EvidenceNotice` field and V0 command without
inventing an owner for an Update-root selector or leaving a legacy runtime
reader.

## Current contract

R-064 retains transaction/recovery behavior only as an H3 technical tracer;
it explicitly does not select bootstrap, installation, or activation support.
R-087 selected a C1 root conversion, but a source audit finds no maintained
initializer, parent pointer, or caller that can atomically select a sibling
root. `internal/update` rejects uninitialized roots and preserves an existing
lock; `cmd/ardents-release` only accepts `-update-root`; all root creation is
in test fixtures.

## Hypotheses

- **H1:** perform a C0 hard cut of the unobservable V1 fixture format to V2,
  retain exact V0 vectors only in a C4 verifier, and delete the command.
- **H2:** add a C1 selector/bootstrap so the V1 root can be converted.
- **H0:** retain V1 reading/writing indefinitely.

## Evaluation criteria

The selected path must preserve the bounded transaction's authorization,
journal, rollback, recovery, and fault evidence; reject V1 in maintained
runtime; avoid a new installer, root manager, Custody writer, or external
compatibility promise; and keep historical V0 reproduction out of product
imports.

## Evidence plan

### Primary sources

- R-064, R-087, ADR-0015, and ADR-0029, accessed 2026-08-23.
- `internal/update`, `cmd/ardents-release`, and their callers, inspected
  2026-08-23.

### Experiment

Run the retained checkpoint, interruption, recovery, idempotence, rollback,
and residue matrices against a V2 fixture. Independently decode the frozen V0
command/result/manifest vectors in a C4-only test package and prove that no
maintained runtime import reaches it.

### Failure scenarios

A V1 reader remains reachable at runtime; C0 drops a transaction invariant;
an invented selector makes a bootstrap claim; a V0 evidence string reaches a
V2 result; or the C4 verifier becomes a runtime dependency.

## Findings

- **Inspection:** no non-test source creates `.ardents-update-transaction-v1`,
  a root directory, or a root selector; the only command takes an existing
  root path.
- **Inspection:** `internal/update` deliberately cannot create, replace, or
  repair its permanent lock, so it cannot honestly become the missing parent
  root owner.
- **Inference:** C1 has no owner in this product scope. Adding one would
  select a bootstrap/installer surface excluded by R-064.

## Options

| Option | Disposition |
|---|---|
| H1: C0 V2 fixture cutover plus C4 V0 verifier | Choose. The representation has no maintained runtime observer; retained transaction evidence remains testable. |
| H2: introduce C1 root selector | Reject. It invents an excluded bootstrap owner merely to migrate test fixtures. |
| H0: retain V1 runtime | Reject. It preserves a retired evidence representation without an observer. |

## Recommendation

Choose H1 with high confidence. The strongest counterargument is a private
unrecorded root, but Stage 8's observer audit cannot create a support promise
for it. A future selected bootstrap may define its own durable-root migration
under a new decision.

## Disposition

Accepted under the Product Owner's standing Stage 8 delegation. ADR-0030
supersedes ADR-0029. M13 replaces the V1 fixtures/results with V2, creates the
C4 verifier, and deletes `cmd/ardents-release`; it adds no lifecycle surface.
