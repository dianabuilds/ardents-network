---
status: accepted
date: 2026-08-24
supersedes: none
---

# ADR-0035 — Use live Introduction slots and EndpointTransitBinding v1

## Context

The selected C-2 path must deliver one sealed invitation to a Publisher
without a direct Publisher listener or a Rendezvous Service lookup.
`EntryBinding` v1 is intentionally only User-to-Initiator and cannot authorize
an Endpoint-to-Introduction or Publisher-to-Responder first hop.

## Decision

The Publisher maintains one finite outbound live slot at a State-selected
Introduction Node. It registers an opaque `Reachability` and one-use
`JoinHandle`; the Node retains no offline message. A User sends an already
sealed `SealedIntroduction` through a separate C-2 attachment. The Introduction
Node checks its identity, expiry, Reachability and JoinHandle, spends the handle
once, and forwards the exact sealed bytes on the live slot. It neither opens
HPKE nor learns a Target, plaintext, complete route, or Service origin.

Kind 6 `EndpointTransitBinding` v1 is the closed first-hop record for only
Introduction and Responder transit roles. It binds Network, epoch/digest,
attachment ID, transit role/Node ID, expiry, an ephemeral mTLS client-key
digest, and a finite opaque authorization. It is not an Entry Invite and has
no User identity, Target, Service material, endpoint literal, or fallback.
`EntryBinding` v1 remains the User-to-Initiator record; no reader reinterprets
one form as the other.

Kind 7 `IntroductionSlotRegistration` v1 binds only the opaque Reachability,
JoinHandle, and whole-second slot expiry after Publisher-side TLS admission.
The separate private slot authorization remains only in that admission binding.
The registration record is never a descriptor, Service publication, Target
lookup, or retained message.

After authenticated delivery, the Publisher validates the HPKE material and
uses a separately admitted Responder first hop to open exactly one
State-pinned Responder-to-Rendezvous leg. Rendezvous still pairs only those
two outbound Node legs and never plans or looks up a Service.

The HPKE plaintext is `ServiceIntroductionInstruction` v1. It binds the
Service Target, Credential generation, current publication digest, and the
one attachment ID. Its codec and comparison live in `service/publication`:
only the decrypted Publisher compares it against its retained current
publication. It carries no Node endpoint, candidate, retry, route plan, or
Application Data.

## Consequences

- C-2 has explicit finite slot, submit, forward, replay, deadline, drain, and
  withdrawal ownership; unavailable slot/delivery is an explicit failure.
- Publisher runtime must compare the decrypted instruction with the current
  Service publication and with the authenticated visible header before it can
  use a Responder attachment.
- The current codec adds no live Node duty, descriptor, service connection,
  direct Publisher ingress, retention, browser proxy, or privacy claim.

## Compliance

R-105 records the decision evidence and the Product Owner accepted C-2 on
2026-08-24. Canonical codec vectors refuse malformed role, registration, and
framing substitution. The required multi-process tracer precedes retained C-2
runtime.
