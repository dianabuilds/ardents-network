---
status: accepted
date: 2026-09-06
supersedes: ADR-0028
---

# ADR-0075 — Use Service Connection v2 Terminal receipts

## Context

The closed v1 grammar proves that a local carrier accepted a Terminal write,
but not that the peer consumed it. A recovered logical connection must retain
the directional close until peer receipt, including when a replacement
Attachment cuts over immediately after the local write. Reusing the v1
Acknowledgement bytes for that receipt would silently change a fixed grammar
and make mixed implementations disagree about orderly close.

## Decision

Replace the unannounced `ardents-service-connection-v1` envelope with the
closed `ardents-service-connection-v2` envelope (version 2). The record set
remains InstanceChallenge, InstanceProof, Continuity, Data, Acknowledgement,
and Terminal. A v2 Acknowledgement has either its fixed 16-byte data receipt
form or a fixed 17-byte Terminal-receipt form whose final byte is exactly
`1`. The Terminal receipt names the same Attachment generation and offset and
is accepted only for a local Terminal being written or already written on
that generation and offset.

v2 has no v1 reader, compatibility mode, negotiation, profile fallback, or
peer-selected grammar. A C0 deployment changes both endpoints together; a
v1 peer fails explicitly at the envelope boundary. On a recovery-enabled
stream, local completion and terminal replay use the v2 Terminal receipt. A
headless stream has no replacement source and retains its existing orderly
half-close behavior rather than inventing recovery.

## Consequences

- `internal/service/connection` remains the sole grammar and receipt owner;
- a received Terminal is acknowledged separately, and stale acknowledgement
  writes cannot mark a replacement Attachment as acknowledged;
- carrier loss during a serialized write closes the failed Attachment so the
  writing worker can execute the one bounded recovery episode; and
- v1 bytes remain historical compatibility evidence only, not a maintained
  alternate parser or runtime path.

## Compliance

The change is limited to the selected C0 closed grammar. It introduces no new
record kind, dependency, cryptographic primitive, public-network claim, or
fallback route. Tests cover strict record parsing, normal headless close, and
continuity recovery with Data-before-Terminal replay.
