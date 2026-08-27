---
status: accepted
date: 2026-08-26
---

# ADR-0043 — Derive Grace from signed Name deadlines

## Context

Canonical Records contain signed Lease and Grace deadlines, but the prior
current-proof leaf treated an `active` Record as unavailable immediately at
`LeaseExpiresAt`. Its explicit `grace` state could occur only after a separate
materialization revision. That contradicted the accepted lifecycle: Grace
preserves resolution and exclusive renewal until its finite end.

Making a project-operated time-advance service would create an unselected
registrar/control root. Waiting for an Epoch close for every Lease boundary
would also make ordinary Grace availability depend on a currently unselected
H4-6 close cadence.

## Decision

For a current `active` Record, derive its observable Grace state at
verification time from its signed deadlines. An authenticated materialization
leaf V3 commits the earliest eligible active-to-Grace deadline across the
complete lineage and the finite lineage/Record `notAfter` boundary. A verifier
returns the same immutable Binding with the Grace warning strictly after that
deadline and until `notAfter`; it fails closed thereafter.

An explicitly materialized `grace` Record, or an explicitly materialized Grace
parent, continues to yield the Grace warning. A materialization whose Record
validity ends before a usable Grace interval has no derived Grace interval.
The current Authority may renew an active Record through its signed Grace end;
the renewed successor is again an explicit active Record.

This is local proof semantics, not a new time service: all Endpoint clock,
freshness, offline, and skew limits remain an explicit selected profile
responsibility. The rule does not create a current `released` Record, select a
reclaim winner, or make root claims public. Explicit Release, reclaim, conflict
resolution, recovery completion, and public current-state materialization
remain H4-6-controlled operations.

## Consequences

- An H4-4B verifier can show Grace without a per-Name project control action.
- Current-proof V3 changes the compact leaf encoding. V1/V2 leaves retain
  their historical static-state interpretation; newly materialized state uses
  V3.
- The threshold materialization still authenticates the lineage summary, and
  a stale/conflicting/missing current proof still fails closed.
- H4-4B is not complete merely because derived Grace works: lifecycle release
  and reclaim require the selected shared control path.

## Compliance

- [R-116](../research/records/r-116-canonical-name-time-materialization.md)
  records the evaluation and proof-level evidence.
- Namespace tests cover own- and parent-deadline Grace derivation, static Grace
  compatibility, released refusal, and renewal while the signed Grace interval
  remains live.
