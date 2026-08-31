---
status: accepted
date: 2026-08-26
---

# ADR-0042 — Bind the accepting alpha-control command to enrollment v3

## Context

ADR-0041 lets a participant accept a signed Alpha Name Corpus only after the
enrolled artifact and independent control evidence pass verification. A
manifested `corpus.pub` alone does not authenticate the executable that writes
the persistent corpus floor: an arbitrary separately obtained program could
present the same command-line interface.

## Decision

Preserve enrollment v1 and v2 verification for their existing non-acceptance
uses. Define `ardents-closed-alpha-enrollment-v3` for the corpus-acceptance
route. It retains the v2 `corpus_authority=corpus.pub` companion and adds the
canonical package-owned `control_artifact` name for `ardents-control` on the
declared platform. This is `ardents-control-<platform>` on non-Windows
platforms and `ardents-control-<platform>.exe` on Windows; descriptor parsing,
bundle construction, and running-companion verification use the same naming
contract.

The entry is an ordinary, separately manifest-pinned bundle file, never a
Release metadata input. Before `accept-alpha-corpus` reads ACA2 or corpus
bytes, the command must verify the v3 enrollment and show that its own running
file is the exact named entry by same-file and byte comparison. A v1 or v2
bundle, a renamed command, a copy outside the bundle, or changed bytes fails
closed.

## Consequences

- The alpha corpus procedure has one explicit executable provenance boundary;
  it does not trust a file merely because its name or flags look right.
- A v3 bundle needs both Endpoint and control binaries in its exact inventory.
  This is an alpha procedure requirement, not an installer, updater, or code
  signing system.
- Existing v1/v2 endpoint enrollment remains readable where it is already
  specified, but it cannot be used to accept an alpha corpus.
- The decision authenticates delivered local bytes only. It does not select a
  publication source, make a release page authoritative, start an Endpoint,
  establish a browser address, or grant canonical Namespace authority.

## Compliance

- [R-113](../research/records/r-113-alpha-corpus-distribution-floor.md) owns
  the procedure evidence and the remaining source-provenance gate.
- [ADR-0041](0041-alpha-control-corpus-component-v2.md) remains authoritative
  for the four-component ACA2 contract.
- [ADR-0040](0040-bounded-alpha-name-overlay.md) remains authoritative for
  the non-canonical alpha naming boundary.
