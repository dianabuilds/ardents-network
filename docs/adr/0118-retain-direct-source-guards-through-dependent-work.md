---
status: accepted
date: 2026-09-29
---

# ADR-0118 — Retain Direct Source guards through dependent work

## Context

ADR-0005 excludes a Direct-Origin Source identity and known family from
Route/Resolution roles while exposure and derived work remain. Today State's
serving Duty ends at the Epoch `ValidUntil` even while its Source server can
answer from retained bytes, and an outbound wave's initial Duty ends at its
15-second journal deadline even if contact or verification has not joined.
[R-169](../research/records/r-169-source-duty-lifetime.md) records the evidence.

## Decision

State owns the local Direct Source collision guard through joined completion of
every operation that can serve from or contact that Source, and through the
authenticated terminal bound of state derived from contact. Neither Epoch
validity nor a wave deadline alone releases an active guard. Stop admission and
join dependent work before removing a guard; if that cannot be proven, refuse
new State-dependent work and retain the collision. A serving successor cannot
drop a predecessor still selected by an accepted response. Rapid A→B→C changes
retain every still-dependent identity and known family, even if the same
State-owned serving producer has overlapping records. State-owned exposure
predecessors may likewise overlap only when retaining distinct contacted
tuples requires it; the two Sources in one accepted plan remain distinct.
Cross-producer identity/family collision stays forbidden. An outbound wave
holds its contacted Source guards
through contact, verification, terminal publication, and any retained derived
State bound.

The 64-record installation-wide Direct Source cap remains a hard availability
limit. Refuse a transition before exposing or publishing another unguarded
Source if its required guards cannot fit. After a crash, an exclusive State-root
owner may release its own stale *work-only* guards after verifying the durable
current/pending/control floors. Process death and the exclusive root lease prove
old handlers cannot resume; no persisted handler set is assumed. A guard that
could protect retained derived State stays until its authenticated terminal
bound is proven. If durable evidence cannot establish which guard is work-only,
uncertainty preserves collision and refuses recovery. Other producers' records
are never reclaimed as part of this cleanup.

## Consequences

This chooses guarded continuity over time-only cleanup. It can postpone a
successor or make State unavailable under cap pressure or uncertain recovery.
It selects no expired-Epoch response policy, wire change, new time authority,
or new long-lived protocol promise. The current Network owner defines the
precise release and recovery rules; implementation must prove them with
boundary, concurrent response, A→B→C, cap, and restart tests.
