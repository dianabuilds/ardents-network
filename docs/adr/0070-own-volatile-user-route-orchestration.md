---
status: accepted
date: 2026-09-03
extends: ADR-0066
partially-supersedes: ADR-0024 (Endpoint-local full Route selection wording)
research: R-136
---

# ADR-0070 — Route owns volatile User-route orchestration

## Context

ADR-0024 selects the native Interactive Route and says Endpoint-local `route`
selects the full Route. ADR-0066 separately makes Endpoint the durable
at-most-once owner of role-scoped Transit Grant requests and keys. The former
implementation left the actual User path in Endpoint while an unused Route
selected random candidates from a Snapshot. That split obscures which owner is
allowed to select a peer, open a carrier, or terminalize a presented Grant.

## Decision

`internal/route` owns one complete volatile User-route operation behind
`Open`, `Attach`, and `Close`. Its `Attach` input is only the authenticated
Target that Endpoint derived from a Service Link. Route:

1. reserves its caller-owned capacity before State or network work;
2. obtains one State resolution projection and its exact Gateway, Entry-bound
   Initiator, credential issuer, and descriptor-bound Introduction/Rendezvous
   facts;
3. performs private reachability through Entry and verifies the descriptor
   against that Target and State epoch;
4. opens the exact Entry/Initiator/Rendezvous carrier, submits the sealed
   Introduction, and returns only an opaque `net.Conn` Attachment plus
   immutable authenticated evidence; and
5. cancels and joins pending work and closes every active attachment.

Route does not rank a candidate list, accept a caller-supplied peer or URL,
fall back to a direct Service connection, own a durable Grant journal, or
import `internal/route/credential`.

Endpoint continues to own Service-Link parsing, local capability activation,
and its separate durable Grant/key journals. For a membership descriptor it
provides Route a callback adapter containing the exact State/issuer/transit
tuple. Route provides that adapter only the opaque Entry-to-Initiator
Credential Relay exchange. For a fixed descriptor Route verifies the existing
Grant against State directly. The credential callback enters durable
`presenting` before use; Route records `presented=true` only after the
receiving node's successful Introduction delivery result confirms TLS
admission. A connection loss or refusal before that confirmation is
ambiguously presented and is conservatively burned. Any later Service TLS
error therefore leaves the Grant spent/burned rather than reusable.

This supersedes only ADR-0024's old selection/orchestration wording. Its
native profile, TLS, Entry boundary, no-fallback rule, carrier set, and wire
decisions remain current. ADR-0066 remains the durable issuance decision.

## Consequences

- State and Descriptor facts now have one volatile orchestration owner.
- Application Connection receives only Route's verified attachment evidence;
  its public Service-Link grammar and success format do not change.
- The former random Snapshot selection path and its selection-specific tests
  are retired. State is authoritative for every adjacent User peer.
- Route tests exercise admission-first ordering, exact cleanup, cancellation,
  and evidence; Endpoint tests exercise capability precedence and its durable
  credential adapter. Existing wire codec and process tests remain required.

## Non-claims

This makes no anonymity, availability, public-route, independent-operation,
or qualified-Application-isolation claim. It selects no public transport,
storage, consensus, or protected Application runtime.
