---
status: accepted
date: 2026-08-29
supersedes: none
---

# ADR-0054 — Separate Functional Alpha transition contracts

## Context

H4-6A makes current alpha control inputs observable, but an inspectable catalog
alone does not state how Release Safety, Network Epoch, Compatibility, and
Namespace materialization transition or fail. Conflating them would make a
project alpha key look like unbounded permanent authority. Canonical Namespace
close/release/reclaim has no selected authenticated global owner.

## Decision

The current Functional Alpha declares four independent H4-6B contracts:
Release Safety, Network Epoch, Compatibility, and Namespace materialization.
Each has its own authority root, predecessor, freshness, rotation, revocation,
rollback floor, emergency action, user-visible failure, and retained evidence.
`ardents-control inspect-transitions` is a read-only projection of the existing
enrollment-pinned H4-6A inspection; it neither authorizes an Endpoint nor
changes any Release, State, Namespace, or Update root.

Namespace materialization is **not selected** for this profile. The result is
an explicit `not-selected` outcome: alpha control must not materialize, close,
release, reclaim, or administratively recover a canonical Name. Target Links
remain the complete current alpha path. A future Namespace choice needs a real
authenticated close and independently reviewable evidence.

An emergency may stop new work, terminate an unsafe build, or withdraw/drain a
State duty. It may not seize a Name, rewrite a live destination, silently
downgrade a Route Profile, or force executable installation.

## Consequences

- The catalog remains an index; Release, State, and Compatibility authorities
  do not collapse into it or into each other.
- `forged`, `stale`, `replayed`, `revoked`, `conflicting`, `withheld`, and
  `unavailable` inputs have explicit report outcomes and no fallback path.
- Functional Alpha makes no canonical Namespace or public-control claim.
- H4-6C still requires real independent custodians, builders, and auditors.

## Compliance

- ADR-0004, ADR-0006, ADR-0038, ADR-0043, and ADR-0053
- [R-123](../research/records/r-123-separated-alpha-transition-contracts.md)
- [Alpha control transition contract](../technical/alpha-control-transition.md)
