---
status: accepted
date: 2026-08-23
supersedes: ADR-0029
---

# ADR-0030 — Retire Update V0 as an unobserved test format

## Context

R-087 selected a C1 V1→V2 root migration to remove the Update evidence field.
The subsequent source audit found that this H3 tracer has no production root
initializer, parent selector, or root-switch owner: Update accepts only an
already initialized path and test fixtures create every root. Adding a C1
selector would select a bootstrap/installer surface excluded by R-064.

## Decision

Treat the V1 Update root and V0 command/result as C0 test representations.
Maintain the transaction/recovery behavior with V2 fixtures and manifests that
omit EvidenceNotice; reject V1 in maintained runtime. Retain exact V0 vectors
only in a C4 independent verifier and delete `cmd/ardents-release`. A later
selected bootstrap owns any real root migration under a new decision.

## Consequences

- no root reset, selector, installer, activator, or Custody writer is added;
- the bounded transaction's behavior and fault evidence remain maintained; and
- V0 reproduction cannot become a runtime compatibility path.

## Compliance

This ADR records the source audit, alternatives, and required V2/C4 evidence.
