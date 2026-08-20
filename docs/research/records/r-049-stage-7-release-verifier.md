---
id: R-049
title: Which maintained TUF-compatible Go verifier and H3 release profile meet ADR-0006?
status: review
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
  affecting multi-repository cache paths through v2.4.0;
- [TUF implementation list](https://theupdateframework.io/docs/getting-started/)
  and [tuf-conformance v2.4.0](https://github.com/theupdateframework/tuf-conformance/tree/v2.4.0);
- [DataDog/go-tuf v1.1.1-0.5.2](https://github.com/DataDog/go-tuf/tree/v1.1.1-0.5.2); and
- [X41 go-tuf assessment](https://x41-dsec.de/static/reports/X41-go-tuf-Audit-2023-Final-Report-PUBLIC.pdf),
  which covers an older 2023 revision and excludes dependencies.

### Experiment

The build-ignored harness in
[`experiments/r-049-stage-7-release-verifier/`](../../../experiments/r-049-stage-7-release-verifier/)
pins candidate source/module closure and runs upstream conformance plus Ardents
vectors through a bounded byte Adapter. Clones, caches, generated repositories,
binaries, and raw output stay in fresh owned system-temporary roots outside Git.

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
  fetcher seams, is the official list's Go reference implementation, and
  deprecates its legacy v0.7 design as difficult to maintain and error-prone.
- **Sourced fact:** the 2026 multi-repository cache traversal advisory is patched
  only after v2.4.0. O1 pins `v2.4.2` at
  `f5edbde31e5507f46db2069402dc38903fe6d9d4` and excludes multi-repository,
  local-cache, and repository-name input.
- **Sourced limitation:** X41's 2023 review found one medium repository symlink
  issue and one low cross-repository key-reuse issue, but it reviewed an older
  commit, excluded dependencies, and does not establish current client safety.
  The selected surface imports no repository/signing administration and gives
  every release authority an independent root.
- **Measured:** the exact O1 tag passed its upstream suite on Windows Go 1.26.6
  and Linux Go 1.26.5. Its official CI compatibility setting
  `GODEBUG=rsa1024min=0` is needed only by old upstream RSA test fixtures; the
  Ardents synthetic corpus uses Ed25519 and the runtime profile does not relax
  Go cryptographic policy.
- **Measured:** `tuf-conformance v2.4.0` at
  `500c525c9ce287a472fd334fe8d885cace667d32` classified all `108` client cases
  correctly against exact O1, with no xfail. The corpus covers threshold/root
  rotation, expiry, rollback/fast-forward, length/hash, unusual role names, and
  delegation graphs. Profile-specific stricter rejection is tested separately.
- **Measured:** the Ardents profile passed ten repetitions on Windows and ten
  no-cgo Linux repetitions with `--network=none`, one CPU, a `128 MiB` hard
  memory limit, read-only root, `64` PIDs, and all capabilities dropped. The
  conservative Linux test duration was `0.36–0.51 s` per run and total process
  `VmHWM` was at most `21,360 kB`. Three three-second Windows runs placed the
  full-decision benchmark at `35.52–41.29 ms/op`, at most `25,578,380 B/op` and
  `304,974 allocs/op`; shape plus artifact verification was at most
  `0.428 ms/op`, `464 B/op`, and `7 allocs/op`.
- **Measured:** the profile caps each metadata object at `1 MiB`, aggregate
  metadata at `8 MiB`, fetches at `32`, root rotations at `16`, roles at `32`,
  keys and signatures at `64`, and targets at `1,024`. It disables delegated
  targets, local cache, unsafe-local mode, network ownership, and every
  repository/signing Interface. The precommitted delegation depth `4` was a
  ceiling, not an acceptance requirement; Stage 7 needs one top-level release
  role, so the selected ceiling is the stricter `0`.
- **Measured:** trusted-metadata construction captures one UTC `RefTime` and all
  expiry checks in that updater use it. Maintained code must not call the
  exported test-only `UnsafeSetRefTime` seam.
- **Boundary finding:** disabling candidate cache also disables its cross-run
  metadata history. That cache cannot be a security watermark: its atomic
  rename is not the Stage 7 durable floor transaction, and deletion must not
  enable rollback. The Release Decision Module therefore supplies the installed
  trusted root plus Ardents-owned `version + digest` floors for root, timestamp,
  snapshot, and top-level targets; lower versions and same-version/different-
  digest inputs are `release-invalid`.
- **Measured:** the compiled client path contains `12` modules and `55`
  non-standard-library packages, requires no cgo, and produced a conservative
  `16,514,037`-byte Linux no-cgo test binary. All module licenses are
  Apache-2.0, MIT, or BSD-3-Clause. Third-party `x/sys`/protobuf internals use
  `unsafe`; Ardents code does not, and no first-party unsafe or cgo exception is
  needed.
- **Measured:** `govulncheck` found no called vulnerability. Raising O1's
  declared `x/crypto v0.50.0`, `x/sys v0.43.0`, and `x/term v0.42.0` to
  `v0.52.0`, `v0.45.0`, and `v0.43.0` preserved the upstream suite and ten
  profile repetitions. The raised client scan found zero symbol/package
  vulnerabilities and one unimported module-only `x/crypto/openpgp` advisory.
- **Measured rejection:** O2 pins the legacy API and much older dependency
  versions. On current Linux its client suite produced `81 passed, 5 failed`,
  and its file store produced `12 passed, 1 failed`; failures include expired
  fixed fixtures and platform assumptions. It has no current compatible
  conformance adapter or evidence equal to O1. Test decay alone is not a runtime
  vulnerability, but it falsifies the reproducible-maintenance criterion.

## Options

- **O1:** patched go-tuf v2 updater/trusted-metadata behind one Ardents Release
  Decision Module; no repository/signing, cache, or untrusted TAP-4 map surface.
- **O2:** DataDog's maintained legacy fork. Rejected because the deprecated
  design, decayed suite, old closure, and absent current conformance evidence do
  not meet the conjunctive criteria.
- **O0:** choose none; redesign Stage 7 release verification or stop.

## Recommendation

Select **O1** in review: `github.com/theupdateframework/go-tuf/v2 v2.4.2` at the
recorded commit, with the raised three-module MVS set above, behind exactly one
Release Decision Module. Freeze the measured caps, one top-level targets role,
`DisableLocalCache=true`, `UnsafeLocalMode=false`, injected bounded bytes, exact
target identity, and target length/hash verification. Do not expose signing,
repository administration, multi-repository maps, delegated targets, ambient
networking, or cache paths.

Before returning `release-accepted`, the Module must compare all four verified
metadata identities with durable floors, retain the exact root bytes captured
by its byte Adapter, and durably publish the complete verified consecutive root
chain plus the successor floors. A failure publishes neither a release decision
nor a partial floor transaction. R-054 owns the canonical serialization; this
record fixes the required semantic inputs and makes every cache disposable.

At S7.1 integration, repeat checksums, license inventory, upstream tests,
conformance, no-cgo build, and current reachable scan against the complete root
module before merging. Any decision mismatch, reachable unpatched
high/critical advisory, or need to widen this surface selects O0. The closure is
removed with the one Release Decision Module and does not affect stored release
formats. Confidence: high for the bounded client selection; no claim is made
about release-key custody, installer safety, independent supply, or public
release operation.

## Disposition

- State: `review`; O1 and the exact bounded profile await Product Owner
  acceptance. S7.1 maintained coding remains closed.
- The proposed closure is preregistered in `dependencies.md`; acceptance does
  not by itself authorize a `go.mod`, package-map, or maintained-code change
  before the Stage 7 entry gate.
- No new ADR is proposed: O1 implements ADR-0006 without changing its trust
  semantics. Widening roles, delegation, cache, multi-repository input, or
  release authority would reopen R-049 and may require a superseding ADR.
