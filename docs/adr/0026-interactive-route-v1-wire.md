---
status: accepted
date: 2026-08-23
supersedes: none
---

# ADR-0026 — Use the closed Interactive Route v1 wire

## Context

ADR-0024 selects a native TCP/TLS Route but deliberately retires the H3 tracer
bytes. A TLS carrier does not itself specify Route framing, State binding,
sealed Introduction metadata, or the no-downgrade reader policy.

## Decision

Adopt R-078's sole `ardents-interactive-route-v1` application grammar:

- State-pinned mutual TLS 1.3 and exact ALPN precede every Route record;
- fixed binary LegBinding records authenticate only adjacent C-5 roles and
  State/attempt/expiry facts; and
- fixed binary SealedIntroduction records use standard-library X25519,
  HKDF-SHA256, and AES-128-GCM HPKE with the visible header as AAD.

The format has no legacy reader, generic map, optional/unknown field policy,
or Node-selected Profile negotiation. State/publication alone names supported
generations. Service Target/Instance material is encrypted to the Service and
never appears in the Introduction-visible header or a C-5 binding.

## Consequences

- M8 owns canonical codecs, public synthetic vectors, mutation/fuzz,
  reciprocal-peer, replay, mixed-generation, and downgrade tests;
- `routeplan`, H3 `AS*`/nested-TLS codecs, and their command readers are C0
  retirement inputs, not v1 compatibility paths; and
- a future overlapping generation needs its own accepted record/ADR under
  ADR-0006; no public compatibility or Qualification claim is made here.

## Compliance

[R-078](../research/records/r-078-interactive-route-v1-wire.md) contains the
exact bytes, source evidence, alternatives, and required M8 proof. This ADR
selects no public directory, service fallback, deployment profile, or
first-party cryptographic primitive.
