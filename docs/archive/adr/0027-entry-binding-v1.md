---
status: accepted
date: 2026-08-23
supersedes: ADR-0026 kind assignments only
---

# ADR-0027 — Bind each Entry Invite to a fresh TLS attempt key

## Decision

The User-to-Initiator leg uses R-079 EntryBinding. It carries the exact signed
R-077 Invite and the digest of a fresh per-attempt mTLS client public key. The
Initiator validates and atomically consumes the capability/key/attachment
tuple against current State before allocating a Route attachment.

EntryBinding replaces R-078 kind `1`; Node-to-Node LegBinding and
SealedIntroduction move to kinds `2` and `3`. User is never represented as a
State Node, and the fresh TLS key is not a User identity, Credential, Persona,
or durable authority.

## Consequences

M8 must revise its unannounced v1 codec and prove replay, key substitution,
State/Invite mismatch, and process cleanup paths. No legacy reader, direct
fallback, or compatibility promise is introduced.

## Compliance

This ADR contains the format, failure cases, and required test evidence.
