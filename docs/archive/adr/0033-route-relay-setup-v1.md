---
status: accepted
date: 2026-08-24
supersedes: none
---

# ADR-0033 — Use the closed Route RelaySetup v1 exchange

## Context

`EntryBinding` authenticates one User-to-Initiator TLS attempt, and
`LegBinding` authenticates an already-open Node-to-Node leg. Neither authorizes
the intermediate action: an Endpoint-selected Initiator or Responder opening
one exact adjacent Rendezvous leg. Letting the transit Node derive or choose
that peer would contradict endpoint-local Route selection in ADR-0024.

## Decision

After admitted Entry TLS and before application-carrier bytes, the Endpoint
sends one canonical `RelaySetup` record. Its fixed fields are Network ID,
epoch, epoch digest, attachment ID, whole-second expiry, transit role/Node ID,
and one next Rendezvous Node ID/public key. It carries no literal endpoint,
Service data, target, instance, route plan, candidate list, URI, or retry
policy.

The transit Node verifies that it is the stated role/identity, consumes the
attachment once, and rechecks its fresh State view for the exact next
Rendezvous identity, key, role/profile, literal endpoint, and validity. Only
then may it open TLS and complete the existing reciprocal `LegBinding`. It
returns a byte-for-byte equivalent `RelayReady` record only after that succeeds.
All mismatch, replay, expiry, State absence/change, dial, TLS, or binding
failure terminates the attachment; no fallback or alternate candidate is tried.

## Consequences

- Endpoint retains next-hop selection while each transit Node sees only its
  immediate peer;
- the `route` package owns new wire kinds 4 (`RelaySetup`) and 5
  (`RelayReady`), with no legacy reader; and
- Initiator/Responder duties must own State recheck, one-use admission,
  dialing, bounded forwarding, drain, and terminal cleanup before they can
  make the exchange live.

## Compliance

R-104 provides decision evidence. Canonical codec vectors cover the grammar
and substitution/malformed refusal. This decision does not implement a transit
listener, direct fallback, complete Route, browser path, or privacy claim.
