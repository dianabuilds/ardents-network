---
id: R-059
title: When may Network State recover a missing current pointer?
status: accepted
owner: product research
started: 2026-08-22
reviewed: 2026-08-22
---

# R-059 — Network State missing-current recovery

## Decision this unlocks

Resolve DA-02 before M3 changes D01 Network State storage or recovery. The
decision fixes whether a non-virgin root without `current` can activate a
generation on restart, and therefore whether a future State migration can add a
reader or repair path.

## Current contract

Accepted R-027 specifies immutable generations and one atomic `current` pointer
as the activation boundary. Outside the exact virgin root, its persistence rule
requires a missing or corrupt pointer to fail closed rather than select an older
or otherwise arbitrary generation. R-029 adopts R-027's persistence appendix
for the authenticated State/Node slice. S8.1 preserves fail-closed freshness
and lifecycle behavior; it does not preserve the current implementation.

The current implementation disagrees: `recoverMissingCurrent` in
`internal/network/state/recovery.go` accepts a complete, unique, acyclic chain
of stored generations, reconstitutes its tip, and calls `persistDecision` to
write `current`. Conversely, `refresh_test.go` requires an absent distribution
control pointer to fail. The two recovery rules are not interchangeable: a
verified body identifies data integrity, not the interrupted activation event
or its authorization as current.

## Hypotheses

- **H1 — fail closed:** any non-virgin root with absent or malformed `current`
  enters a typed unavailable/recovery-required state; no generation becomes
  active and no pointer is written automatically.
- **H2 — reconstruct one chain:** one complete, unique, authenticated chain
  authorizes State to rewrite `current` and continue normal admission.
- **H0 — redesign:** neither rule gives a safe/recoverable outcome; choose a
  different committed-state representation before M3.

## Evaluation criteria

Before examining the implementation, a candidate must satisfy all of these:

1. a crash before pointer replacement cannot activate a body merely because it
   is parseable and authenticated;
2. an attacker with filesystem write/delete capability cannot turn pointer
   deletion into rollback, stale-current selection, or resumed Node readiness;
3. a valid virgin root remains distinguishable from an interrupted non-virgin
   root without guessing from generation topology;
4. restart reports a typed, actionable terminal state and preserves evidence
   required to repair or replace the root;
5. normal commit remains one durable atomic activation boundary and no dual
   writer or automatic retry hides a failed activation; and
6. the rule can be tested with missing, malformed, multiple-tip, orphan, and
   interrupted-write fixtures on each selected filesystem platform.

The recommendation is falsified if an accepted contract proves that an
authenticated immutable generation itself is a sufficient activation authority,
or if a failure-injection test shows that H1 can resurrect unsafe work or make
safe repair impossible while H2 cannot.

## Evidence plan

### Primary sources

- R-027, persistence/activation clauses at lines 698-735, accessed 2026-08-22.
- R-029, adopted R-027 persistence appendix and immutable-generation/current
  pointer technology profile, accessed 2026-08-22.
- S8.0 Network State delta review F014/F016, accessed 2026-08-22.
- Current `internal/network/state/{recovery,storage}.go` and
  `internal/network/store/{state,files}.go`, inspected 2026-08-22.

### Experiment

No new experiment or format mutation is authorized before this decision. M3
must add a deterministic fault table that creates: virgin root; staged-but-not
committed generation; missing pointer with one chain; missing pointer with
multiple tips; orphan branch; malformed pointer; and a valid committed root.
For each case it records State availability, active generation, Node admission,
pointer writes, retained artifacts, and repair result. A platform filesystem
contract is required before treating rename/directory sync as equivalent.

### Failure scenarios

- deletion of `current` after a valid generation body is durable but before its
  atomic rename;
- deletion/replacement of `current` by a local filesystem attacker;
- a stale unique chain after an intended later revocation or conflict;
- partial pointer bytes, cross-root copied generation, or orphan branch;
- crash while emitting a typed recovery-required observation; and
- repair attempt concurrent with State/Node start.

## Findings

- **Sourced fact:** R-027 calls `current` the only activation boundary and
  explicitly requires a non-virgin missing/corrupt pointer to fail closed.
- **Sourced fact:** R-029 adopts R-027 persistence rather than replacing it.
- **Measurement:** current `recoverMissingCurrent` selects a unique chain and
  calls `persistDecision(..., true)`; its success changes durable activation
  without a separate repair authority.
- **Measurement:** current distribution-control recovery rejects a missing
  `current` pointer when generations exist, demonstrating incompatible behavior
  within the current State tree.
- **Inference:** H2 confuses a verified immutable body with the committed
  activation fact. It cannot distinguish an intentional pointer deletion from
  an interrupted activation and therefore violates criteria 1-3.

## Options

| Option | Product and security fit | Operational consequence | Disposition |
|---|---|---|---|
| H1 fail closed | Matches accepted persistence/activation semantics and preserves explicit unavailable outcome. | Requires a target-owned inspection/replace/forward-repair path; does not resume Node work automatically. | **Recommend.** |
| H2 reconstruct | May reduce apparent downtime after a benign deletion. | Lets topology select activation and makes pointer deletion behavior ambiguous. | Reject unless a superseding contract selects it. |
| H0 redesign | Appropriate if platform durability review shows the present pointer scheme cannot express a safe repair state. | Blocks M3 until a new representation and compatibility plan are accepted. | Keep as a falsification exit. |

## Recommendation

The Product Owner accepted H1 on 2026-08-22: outside the virgin root, missing or malformed
`current` must fail closed and must not be recreated automatically from stored
generations. M3 replaces the current helper with a typed recovery-required
outcome, preserves forensic artifacts, prevents Node readiness/new work, and
adds the fault table above. A Product Owner acceptance closes DA-02; DA-05 and
the M3 compatibility design remain separate gates.

The strongest argument against H1 is availability after a harmless pointer
loss. That is an operational cost, but R-027 deliberately assigns the pointer
the activation authority and rejects guessing about interrupted state.

## Disposition

- State: `accepted` by the Product Owner on 2026-08-22; M3 may implement the
  bounded fail-closed recovery change.
- No ADR is required if this confirms R-027/R-029. A superseding recovery or
  storage model would require the applicable ADR/research route.
- No experiment code is retained yet; M3 owns the deterministic failure matrix
  if H1 is accepted.
