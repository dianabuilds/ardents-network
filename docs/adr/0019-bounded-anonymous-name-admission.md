---
status: accepted
date: 2026-08-20
---

# ADR-0019 — Bound naming admission with scoped anonymous work

## Context

Naming surfaces need finite local abuse cost without money, accounts, IP
reputation, stable identity, wallets, tokens, or personhood claims. R-045 O1
failed every weaker-client solve-latency gate. Its separately predeclared O1b
profile passed accessibility, verifier, retained-state, replay, restart,
parallelism, and scope-binding gates.

## Decision

Accept R-045 O1b. A Node issues a short-lived HMAC-SHA-256 challenge bound to
its boot secret, identity, network/epoch, surface, operation digest, Isolation
Context, expiry, and fresh 16-byte nonce. One SHA-256 leading-zero proof is
single-use and local to that Node and context.

The exact `(work bits, spent cap, in-flight cap)` values are:

- exact-name resolution: `(16, 4096, 64)`;
- renewal/update: `(16, 2048, 32)`;
- policy/recovery: `(17, 1024, 16)`;
- root claim: `(18, 1024, 8)`.

TTL is at most 30 seconds. Full spent state and full in-flight state reject new
work immediately without eviction, unbounded queueing, or naming-state
mutation. Restart creates a new boot secret and invalidates old challenges.

## Consequences

- S6.5 may implement `Admission.Verify` at every selected naming surface.
- The weaker Linux solve p95 remained between 72.73 and 524.81 ms; verifier p95
  remained below 4.2 us and retained state below 1 MiB per surface.
- Specialized solvers can still exhaust a Node's finite window. This is a local
  amplification guard, not Sybil resistance, fairness, personhood, or
  anti-squatting.
- The profile uses only standard-library HMAC and SHA-256 and adds no dependency.

## Compliance

- [R-045](../research/records/r-045-anonymous-cost.md) freezes the failed O1
  result, accepted O1b profile, hostile corpus, and measurements.
- Anonymous Cost retains the limitations stated in the product contract and
  threat model.
- S6E1 may disclose only synthetic test boot secrets after worker termination;
  it is not a live key-export mechanism.
