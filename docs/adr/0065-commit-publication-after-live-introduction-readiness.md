---
status: accepted
date: 2026-08-31
extends: 0035-live-introduction-slots-and-transit-binding-v1.md; 0064-separate-service-authority-custody-from-instance-enrollment.md
---

# ADR-0065 — Commit headless publication after live Introduction readiness

## Context

The maintained lower-level publication path commits a Service Instance
generation after an owner-local `ARIA` signature. No accepted headless product
owner provisions that signer or socket. Retaining it in the supported runtime
would add a hidden publication authority, while adding a portable Node receipt
would reopen the accepted wire without a consumer that needs such a proof.

ADR-0035 already defines matching `IntroductionSlotReady` as the exact proof
that a State-selected Introduction Node retained one authenticated live slot.
ADR-0064 gives Endpoint only a non-exporting host Instance binding and requires
ambiguous or consumed generations to fail closed.

## Decision

The headless Publisher uses one Endpoint-owned publication transaction.
Endpoint stages the Instance-signed Publication and advances its durable local
generation floor, registers the exact State-selected Introduction slot using
acquired one-use transport authority, and verifies the matching canonical
`IntroductionSlotReady`. The canonical registration and ready records form the
local acknowledgement transcript committed by the existing Publication
acknowledgement digest. Endpoint exposes the Publication and reports
`published` only after that readiness transition succeeds.

Failure, cancellation, or ambiguity after the generation floor advances
terminally closes the live binding; restart requires a fresh Instance and a
higher Credential generation. Endpoint never rolls the floor back or exposes a
record without its live slot. Withdrawal prevents new work, drains retained
work, closes the slot, removes the exposed record, and withdraws the binding.

The legacy owner-local `ARIA` signer/socket remains lower-level compatibility
evidence only and is absent from the maintained headless runtime. No public
wire field, Node signature, Credential, Transit Grant v1, Route, Target, or C-2
semantics change.

## Consequences

- Publication readiness and retained-slot readiness are one local transaction,
  so crash tests must cover every boundary between floor advancement, slot
  registration, readiness, exposure, and binding commit.
- A ready transcript is local evidence, not a portable Node receipt or a new
  Application-visible authority claim.
- A User-only candidate or a versioned signed slot receipt would require a new
  Product Owner decision rather than an implementation fallback.

## Compliance

[R-132](../research/records/r-132-headless-publication-readiness.md) records the
accepted trade-off and rejected alternatives. The Product Owner accepted the
exact R-132 statement on 2026-08-31.
