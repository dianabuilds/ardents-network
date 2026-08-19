---
status: withdrawn
date: 2026-08-19
withdrawn: 2026-08-20
---

# ADR-0013 — Withdraw the initial Stage 6 cryptographic suite

## Context

The original 2026-08-19 decision attempted to select Ed25519 for Name
Authority signing, BLS12-381 for Recovery Policy, OHTTP for resolver query
hiding, SHA-256 for commitments, and HKDF-SHA-256 for connection-only keying.
It also proposed `Signer`, `ThresholdVerifier`, and `QueryHider` interfaces.

Validation on 2026-08-20 found two decision-invalidating problems:

1. the named implementation, `golang.org/x/crypto/bls12381`, does not exist;
2. ordinary BLS aggregation plus aggregate verification does not define the
   accepted recovery protocol. R-003 requires `t` distinct scoped Recovery
   Authorities and prevents the current Name Authority from defeating recovery
   intended to survive its compromise. The original text instead assigned the
   threshold to the existing Name Authority holder and omitted participant
   setup, shares, identity, proof of possession, rogue-key defense, and recovery
   transcript binding.

The proposed Go file and concrete implementations were never created. Their
names described a possible future boundary, not maintained S6.0 architecture.

## Decision

Withdraw the original suite. ADR-0013 provides no Stage 6 import authorization
and no recovery security claim.

R-044 returns to `open` and must compare maintained implementations or an
explicit multi-signature policy against the complete Recovery Policy trust
model. The replacement decision must cover:

- Name Authority signing;
- distinct `t`-of-`n` Recovery Authorities, setup, replacement, and loss;
- participant authentication, duplicate/rogue-key rejection, domain separation,
  and cross-policy/generation replay;
- resolver query hiding and its fail-closed boundary;
- exact dependencies, versions, licenses, maintenance, audit history, and
  conformance vectors; and
- measured CPU, memory, latency, proof size, and retained state.

Ed25519, OHTTP, BLS, FROST, and individually authenticated threshold policies
remain candidates only. Existing R-026 evidence may be cited by the replacement
research, but it does not select the complete Stage 6 suite by inertia.

## Consequences

- S6.2 cryptographic role integration and S6.4 Recovery Policy remain blocked.
- No code may introduce a Stage 6 cryptographic primitive or dependency under
  ADR-0013.
- `internal/nameauthority/sig.go`, `Ed25519Signer`,
  `BLSThresholdVerifier`, and `OHTTPQueryHider` are not current architecture and
  must not be cited as existing implementations.
- A replacement that creates technology lock-in requires a new accepted ADR
  after R-044 reaches a supported recommendation.
- Maintained feasibility code may not be promoted or cited as Stage 6 evidence
  while the coding gate is closed.

## Compliance

- [R-044](../research/records/r-044-cryptographic-suite.md) is the open research
  record for the replacement.
- [R-003](../research/records/r-003-service-name-contract.md) and
  [CONTEXT.md](../../CONTEXT.md) remain authoritative for Recovery Policy and
  Recovery Authority semantics.
- Runtime dependencies must be reviewed in
  [dependencies.md](../development/dependencies.md) before `go.mod` changes.
