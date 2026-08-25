---
status: accepted
date: 2026-08-25
supersedes: none
---

# ADR-0039 — Use State-authorized opaque Transit Grants for C-2 admission

## Context

ADR-0035 binds an Endpoint-to-Introduction or Endpoint-to-Responder TLS
attempt to one exact State tuple, but deliberately leaves the finite opaque
authorization to the receiving Node. The process fixture provided synthetic
callbacks only; promoting those strings to participant configuration would
make an unowned fixture a hidden authorization service.

H4-3A needs an offline, bounded admission decision for the closed alpha. It
must not give a Route Node Service Authority trust, reveal Target or Service
data, require a live registration database, or let a Node select a route.

## Decision

`EndpointTransitBinding.Authorization` carries a closed, versioned opaque
`Transit Grant v1`. One current Network State authority signs the grant with
its existing Ed25519 authority key. The grant contains only the exact
Network ID, Epoch, State digest, attachment identifier, transit role, transit
Node identifier, TLS client-key digest, expiry, random grant identifier, and
issuer identifier. The signature covers its canonical binary representation.

An Introduction or Responder verifies the issuer against its current,
authenticated State authority view and verifies every signed field against the
presented binding and its State-assigned local duty. Before allocating C-2 or
relay work it durably records the grant identifier in its owner-local replay
ledger. A repeated, expired, changed, unknown-issuer, withdrawn-State, or
failed-persistence grant rejects without fallback. State change withdraws the
duty, so an old State authority view cannot keep accepting a grant.

The Publisher retains Responder grants. A Reachability Descriptor may contain
only the exact Introduction grant necessary for the User's C-2 first hop.
Grants contain no Target, publication body, HPKE plaintext, full Route,
address history, or peer-selection instruction.

Because a grant binds the TLS client-key digest, an offline-issued grant is
usable only with its matching private, one-use TLS client key. For the closed
alpha, each finite grant is provisioned with that pair into the intended local
Endpoint's protected alpha material; the publicly reachable Descriptor carries
only the opaque Introduction grant, never its matching private key. The
Publisher keeps both Responder grant and its matching pair locally. This is a
finite per-attempt capability, not a stable User identity, shared CA, or
browser certificate. A future on-demand issuer or different key-binding model
requires a superseding decision.

The Transit Grant authorizes only the adjacent TLS attachment. It is not the
Publisher-selected `JoinHandle`: Introduction retains that independently inside
the registered slot and the sealed C-2 record. Reusing the old opaque field as
both values would make key-bound grants unable to reach a legitimate slot.

Issuance is a finite project-operated action at publication time for the
closed alpha. It is not a permissionless resource market, public control
plane, availability promise, or independent-operator claim.

## Consequences

- The Node receives a locally verifiable one-use capability without a live
  broker, but its replay ledger becomes a durable Node responsibility.
- A single current State authority can issue a finite alpha grant. Threshold
  authorization for grant issuance is intentionally not claimed; changing
  this requires a new versioned decision.
- The grant grammar is purpose-specific and cannot become a generic bearer
  credential, proxy authorization, or substitute for Entry Invites.
- Authority key rotation and State withdrawal invalidate prior operating
  context by the existing Node lifecycle rather than an out-of-band revocation
  service.

## Compliance

R-109 records the evaluation, alternatives, and required refusal evidence.
H4-3A must demonstrate canonical-vector verification, all exact-tuple and
issuer substitutions rejecting, atomic one-use spending across restart, and a
two-Endpoint path that exposes only the Introduction grant. The result remains
a closed alpha until those tests and the supported-browser qualification pass.
