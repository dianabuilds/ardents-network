---
id: R-140
title: Does quic-go v0.62.0 preserve the maintained QUIC Carrier contract?
status: decided
owner: Product Owner and Codex
started: 2026-09-04
reviewed: 2026-09-04
---

# R-140 — Does `quic-go v0.62.0` preserve the maintained QUIC Carrier contract?

## Decision this unlocks

Select the exact maintained `quic-go` version after the direct dependency was
advanced from the `v0.61.0` version researched by R-094. The decision concerns
only the private QUIC-v1 Carrier implementation; it creates no new Route,
Carrier, migration, capacity, anonymity, or availability claim.

## Current contract

[ADR-0048](../../adr/0048-maintain-tcp-and-quic-carriers.md) retains one
State-selected TCP/TLS or QUIC-v1 Carrier. The QUIC profile exposes one ordered
stream, mutually authenticated TLS 1.3 and reciprocal LegBinding; it fixes a
1200-byte initial packet, disables datagrams and 0-RTT, and never chooses a
fallback. R-094 remains historical evidence for `v0.61.0`, not evidence for a
changed library version.

## Hypotheses

- **H1:** `v0.62.0` compiles on the selected Go toolchain and preserves the
  private API and bounded behavior used by the existing QUIC Carrier tests.
- **H2:** its API or transport behavior changes a fixed Carrier invariant, so
  the update must revert to `v0.61.0` or change the contract deliberately.
- **H0:** the release, license, source identity, or reachable-vulnerability
  review is unacceptable for a maintained direct dependency.

## Evaluation criteria

The exact tag must be immutable, signed, MIT licensed, and compatible with the
pinned Go toolchain. The root module must verify its checksum and stay tidy;
the actual `internal/route` QUIC tests must prove TLS peer binding, reciprocal
LegBinding, 1200-byte configuration, disabled optional semantics, cleanup, and
no fallback. A changed release does not inherit R-094's external loss/reorder,
MTU, NAT-rebinding, or host-resource qualification without rerunning those
tests against its exact artifact.

## Evidence plan

### Primary sources

- quic-go's [immutable, signed v0.62.0 release](https://github.com/quic-go/quic-go/releases/tag/v0.62.0),
  accessed 2026-09-04. It declares Go 1.26 as the breaking minimum and records
  its API and transport changes.
- The tag's [go.mod](https://raw.githubusercontent.com/quic-go/quic-go/v0.62.0/go.mod),
  accessed 2026-09-04, declares `go 1.26.0`.
- quic-go's [security policy](https://github.com/quic-go/quic-go/security/policy),
  accessed 2026-09-04, defines private vulnerability reporting.
- The tagged [MIT license](https://github.com/quic-go/quic-go/blob/v0.62.0/LICENSE),
  accessed 2026-09-04.

### Experiment

The current root module pins `github.com/quic-go/quic-go v0.62.0` and its
checksum. Run `go mod verify`, `go mod tidy -diff`, the `internal/route` test
package, and repository quality gates with Go 1.26.6. Do not make library types
part of a public interface or enable a new library feature merely because the
release offers it.

### Failure scenarios

Reject or roll back on incompatible Go/toolchain requirements, checksum or
license failure, a reachable vulnerability, changed TLS/LegBinding behavior,
0-RTT/datagram enablement, a 1200-byte MTU regression, fallback, incomplete
cleanup, or a failed exact-version network-profile qualification.

## Findings

- **Sourced fact:** the upstream v0.62.0 release is immutable and signed, is
  marked latest by upstream, requires Go 1.26 or newer, and adds stream
  priorities. Ardents does not call the new priority API.
- **Sourced fact:** the release tag declares `go 1.26.0`; the repository uses
  Go `1.26.6`, meeting that minimum.
- **Sourced fact:** upstream publishes a private path for reporting exploitable
  security defects. The selected tag retains the MIT license.
- **Measurement (2026-09-04):** `go list -m -json github.com/quic-go/quic-go`
  resolved exactly `v0.62.0` with checksum
  `h1:ZHDjCk5OacATwGvs8PWE97CTvX7AqZiVoW7++ZOXTf8=`.
- **Measurement (2026-09-04):** `go mod verify` passed, `go mod tidy -diff`
  produced no output, `go test ./internal/route -count=1` passed, and
  `make quick-check` passed. The route package's QUIC tests exercise exact TLS
  and LegBinding, an idle stream, disabled 0-RTT/datagrams, fixed initial
  packet size, authenticated failure abort, and no TCP fallback.
- **Limitation:** R-094's Linux loss/reorder, MTU-1280, selective-blocking,
  NAT-rebinding, and separate-host evidence was executed for `v0.61.0`. It is
  not attributed to `v0.62.0`; an exact candidate must repeat those named
  profile tests before a C0 audit freeze or host/capacity claim.

## Options

1. Revert to `v0.61.0`. It preserves the old evidence but discards the current
   reviewed release and keeps a stale direct dependency.
2. Pin `v0.62.0` and repeat the exact-version qualification before C0 freeze.
   Chosen.
3. Remove QUIC. Rejected: it changes the accepted two-Carrier contract rather
   than repairing version evidence.

## Recommendation

Pin `quic-go v0.62.0` as the maintained development baseline. Keep the Route
surface unchanged and treat the external carrier-profile matrix as a required
pre-freeze requalification, not an inherited result. Confidence is high for
the exact source/toolchain/local behavior, and low for unrerun external-network
behavior. The strongest contrary argument is that the missing exact-version
network matrix may reveal a transport regression.

## Disposition

**Decided.** `go.mod`, `go.sum`, the dependency register, and ADR-0048 pin
`v0.62.0`. R-094 remains immutable provenance for `v0.61.0`. Before a C0 audit
freeze, run and record the selected exact-artifact loss/reorder, MTU-1280,
selective-blocking, NAT-rebinding, cancellation/cleanup, resource, and
separate-host checks; otherwise the relevant claim remains withheld.
