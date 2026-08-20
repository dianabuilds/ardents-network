---
status: accepted
date: 2026-08-20
---

# ADR-0020 — Authenticate the current Namespace in each Network Epoch

## Context

Self-signed Name Records do not prove which root claim or later transition is
current. Returning a complete signed parent chain also exceeds the fixed
private-resolution response for a legal 127-label Service Name.

## Decision

Each current Network Epoch materialization signs one canonical statement that
binds the network, rule, epoch number and authenticated Epoch digest, cutoff,
current Record root/count, accepted transition root/count, and deterministic
rejection root/count. A resolution
response carries the exact signed current Record, its threshold-materialized
effective lineage summary, and one compact Merkle membership proof.

The installed Network Epoch authority set and threshold authenticate the
statement. The S6E1 verifier additionally receives the complete bounded corpus
and independently recomputes every root and lineage summary. Missing, stale,
mutated, below-threshold, or forked evidence fails closed.

## Consequences

- Resolver responses remain within 4096 bytes at the maximum legal Name depth.
- Rotation, recovery, Release, reclaim, and parent state are authenticated as
  current shared Namespace state rather than merely self-signed records.
- A captured Network Epoch threshold can censor or fork the entire Namespace.
  This is visible and fail-closed where evidence is available, but not removed.
- No second registrar, consensus system, storage engine, or cryptographic
  dependency is selected.

## Compliance

- [R-057](../research/records/r-057-current-namespace-materialization.md)
  freezes the statement, proof, corpus, and limitation contract.
- [ADR-0004](0004-authenticated-epochs-and-separated-control-roots.md) owns the
  Network Epoch trust root and captured-threshold limitation.
- [R-055](../research/records/r-055-stage-6-evidence-serialization.md) owns the
  independent development-evidence encoding and mutation requirements.
