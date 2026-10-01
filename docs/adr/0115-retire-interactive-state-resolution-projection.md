---
status: accepted
date: 2026-09-28
partially-supersedes: ADR-0066 (State Publisher attachment projection); ADR-0070 (State private Resolution input to the former User Route)
---

# ADR-0115 — Retire the uncomposed interactive State resolution projection

## Context

ADR-0066 required State to supply one indivisible interactive Publisher
attachment, and ADR-0070 made a private Resolution view an input to its
volatile User Route. ADR-0092 removed the generic Endpoint Transit-acquisition
chain, ADR-0093 removed the Route v2 execution closure, and ADR-0100/0105
removed private naming Resolution and Namespace. The selected closed Route
instead consumes State's accepted closed profile, exact signed recipient
constraints, and current Snapshot. At the integrated Node/Endpoint baseline,
`CurrentResolution` and the interactive view operations have no non-test
caller; Snapshot's Destination/Transit fields only served that view. The
Product Owner selected narrow retirement in
[issue #315](https://github.com/dianabuilds/ardents-network/issues/315).

## Decision

1. Retire State's `ResolutionView`, its Gateway, CredentialIssuer, and
   PublisherAttachment projections, and their exclusive Snapshot authority
   and Destination/Transit fields. The former interactive Endpoint attachment
   is no longer a maintained State Interface. This supersedes ADR-0066's
   indivisible Publisher projection clause and ADR-0070's State private
   Resolution input to the former User Route; it does not restore either
   retired execution chain.
2. Keep `epoch`'s authenticated AREP v2/v3 grammar, candidate-association
   checks, and historical generation readers. Keep State's current, pending,
   conflict, trusted-time, and Direct Source admission and durable floors.
   Keep the generic authenticated View commitment in Snapshot and the
   `Current`, `CurrentNodeDuty`, `CurrentClosedProfile`, and
   `CurrentClosedRoute` projections. No wire or persisted identity changes.
3. The maintained closed participant requires the accepted closed Route
   profile joined to the same current State generation and Node Records.
   Endpoint owns its closed token journal and cannot accept caller-selected
   peers, roles, or fallback authority. Missing, conflicting, expired, or
   mismatched State remains unavailable.

## Consequences

The State root loses one uncalled interactive Interface and its obsolete
projection tests. Focused `epoch` tests retain signed historical Gateway and
issuer candidate-association coverage that those tests had also exercised.
Product scope, Endpoint's technical owner, and the package map describe only
the supported closed State projection. A future interactive path requires its
own accepted contract and actual consumer; this decision does not select one.
