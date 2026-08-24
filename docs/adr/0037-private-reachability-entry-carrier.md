---
status: accepted
date: 2026-08-25
supersedes: none
---

# ADR-0037 — Carry private reachability through a closed Initiator operation

## Context

ADR-0036 selects fixed-size OHTTP for Target-private reachability. Its ordinary
HTTP adapter was sufficient to qualify the Relay/Gateway boundary, but an
Endpoint using that adapter would make a direct TCP connection to the Relay.
That contradicts the Route boundary: the Endpoint's network-adjacent peer is
its State-authorized Initiator Entry, not an arbitrary HTTPS origin.

The existing Entry binding is deliberately only a one-attempt admission
capability. The accepted `RelaySetup` that follows it authorizes a native
Initiator-to-Rendezvous leg, not arbitrary TCP or HTTP forwarding.

## Decision

Add a separate, single-use **ResolutionRelaySetup** operation after an
admitted User-to-Initiator Entry TLS connection. It binds Network ID, Epoch,
State digest, attachment ID, Initiator identity, one State-selected Gateway
identity/public key, whole-second deadline, and a fixed opaque-envelope
capacity. It carries no Target, descriptor, Service, application data,
endpoint literal, alternate candidate, or browser authority.

The Initiator validates that exact setup against its current authenticated
State facts, derives the Gateway's literal HTTPS endpoint only from those
facts, exchanges one opaque OHTTP envelope with it, returns one bounded opaque
response, and closes both legs. It does not decrypt or retain the envelope.
The Entry invite remains admission-only; the new setup limits the one action.
Existing C-2 `RelaySetup`/`RelayReady` semantics stay unchanged.

## Consequences

- H4-3 can compose descriptor acquisition without a User-to-Relay TCP
  connection or a generic proxy surface.
- A private lookup uses a fresh Entry attachment distinct from the later C-2
  Service Connection and applies the existing replay/TLS-key binding.
- The State model must supply one unambiguous Destination Resolution Gateway
  fact for the Initiator duty. Missing, expired, substituted, oversized, or
  failed exchanges are explicit private-reachability failures with no
  fallback.
- The new closed Route records and Initiator resource accounting require
  behavior evidence for exact binding, one-use forwarding, expiry, overflow,
  wrong Gateway, and absence of Target parsing before browser handoff uses
  them.

## Compliance

R-107 records the alternatives, evidence, and Product Owner acceptance.
`docs/technical/private-reachability.md` defines the operational boundary.
