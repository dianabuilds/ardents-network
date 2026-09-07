---
status: proposed
date: 2026-09-06
extends: ADR-0074
---

# Separate public state validity, agreement, and activation

## Context

[R-149](../research/records/r-149-autonomy-transition.md) found that owner
signatures, admitted input ordering, derived-state correctness, available data,
commitment/finality, and current usability are separate proof obligations.
Replacing the current threshold by one generic validator success flag would
hide these distinctions. A signed record may be valid but uncommitted; a committed
snapshot may be expired; an included leaf does not prove a complete correct View.

## Proposed decision

An autonomous public profile defines one closed validation contract for all
these obligations. State becomes usable only after its own profile proves:
owner authorization and deterministic transitions; the agreed input history and
projection; sufficient availability and verification evidence; its selected
commitment/finality; and activation within trusted freshness bounds, compatible
rules, and retained safety floors. Neither a distributor nor an interchangeable
consensus plugin may assert these facts through unchecked booleans.

Keep receipt, inclusion, provisional agreement, committed state, and current
usable state distinct. Provisional branches cannot grant live duties or current
Name authority. A contradiction with an already committed safety floor produces
an explicit conflict/fork outcome; it cannot silently revoke prior ownership or
reset the floor. This is conditional on a selected fault/finality model, not an
absolute guarantee against every adversary. Probabilistic finality requires an
explicit residual-risk and post-commit reorganization contract before selection.

An ordinary Endpoint is not required to produce consensus work or replay an
unbounded global history. Full verification and bounded Endpoint verification
have separate declared evidence paths and budgets; a sampled Merkle proof alone
cannot carry completeness, validity, availability, or latest-state claims.
Production influence has its own reviewed Sybil boundary; open participation is
not one-key-one-vote and relay capacity is not consensus weight by default.

The working semantic contract belongs to the
[operating model](../product/operating-model.md#autonomous-shared-state-contract),
with requirement maturity in the [functional map](../product/functional-map.md#autonomous-public-requirements).
The maintained State/Namespace/Release owners remain responsible for their
current boundaries; this proposal creates no generic ledger package or new API.

## Alternatives and consequences

A single signed-quorum verdict is simpler but can hide invalid projections and
stale state. Requiring every client to replay everything simplifies part of the
trust argument but conflicts with bounded long-term client costs. The proposed
separation makes the proof obligations explicit and may reject otherwise
attractive consensus systems or require a more limited product profile.

This proposal selects no algorithm, chain, committee, weight, clock mechanism,
proof format, parameter value, storage migration, or public claim. Adoption as an
implementation contract additionally requires the unresolved choices and
conformance cases in R-149. Current C0 signatures, conflicts, floors, Release
Safety, and Namespace behavior remain binding.
