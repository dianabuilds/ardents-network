---
status: accepted
date: 2026-08-27
supersedes: ADR-0024 (TCP-only Carrier set) and ADR-0025 (TCP-only Entry resolution clause)
---

# ADR-0048 — Maintain TCP/TLS and QUIC v1 behind one Carrier contract

## Context

H4-2 requires a failed or blocked adjacent transport to be replaceable without
giving a Node or Adapter authority to select a peer, downgrade a profile, or
continue an old attachment under different delivery semantics. TCP/TLS is the
maintained baseline. R-094 demonstrated that pinned `quic-go` QUIC v1 can
provide the same mutually authenticated TLS 1.3 state, reciprocal LegBinding,
ordered byte lane, bounded deadline, MTU-1280 configuration, and cleanup oracle.

## Decision

Maintain two adjacent-Node Carrier Profiles behind `internal/route`'s narrow
Carrier Interface:

- `ardents-carrier-tcp-tls-v1`;
- `ardents-carrier-quic-v1`, implemented with `quic-go v0.61.0`.

Both use the existing `ardents-interactive-route-v1` TLS ALPN and closed
LegBinding. QUIC disables 0-RTT and datagrams, exposes exactly one ordered
bidirectional stream, fixes `InitialPacketSize=1200`, sends bounded keepalive
inside the negotiated idle timeout so a healthy idle Carrier survives, and
keeps all QUIC types, connection identifiers, migration operations, and socket
details private. Graceful close sends the stream FIN and closes with success;
failed post-open authentication aborts the stream/connection with a nonzero
error.

`OpenNodeLeg` opens exactly one caller-supplied, already State-authorized
profile. It does not race, retry, infer, or fall back. H4-2D owns authenticated
profile selection and replacement as a new attachment; Service Connection
continues to own logical recovery.

Signed Node Record schema v1 remains readable and canonically means
`ardents-carrier-tcp-tls-v1`. Schema v2 signs one explicit Carrier Profile
immediately after the endpoint. State accepts only the two profiles above,
rejects unknown values deterministically, and projects the selected record and
candidate profile through its narrow Node-duty view. Rendezvous listens on its
own record's profile; Initiator and Responder open the selected Rendezvous
record's profile. A linked State successor withdraws the old duty and therefore
cannot silently change the Carrier of a live attachment.

## Consequences

- UDP blocking is a classified failed QUIC attempt, never permission to try
  TCP. TCP requires a separate State-authorized attempt.
- Node lifecycle events and terminal results include the selected Carrier
  Profile so an operator can attribute readiness, pressure, drain, and failure.
- Carrier attempts and authenticated byte-lane operations expose only the
  transport-neutral `stale`, `incompatible`, `unauthorized`, `canceled`,
  `timeout`, `unavailable`, or `closed` failure classes while keeping the
  transport-specific cause private.
- A version or dependency change repeats the R-094 security, MTU, cancellation,
  cleanup, and host-profile checks.
- This decision makes no path-migration, censorship-resistance, capacity,
  anonymity, or availability claim.

## Compliance

- [ADR-0024](0024-native-interactive-route-foundation.md) remains the Route and
  authentication foundation.
- [R-094](../research/records/r-094-hostile-network-carrier-profiles.md) is the
  dependency and behavior evidence.
