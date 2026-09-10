---
status: accepted
date: 2026-09-09
---

# Activate one data lane after an authenticated Rendezvous pair

## Context

The Product Owner approved the exact JOIN-to-data proposal on 2026-09-09.
[R-154](../research/records/r-154-join-data-lane-transition.md) records the gap:
ADR-0081 specifies the JOIN body and opposite-side pairing but does not define
how that operation creates an accepted data lane. Ordinary OPEN selects a real
next Node and cannot supply an unchecked self-hop at Rendezvous.

## Decision

Use the [JOIN transition](../technical/protected-route-protocol.md#rendezvous-join-data-lane-transition)
on a dedicated, independently admitted class-2 role TLS channel for each side.
HELLO/ADMIT/ACCEPT remain on lane 0. Exactly one JOIN operation 5 uses local odd
lane 1, without OPEN. Only an accepted fixed RESULT on that lane, matching the
local request nonce after opposite-side pairing, activates its framed data.

Retain the original 64-KiB receive window and independently admitted byte/time
reserve. Setup expires at the earlier JOIN deadline or ten seconds from its
reservation; pairing never extends the original data or Work Safety lifetime.
Existing BYTES/CREDIT/EOF/CLOSE carry Service TLS ciphertext and all accounting.
The pairing owner maps the two local lanes without copying their operation
nonces or lane identifiers between role TLS channels. Source JOIN and capsule
submission proceed concurrently after their tokens are durably marked.

This completes the closed candidate grammar. It adds no authority, token class,
identity, next-hop choice, dependency, protection mode or raw-stream fallback.
A candidate lacking this transition cannot participate in this data path;
existing floor-preserving artifact/profile adoption remains required.

## Consequences

Implement the actual pairing and framed data owners before claiming the full
text journey. Required evidence includes both Carriers, refusal before pairing,
wrong lane/nonce/profile/context, duplicate/third side, malformed padding,
setup expiry, data beyond setup but within original admission, bounded credit
and aggregate bytes, directional EOF, cancellation and joined shutdown.
Selection does not prove these behaviors, installed confinement, latency or
privacy qualification. Prior net.Pipe fixtures remain scoped component evidence.