---
status: accepted
date: 2026-08-26
supersedes: none
---

# ADR-0046 — Bind the Destination Resolution Gateway profile in Network State

## Context

ADR-0036 and ADR-0037 require a User Endpoint to perform one private Target
lookup through an Initiator and one Destination Resolution Gateway.  State
already assigns the Gateway Node's `destination-resolution` duty and lets the
Initiator derive that Node's literal HTTPS endpoint.  It did not, however,
carry the Gateway's signed OHTTP key configuration.  A participant runtime
would then have to choose an arbitrary candidate, read a profile from an
operator plan, or make a first unauthenticated lookup.  Each is a second
selection or discovery authority.

## Decision

For the interactive Route profile, Network State is the sole source of the
selected Destination Resolution Gateway fact.  A versioned Epoch projection
binds exactly one Gateway Node ID to the Gateway's signed, bounded OHTTP
`GatewayProfile` bytes.  The Epoch's threshold signatures authenticate that
association.

State accepts that projection only when its Node ID is the one authenticated
candidate assigned to the `destination-resolution` domain.  It returns the
candidate's Node ID, public key, family, validity window, and the exact
profile bytes together as one narrow, time-bounded resolution view.  An absent,
ambiguous, stale, expired, or mismatched projection is unavailable; there is no
candidate ordering rule, URL override, profile override, or fallback.

`internal/service/reachability` remains the owner of the GatewayProfile grammar
and self-signature.  The Endpoint decodes and verifies the State-projected
profile against the State-projected Node identity, Network ID, and exact
lookup window, then passes only those validated facts to the existing closed
Entry-to-Initiator OHTTP carrier.  The Initiator continues to derive the
Gateway's HTTPS endpoint only from its own State duty.

## Consequences

- An interactive State Epoch without this projection cannot authorize a
  participant private-reachability lookup, even if it otherwise contains a
  destination-resolution Node.
- The profile's State association and its Node self-signature have distinct
  owners: State selects and authenticates the association; Reachability
  authenticates the OHTTP key material.  Neither becomes a Name, Target, or
  Publisher authority.
- The profile is public configuration, but it is not an Internet DNS record,
  user-supplied endpoint, generic proxy route, or direct Endpoint-to-Gateway
  dial permission.
- Existing Epoch encodings without the projection remain historical/limited
  State evidence; they cannot be silently upgraded into private-reachability
  authority.

## Compliance

- [ADR-0036](0036-target-private-reachability-v1.md) owns the Target-private
  reachability protocol.
- [ADR-0037](0037-private-reachability-entry-carrier.md) owns its closed
  Initiator carrier.
- [R-106](../research/records/r-106-target-reachability-alpha.md) records the
  missing-fact audit and Product Owner acceptance.
