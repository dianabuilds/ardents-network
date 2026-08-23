---
id: R-093
title: Can a native Route leg carry opaque bytes across distinct hosts?
status: open
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-093 — Cross-host native leg

## Decision this unlocks

Decide whether the selected native Route wire has enough cross-host functional
evidence to justify designing a bounded peer-facing Route process suite. It
does not select a Node operating profile, public deployment, Docker live
profile, or Route Qualification.

## Current contract

ADR-0024 selects TCP legs protected by state-pinned mutually authenticated TLS
1.3. ADR-0026 fixes the `ardents-interactive-route-v1` ALPN and reciprocal
LegBinding. The maintained Route Module has no peer-facing runtime, and the
active live profile remains inactive. R-092 measures a synthetic loopback Node
profile and must not be repurposed as WAN evidence.

## Hypotheses

- **H1:** Two separately networked hosts can establish the exact synthetic
  TLS/ALPN and reciprocal LegBinding contract, carry a bounded opaque payload
  unchanged, and leave no active process or listener after termination.
- **H2:** A network boundary, certificate exchange, or binding verification
  prevents that exchange; a peer-facing runtime must not proceed without a
  redesign or an explicit rejected condition.
- **H0:** The tracer accepts a downgraded TLS/ALPN, substituted peer,
  nonreciprocal binding, corrupted payload, or malformed attempt.

## Evaluation criteria

One complete run has all of the following:

- TLS 1.3, exact `ardents-interactive-route-v1` ALPN, and the configured peer
  Ed25519 certificate key are observed on both sides;
- the client and server successfully verify reciprocal LegBinding records;
- the returned payload length and SHA-256 digest exactly equal the sent opaque
  payload; and
- a changed Attachment identifier is rejected after TLS and before payload
  acceptance.

The VPS must expose only the disposable tracer port and must not modify or
share credentials, volumes, networks, or containers with the existing
application deployment. Client hosts initiate the outbound connection; this
experiment makes no NAT traversal or inbound Endpoint claim.

## Evidence plan

### Primary sources

- ADR-0024 and ADR-0026, inspected 2026-08-24.
- `internal/route` LegBinding codec and reciprocal peer tests, inspected
  2026-08-24.
- The current testing and technical contracts, inspected 2026-08-24.

### Experiment

`experiments/r-093-cross-host-native-leg/` contains a standalone executable
with identity, server, and client roles. Each host creates an ephemeral local
Ed25519 certificate/key pair; only the public certificate is copied to its
peer. The server accepts a declared finite number of valid or deliberately
rejected connections and emits JSON evidence. The client emits its own JSON
evidence after sending random bounded bytes and verifying the echo.

Retain command transcripts, both JSON results, source commit, Go version,
binary SHA-256, host identity, public-certificate SHA-256, and firewall/port
facts outside Git. Never retain private keys, packet captures, or application
payloads in the repository.

### Failure scenarios

- wrong peer certificate, TLS version, or ALPN;
- changed Attachment identifier after successful TLS;
- truncated, corrupted, or mismatched echo;
- unreachable VPS listener, a blocked outbound client, or a listener left
  active after its finite run; and
- an attempt to treat co-resident containers or this single VPS as independent
  Node operators.

## Findings

- **Sourced fact:** the selected wire and its reciprocal LegBinding codec have
  maintained deterministic tests.
- **Measurement:** pending. No cross-host result exists at creation.

## Options

- Run the disposable tracer first, then create a peer-facing Route process
  design only if its required evidence passes.
- Treat the existing loopback-only harness as WAN evidence. Rejected: it does
  not traverse distinct host, firewall, NAT, or certificate-distribution
  boundaries.

## Recommendation

Run the named VPS-to-local experiment, including its binding-rejection case.
Confidence is high that it is a useful functional gate; its strongest
limitation is that one successful carrier leg cannot establish route topology,
privacy, independent operation, or production readiness.

## Disposition

Open. R-093 adds no ADR, maintained package, dependency, public protocol, or
hosting claim. Retain the disposable tracer only while it has a measurement or
falsification duty.
