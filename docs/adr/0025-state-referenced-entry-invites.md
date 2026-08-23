---
status: accepted
date: 2026-08-23
supersedes: none
---

# ADR-0025 — Use State-referenced Entry Invites

## Context

The selected native Interactive Route needs a bounded Entry authorization. H3
Bridge Invites carry a WebTunnel envelope, even though authenticated Network
State already owns the candidate identity, endpoint, key, Role Domain, and
validity facts. Retaining that envelope would preserve a second reachability
authority and the retired H3 transport.

## Decision

Adopt the signed, state-referenced Entry Invite v1 defined by R-077. It binds
exact Network/Epoch/Digest/Profile, candidate identity/family/record/domain
facts, validity, and one two-slot replacement lineage. It carries neither a
carrier endpoint/key nor any Target, Route, Service, or retry instruction.

`entry` resolves the adjacent endpoint and mutual-TLS pin only from current
authenticated State and rejects a mismatch, stale State, expired/signature-
invalid Invite, wrong Role Domain, or local conflict. The only maintained
carrier is the native TCP/TLS 1.3 Profile from ADR-0024. H3 Bridge Invite,
transition, and WebTunnel bytes have no reader or migration path.

## Consequences

- State remains the single candidate/discovery authority;
- Entry retains bounded durable Invite/replay/replacement history, but cannot
  select a Route or route around a State revocation;
- M7 deletes `bridge` and `camouflage` rather than adapting either; and
- historical H3 evidence stays readable only through its named provenance path.

## Compliance

[R-077](../research/records/r-077-entry-invite-v1.md) specifies the canonical
format, validation, failure cases, and required M7 tests. This selection adds
no public Entry distribution, camouflage, independent-operation, or Route
Qualification claim.
