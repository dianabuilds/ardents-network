---
id: R-049
title: Which maintained TUF-compatible Go verifier and H3 release profile meet ADR-0006?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-049 — Stage 7 release verifier and profile

## Decision this unlocks

Select the maintained verification Implementation and exact bounded metadata
profile for S7.1. The decision authorizes dependency review and one Release
Decision Module; it does not authorize download, package installation, update
activation, public release roots, or independent-supply claims.

## Current contract

[ADR-0006](../../adr/0006-separate-release-safety-from-protocol-transition.md),
[R-048](r-048-h3-stage-7-contract.md), and the
[Stage 7 lifecycle specification](../../development/stage-7-lifecycle-spec.md)
fix separate release/protocol machines, TUF-shaped authority, `3-of-5` ordinary
and `4-of-5` expiring emergency test mechanics, expiring Release Safety, exact
target identity, non-decreasing floors, distributor distrust, and three explicit
retrieval modes. H3 identities remain visibly project-controlled.

## Hypotheses

- **H1:** a pinned patched `theupdateframework/go-tuf/v2` client plus a narrow
  Ardents-owned profile can implement the full client workflow without exposing
  signing/repository administration or unneeded multi-repository input.
- **H2:** another maintained Go TUF 1.x implementation has a smaller safer
  closure and equal conformance/misuse resistance.
- **H0:** no maintained candidate fits; stop rather than implementing TUF or
  cryptographic verification ad hoc.

## Evaluation criteria

- TUF 1.x client-workflow conformance and canonical unrecognized-field handling;
- exact root/targets/snapshot/timestamp role and threshold behavior;
- root rotation, expiry, fixed update-start time, version rollback, freeze,
  fast-forward, mix-and-match, target length/hash, consistent identity, and
  delegation bounds;
- cache/path confinement under malicious metadata and symlink/reparse inputs;
- bounded files, bytes, roles, keys, signatures, delegation depth, fetches,
  retries, clocks, disk, memory, CPU, and errors;
- Go 1.26 compatibility, license, active maintenance, release/source identity,
  audit/advisory history, misuse resistance, dependency closure, vulnerability
  scan, and removal cost;
- injected byte/file fetch Adapter for offline/direct/private tests without
  giving it decision authority; and
- no first-party cryptographic primitive or TUF workflow reimplementation.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [TUF specification 1.0.36](https://theupdateframework.github.io/specification/latest/);
- [theupdateframework/go-tuf](https://github.com/theupdateframework/go-tuf),
  including v2 client/trusted-metadata Interfaces and legacy deprecation;
- [go-tuf releases](https://github.com/theupdateframework/go-tuf/releases); and
- [CVE-2026-24686 / GHSA-jqc5-w2xx-5vq4](https://github.com/theupdateframework/go-tuf/security/advisories/GHSA-jqc5-w2xx-5vq4),
  affecting multi-repository cache paths through v2.4.0.

### Experiment

Create `experiments/r-049-stage-7-release-verifier/` with a README before code.
Pin candidate source/module closure and run the upstream conformance suite plus
Ardents vectors through a bounded local fetch/cache Adapter. Use a fresh external
cache root per case. Measure binary/closure size, build/test time, allocations,
maximum input handling, and every dependency/advisory/license. Independently
classify outputs and verify zero writes outside the owned root.

### Failure scenarios

Below-threshold/duplicate signatures; root rotation gaps; old/future/expired or
mixed metadata; timestamp/snapshot executable authorization; target mismatch;
unknown critical fields; oversized/count/depth bombs; malicious target/repository
names; path traversal; symlink/reparse cache escape; corrupted local cache;
interruption during floor publication; wrong environment/network/platform; and
unavailable distributors.

## Falsification criteria

Before candidate execution, freeze one corpus and the following stop rules.
H1/H2 is falsified if any mandatory TUF/root/target/protocol case is
misclassified once; any candidate write escapes its owned cache root; a trusted
root changes after a gap, reuse, one-sided threshold, or failed durable publish;
or verification needs signing/repository administration, first-party
cryptography, cgo/unsafe, an incompatible license, or an unpatched called
high/critical advisory.

The maximum test envelope is `1 MiB` per metadata file, `8 MiB` aggregate
metadata, `32` roles, `64` keys, `64` signatures per role, delegation depth `4`,
`1,024` target descriptions, and `32` fetches. On the frozen weakest host, the
complete maximum-metadata decision MUST finish within `2 s` monotonic time and
`128 MiB` additional peak RSS. Every corpus case must match its precommitted
classification; exceeding any bound returns a bounded rejection. If no candidate
meets every conjunct, select O0 rather than relax a threshold after results.

## Findings

- **Sourced fact:** TUF secures trusted target retrieval but deliberately does
  not define package format or install files.
- **Sourced fact:** go-tuf v2 exposes metadata, trusted-metadata, updater, and
  fetcher seams and deprecates its legacy v0.7 design.
- **Sourced fact:** the 2026 multi-repository cache traversal advisory is patched
  only after v2.4.0; Stage 7 must pin a patched identity and test path confinement
  even if multi-repository input is excluded.
- **Assumption:** the current closure and client Interface fit Ardents limits.
  The experiment must measure this before a recommendation.

## Options

- **O1:** patched go-tuf v2 updater/trusted-metadata behind one Ardents Release
  Decision Module; no repository/signing or untrusted TAP-4 map surface.
- **O2:** another maintained conformant Go client with a smaller reviewed
  closure.
- **O0:** choose none; redesign Stage 7 release verification or stop.

## Recommendation

Run O1 and at least one credible O2 through the same corpus. Select only after
conformance, closure, advisory, bounds, and confinement pass. Popularity or CNCF
hosting alone is not sufficient. Confidence: medium before measurement.

## Disposition

- State: `open`; no dependency or release serialization selected.
- Required before S7.1 and before any `go.mod` change.
- A selection updates `dependencies.md`, the package map with real code, R-054
  schema inputs, and an ADR only if it changes consequential trust semantics.
