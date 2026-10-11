---
status: accepted
date: 2026-08-20
---

# ADR-0018 — Authorize recovery with bounded individual signatures

## Context

Recovery must survive compromise of the current Name Authority and therefore
requires distinct scoped Recovery Authorities. Withdrawn ADR-0013 named a
nonexistent BLS package and did not define a threshold protocol. R-044 measured
an intentionally simple alternative using reviewed standard-library Ed25519.

## Decision

Accept R-044 O2. A generation-scoped Recovery Policy contains `2 <= t <= n <=
8` strictly ordered distinct Ed25519 Recovery Authority public keys, a monotonic
revision, and one visible delay from 72 hours through 30 days. The current Name
Authority cannot be a participant.

Initiation and cancellation each require at least `t` distinct signatures over
their separate domain, network, canonical Name, generation, effective policy,
operation identifier, successor, and fixed start/completion boundaries. Policy
add, replace, and disable transitions use the same visible delay while the
preceding policy remains effective. No BLS, FROST, DKG, aggregation, dealer, or
new runtime dependency is selected.

## Consequences

- S6.4 may implement `RecoveryPolicy.Authorize` and the delayed recovery state
  machine with standard-library Ed25519.
- Proof size and verification work grow linearly but remain bounded; the
  measured 5-of-8 case used 1,248 logical bytes and 0.404001 ms Linux p95.
- Membership and signer participation are visible. Participant independence,
  custody quality, and recovery availability are not claimed.
- ADR-0013 remains withdrawn; ADR-0014 continues to own S6.2 query hiding.

## Compliance

- The glossary owns the Recovery Policy trust model.
- No `go.mod` or dependency-register change is authorized by this decision.
