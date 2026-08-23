---
id: R-083
title: What exact native endpoint-to-endpoint connection framing preserves authenticated Target/Instance binding and bounded attachment recovery after H3 Service Connection bytes are retired?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-083 — Native Service Connection framing

## Decision this unlocks

Select the one unannounced native endpoint-to-endpoint record grammar that M9
uses for exact Service Target/Instance proof, attachment continuity, ordered
data acknowledgement, and terminal outcome. It must replace the H3 `ASCH`,
`ASPR`, `ASAT`, and `ASCF` records and associated H3 TLS exporter/domain tags.

## Current contract

R-076/ADR-0024 fix native TCP/TLS 1.3 Route legs, no direct Service fallback,
and Service Connection-owned recovery. R-078/ADR-0026 fix only Node-leg and
sealed-Introduction Route records; they do not define endpoint connection
records after opaque Route legs join. R-082 authorizes a C0 break for the H3
Service Connection bytes, but does not select their successor. The endpoint
must authenticate the exact current Service Target and Instance generation
before Application success, retain one logical ordered connection through a
bounded fresh Route Attachment, and fail terminally rather than downgrade.

Publication representation and local Application/Broker bytes have different
authority and observer questions; this record does not select either.

## Hypotheses

- **H1:** one closed binary native endpoint record grammar after fresh endpoint
  TLS 1.3 binds exact profile/network/Target/Instance/generation and carries
  fixed continuity and ordered data records.
- **H2:** endpoint TLS alone is enough, with no application record binding or
  canonical continuity grammar.
- **H3:** preserve or rename the H3 `AS*` records as a temporary adapter.
- **H0:** none of the choices preserves the required connection semantics
  without further product or cryptographic research.

## Evaluation criteria

The selected format must have one canonical bounded encoding and no legacy,
unknown-field, or peer-selected downgrade path. A Node must see only the
opaque endpoint TLS byte stream, never Service Target/Instance material in a
Route binding. Endpoint records must bind native Profile, Network identity,
exact Target, Instance public key/generation, immutable connection context,
fresh attachment exporter commitment, generation, offsets, acknowledgement,
and terminality. Malformed, replayed, substituted, stale, or lower-profile
records must fail closed without direct retry. The implementation must use
maintained standard-library primitives and yield independent synthetic vectors
and mutation tests.

## Evidence plan

### Primary sources

- R-001, R-002, R-004, R-032, R-076, R-078, R-082, ADR-0005,
  ADR-0006, ADR-0024, and ADR-0026, inspected 2026-08-23.
- RFC 8446 and Go `crypto/tls` documentation, previously sourced by R-076 and
  R-078: TLS authenticates and protects each endpoint byte stream but does not
  define Ardents' Target/Instance or continuity grammar.
- The current `internal/serviceconn` proof, TLS-exporter, continuity, and
  frame implementation, inspected 2026-08-23 as a C0 semantic
  characterization input only.

### Experiment

Implement the candidate solely under a new M9 owner with deterministic
endpoint and Route-attachment adapters. Exercise exact Target/Instance
authentication, every field substitution, old/H3/unknown record refusal,
sequential and overlapping attachment replacement, replay, offset/ack
violation, cancellation, terminal cleanup, and a full opaque Route journey
when M11 supplies the measured Node runtime.

### Failure scenarios

- TLS authentication succeeds for the wrong Target or stale Instance;
- a replacement attachment replays or rolls back bytes/acks;
- a Node learns endpoint Service material through Route framing;
- a malformed record allocates unbounded data or survives as a retryable
  lower-profile success; or
- terminal failure leaves an attachment, secret, or Application stream live.

## Findings

- **Inspection:** current H3 `ASCH`/`ASPR` prove Target and generation after
  endpoint TLS, while TLS certificate pinning alone authenticates only the
  Instance key.
- **Inspection:** current attachment continuity binds TLS exporter material,
  profile/network/Target/Instance facts, ordered offsets, and generation, but
  its H3 tags and frames have no observer under R-082.
- **Sourced fact:** TLS 1.3 protects an ordered stream after handshake but
  leaves application framing and context binding to the protocol designer.
- **Inference:** H1 is the only candidate that can retain exact endpoint and
  recovery semantics without treating retired H3 bytes as native syntax.

## Options

| Option | Preliminary disposition |
|---|---|
| Closed native endpoint grammar after TLS | Continue evaluating. It can retain semantic binding without a legacy reader. |
| TLS-only connection | Reject unless it supplies an equivalent explicit Target/generation and recovery proof; current TLS pin does not. |
| H3 `AS*` compatibility adapter | Reject under R-082. It reintroduces a retired reader without an observer. |

## Selected v1 format

Choose `ardents-service-connection-v1` as a closed endpoint grammar carried
only after fresh endpoint TLS 1.3. TLS is pinned to the exact current Instance
Ed25519 key, permits neither tickets/resumption nor alternate ALPN, and uses
the exact native Profile `ardents-interactive-route-v1`. A Node sees only this
TLS ciphertext after opaque Route legs join.

Every record is `"ardents-service-connection-v1\x00" || uint16(body-length)
|| body`. The body has `uint16(version=1)`, one closed kind byte, and the
exact ASCII native Profile. A decoder consumes the whole body, rejects all
unknown kinds/versions/profiles and lengths outside their kind bound, and has
no H3 or generic reader.

`ConnectionContext` is the SHA-256 digest of a canonical, domain-separated
concatenation of Network ID, Target, Instance key and generation, publication
digest, Candidate View, Isolation Context, Destination Binding, exact Profile,
and every Work Safety/recovery deadline. It is immutable for one logical
connection. The selected records are:

1. **InstanceChallenge**: Network ID, Target, Instance generation,
   ConnectionContext, and a fresh 32-byte nonce.
2. **InstanceProof**: the challenge digest and an Ed25519 signature by the
   exact Instance key over the canonical challenge record.
3. **Continuity**: role, attachment generation, send base/end, receive next,
   deterministic role/attachment nonce, ConnectionContext, TLS exporter
   commitment, and HMAC-SHA-256. Its key comes from `EXPORTER-ardents-service-
   connection-v1` and its domain tags use the same v1 literal. It is exchanged
   before data on every attachment.
4. **Data**: attachment generation, logical offset, uint16 payload length,
   and at most 16 KiB payload. This is an allocation/parser bound, not a
   product stream or workload ceiling.
5. **Acknowledgement**: attachment generation and acknowledged logical
   offset.
6. **Terminal**: attachment generation and final logical offset. It has no
   peer-selected error taxonomy or optional payload.

The User sends InstanceChallenge and receives InstanceProof before it gives
Application success. Both sides exchange Continuity in fixed client/server
order on every attachment. Data/ack/terminal are accepted only after that
exchange and only with the current generation/offset rules. A replacement may
use a fresh Route Attachment, but may not change ConnectionContext, lower the
Profile, replay an acknowledged range, or become a direct connection.

## Recommendation

Choose H1 with medium confidence. The strongest objection is that this is a
hard-to-reverse wire; its scope is deliberately only endpoint connection
framing, unannounced, vector-bound, and has no predecessor reader. Publication
and local IPC stay separate so this format cannot acquire their authority.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
ADR-0028 records the consequential wire selection. M9 may implement the new
owner, canonical vectors, and mutation/recovery tests; it must delete every
H3 Service Connection reader/writer in the same C0 cutover. No experiment
directory is retained.
