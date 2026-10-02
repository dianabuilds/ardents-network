---
status: accepted
date: 2026-10-02
---

# Acknowledge bounded replenishment of a paired dedicated JOIN channel

## Context

The Product Owner selected bounded JOIN replenishment on 2026-10-02 in
[the recorded decision](https://github.com/dianabuilds/ardents-network/issues/456#issuecomment-5948490098),
and explicitly clarified that ACCEPT is charged after restoring the allowance.
The [R-154 continuation](../research/records/r-154-join-data-lane-transition.md#bounded-replenishment-continuation)
records the contradiction and comparison. Existing implementation was evidence
of that contradiction, not authority to extend the forwarding-only grammar.

## Decision

Extend [ADR-0083](0083-activate-joined-rendezvous-data-lane.md) and the
forwarding-only channel restriction of [ADR-0085](0085-bound-forwarding-replenishment.md)
only for an independently admitted dedicated class-2 DataJoin channel after
both opposite sides are paired, RESULT-confirmed and live. Later class-2 ADMIT
uses lane zero of that same channel and creates no child or second JOIN.

Retain the original receiver, TLS exporter, HELLO, peer, purpose, context and
absolute deadline bindings. Debit the complete ADMIT from the old allowance.
The installed Hosting owner verifies the token and reserves actual provider-period
work and termination capacity before durable fresh-token spend. Failure refuses
without restoring allowance; cancellation or expiry cannot revive accepted work.
After durable acceptance and a live-side recheck, set remaining reserve to exactly
32 MiB; never add it, refund past debits or change the counterpart's allowance.

Emit one matching lane-zero ACCEPT, status zero and credit 64 KiB, only after that
acceptance. Charge its complete 21 bytes from the new reserve before attempted
output. The original lane-1 windows remain unchanged: acknowledgement credit is
not new data credit. The existing ordered channel and one pending refill supply
matching; no new wire field is introduced. Serialize complete RESULT, ACCEPT and
forwarded data/control frames for each side. Refused/failed refill supplies no
successful acknowledgement; failed output terminates and joins the pair.
Reservations remain owned until joined release, preserving actual cleanup causes.

## Consequences

The [current protocol](../technical/protected-route-protocol.md#rendezvous-join-data-lane-transition)
owns this exact exception. Other channel types and child lanes remain forbidden.
This adds no authority, dependency, protection mode or public protocol promise.
Evidence must exercise real admitted paired server/client refill at its traffic
threshold twice, actual Hosting reserve/spend/replay, concurrent opposite-side
output, unchanged windows/deadlines, refusals, cancellation and both Carriers.
Selection alone supplies none of that implementation or qualification evidence.
