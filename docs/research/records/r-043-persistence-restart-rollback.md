---
id: R-043
title: Which exact persistence, restart, rollback, and cache-proof ownership boundary lets Stage 6 Name Lease/Generation/Record state survive restart, fail closed on tamper, and remain replay-bounded?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-19
---

# R-043 — Stage 6 persistence, restart, rollback, cache-proof

## Decision this unlocks

Freeze the property boundary between durable, restart-derived, and
cache-bounded state; the tamper-detection rule; the replay-bound
contract; and the Go storage interface that S6.4 (authority,
delegation, recovery) consumes. Without this freeze, S6.4 would
either embed a new storage engine silently or fall back to in-memory
state, both of which violate the research discipline rule against
unresearched technology selection and the R-039 durability
contract.

## Current contract

R-039 § Fixed product contract fixes:

- Lease follows Active, finite Grace, and Released; reclaim creates a
  new Name Generation and revisions are monotonic within a generation.
- Recovery Pending fails closed without any administrative fallback.
- Direct Target connections remain pinned; name-origin connections
  never silently retarget.

`internal/network/store` (per `package-map.md`) is the existing H3
state ownership: exclusive state root, bounded durable files,
immutable generations, control journal, atomic pointers, write-new →
fsync → atomic rename → directory fsync. R-005 fixes Time Confidence.
R-029 fixes the authenticated Network Epoch. R-042 fixes the order
key, which already includes `epoch_number`. R-044 fixes Ed25519 and
BLS12-381 for signed entries. R-045 fixes the per-surface capability
counter that must survive restart.

`horizon-3-stage-6-brief.md` S6.4 fixes the requirement that the
predecessor permanently loses future authority power, that delegation
is bounded by parent generation and lifecycle, and that Recovery
Pending is bounded and fail-closed.

What remains open before S6.4 can start is the property boundary
(durable vs restart-derived vs cache-bounded), the tamper-detection
rule, the replay-bound contract, and the storage interface boundary.

## Hypotheses

- **H1:** the existing `internal/network/store` is the right default
  implementation behind a Go `Storage` interface, and the property
  boundary can be expressed entirely in terms of that interface.
- **H2:** `internal/network/store` is the only acceptable
  implementation; no interface is needed.
- **H0:** a new storage engine is required to satisfy the property
  boundary.

## Evaluation criteria

1. **Property boundary explicit:** every Stage 6 state element is
   classified as durable, restart-derived, or cache-bounded, and the
   classification is enforceable through the storage interface.
2. **Tamper fail-closed:** any modification of a durable entry is
   detected through the signed entry and produces a `state-tampered`
   Connection Result; no read, no recovery, no override.
3. **Replay-bound:** every durable key includes `epoch_number`; a
   read with `epoch_number < current authenticated epoch` returns
   `state-stale` and never returns the older value.
4. **Atomic write:** a batch write either commits all entries or
   none; a crash mid-batch leaves the previous state intact.
5. **Replaceable interface:** the Go `Storage` interface is the only
   contract S6.4 imports; the default implementation is
   `internal/network/store`, but a future ADR may swap the default
   without rewriting S6.2, S6.4, or S6.5.
6. **No silent engine swap:** S6.4 does not import a new storage
   engine; any new engine is a new research record and a new ADR.

## Evidence plan

Primary sources, accessed 2026-08-19:

- R-039 — H3 private naming lifecycle (accepted 2026-08-17).
- `horizon-3-stage-6-brief.md` S6.4.
- `stage-6-readiness-checklist.md` §B.3.
- `internal/network/store` — existing H3 state ownership (per
  `package-map.md`).
- R-005 — hostile bootstrap and Time Confidence.
- R-029 — authenticated Node lifecycle (Network Epoch).
- R-042 — claim ordering and `epoch_number` binding.
- R-044 — Ed25519 and BLS12-381 (signed entries).
- R-045 — Anonymous Cost capability counter (durable).
- ADR-0009 — Go project foundation.

The property boundary, tamper rule, and replay-bound are implemented
in S6.4 against this contract; no new experiment is required for
R-043.

## Failure scenarios

- A durable entry reads back after a modification that bypasses the
  signed-entry check.
- A restart-derived entry (ephemeral handle, in-flight admission,
  resolver query state) persists across restart.
- A cache-bounded entry (resolved Target with freshness proof) is
  used after the freshness proof expires.
- A read with `epoch_number < current authenticated epoch` returns
  the older value instead of `state-stale`.
- A crash mid-batch leaves partial state visible to a subsequent
  read.
- The Go `Storage` interface is declared but the default
  implementation does not compile against it.
- S6.4 imports a new storage engine (BoltDB, BadgerDB, LevelDB, etc.)
  without a new ADR.

## Options and recommendation

1. **Option A — reuse `internal/network/store` as-is (no interface).**
   Zero new code, but no compiler-checked contract and no
   testability without an adapter layer. Rejected: a future swap
   would require rewriting S6.4.
2. **Option B — wrap `internal/network/store` behind a Go `Storage`
   interface (recommended).** Default implementation is the existing
   store. S6.4 imports only the interface. A future ADR can swap the
   default without rewriting S6.2, S6.4, or S6.5.
3. **Option C — split into separate Lease, Generation, and Record
   stores.** Cleaner separation, but three places to maintain and a
   larger error surface. Rejected: the property boundary already
   expresses the separation through the value type, not the store
   type.
4. **Option D — introduce a new storage engine (BoltDB, BadgerDB,
   LevelDB, etc.).** New dependency, new audit, new lock-in.
   Rejected: no threat-model driver in R-001 / R-005 / R-029.

Recommendation: **Option B**, accepted by the Product Owner on
2026-08-19.

## Disposition

- R-043 becomes `decided`. The open row in `docs/research/questions.md`
  is updated to point at this record and the frozen contract.
- §B.3 of `stage-6-readiness-checklist.md` is checked.
- S6.4 (authority/delegation/recovery) may implement durable,
  restart-derived, and cache-bounded state through the `Storage`
  interface, with `internal/network/store` as the default
  implementation.
- A future replacement of the default storage engine is a new
  research record and a new ADR; the interface boundary does not
  change.
- This freeze does not authorize code; the Stage 6 coding gate
  remains closed until the corrected brief, plan, and evidence
  contract are accepted and the Product Owner records the coding
  start decision.
- No ADR is required: this is a property boundary and an interface
  boundary that reuses the already-decided H3 supply; no new
  technology is selected.
