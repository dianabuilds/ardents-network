---
status: accepted
date: 2026-08-23
supersedes: none
---

# ADR-0028 — Use the native Service Connection v1 grammar

## Context

ADR-0024 selects native Route legs and endpoint TLS but does not define the
endpoint records needed to bind the exact Target/Instance and preserve one
logical connection across a fresh Route Attachment. R-082 retires the H3
`AS*`/domain-tag bytes with no compatibility observer.

## Decision

Adopt R-083's closed `ardents-service-connection-v1` grammar after fresh,
exact-Instance-pinned TLS 1.3 under the native Route ALPN. The only record
kinds are InstanceChallenge, InstanceProof, Continuity, Data,
Acknowledgement, and Terminal. All carry the exact native Profile through the
common envelope; ConnectionContext binds immutable connection facts and TLS
exporter-derived HMAC binds each attachment. No legacy reader, version
negotiation, direct fallback, or Node-selected profile exists.

## Consequences

- M9 creates `service/connection` as the sole owner of this grammar and
  recovery semantics, with synthetic vectors and substitution/replay/offset/
  terminal tests;
- H3 Service Connection frames, exporter labels, and domain tags are C0
  retirement inputs, not native aliases; and
- publication representation and Application IPC remain separate decisions
  and cannot extend this wire's authority.

## Compliance

This ADR contains the complete record rules, alternatives, failure cases, and
evidence plan. This decision introduces no first-party cryptographic primitive
or
public-network/Qualification claim.
