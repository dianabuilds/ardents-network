---
status: accepted
date: 2026-09-01
extends: 0063-bootstrap-transit-issuer-from-owner-root.md
---

# ADR-0068 — Bind Transit Grant issuer roots to State generation

## Context

ADR-0063 binds an issuer root to the first authenticated State digest, Epoch,
issuer, signer, and deadline. A successor that changes only the independently
authenticated State generation could otherwise retain the same binding tuple.
The v1 owner-root marker and JSON record also have no explicit compatibility
boundary for this correction.

## Decision

`StateDuty` carries State Generation as an independent canonical lowercase
64-hex discriminator. The issuer scope and durable issuer-root record include
it in every validation and equality check. A live issuer whose current State
has any different generation is unavailable before lookup, withdrawal, or
reservation; reopening succeeds only for the exact same generation.

Issuer roots use only `.ardents-local-roles-v2` containing exactly
`ardents-local-roles-v2\n` and strict JSON `Version: 2`. There is no v1 reader,
migration, or reuse path beyond read-only classification for rejection. A v1
marker-only, unbound, or bound root is rejected read-only before lease
acquisition and rechecked under the lease; it is not locked,
permission-mutated, staged, or repaired. Replacement is an explicit new
initialize/bind ceremony in a distinct empty root.

Transit Grant, Profile, Request, and Outcome wire bytes are unchanged.

## Consequences

- State digest equality is not treated as State-generation continuity.
- A stale or copied v1 root requires a deliberate replacement ceremony rather
  than an in-place compatibility action.
- The bounded issuer ledger remains scoped to one exact State generation and
  cannot issue while a generation-only successor is live.

## Compliance

[R-135](../research/records/r-135-transit-issuer-generation-continuity.md)
records the Product Owner decision and retained behavior evidence.
