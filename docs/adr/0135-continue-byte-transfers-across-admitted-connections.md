---
status: accepted
date: 2026-11-28
---

# Continue byte transfers across admitted connections

## Context

[ADR-0134](0134-use-one-application-byte-stream.md) selects one byte stream for
every file. The Product Owner removed the fixed 4 MiB product ceiling for
ordinary transfers and publication. Today the local Connection client
also gives the Reader exchange 60 seconds. The logical Connection binds
its original Work Safety deadline and byte counters across transport recovery.
A dedicated Rendezvous JOIN uses its original finite admission reserve.
[The current Route grammar](../technical/protected-route-protocol.md#rendezvous-join-data-lane-transition)
admits one JOIN per role channel and reserves replenishment for forwarding
parents. [Issue #456](https://github.com/dianabuilds/ardents-network/issues/456)
records the earlier JOIN refill question.

Increasing the application constant would still leave an aggregate size
ceiling. Changing JOIN replenishment alone would still leave the original
logical deadline and the whole-file allocation in the publication path.

## Decision

- Keep one locally authorized byte transfer from the caller's first byte to
  its final result. Internally it may use successive independently admitted,
  finite Service Connections. Each Connection keeps its original time,
  byte, and recovery bounds. Route, Admission and Hosting keep their current
  authority and accounting. A subsequent Connection requires a fresh
  admission under current authority and the same local user's continuing
  explicit operation.
- Bind each continuation to the original authenticated Target, Instance,
  publication identity, directional EOF state and exact next offsets at both
  endpoints. Use fresh endpoint-only continuation material established
  inside the original Service TLS. A continuation request consumes one
  exact next transition, bound to the new authenticated Service Connection.
  The peer confirms it inside Service TLS before any segment payload reaches
  the Application. Retain only bounded pending bytes and continuation state.
  A planned transition requires a confirmed boundary and joined retirement
  of the previous Connection. Ambiguous completion beyond the Connection's
  existing recovery contract interrupts the transfer.
- Keep directional EOF as an application effect of the original transfer.
  Segment closure has its own control semantics. It cannot signal
  application EOF or authorize success. Report success only after both
  directions reach authenticated terminal completion and join cleanup.
  Withdrawal stops new segment admissions and preserves the existing bounded
  drain for an already admitted Connection. Cancellation, revoked local
  authority or exhausted resources end the transfer and join all owned work.
  Process restart ends this first edition's transfer.
- Stage file publication from a stable local read into an immutable,
  owner-private source under a finite local storage reservation. Admit and
  count staged data before publishing the link; read and transmit it in
  bounded portions. Retire the source after withdrawal and original readers
  have joined. A changing file, failed reservation, or incomplete stage
  refuses publication. File content uses the same byte transfer as CLI input.

## Continuation boundary

The Reader initiates each subsequent segment. Either endpoint can request a
boundary before its segment budget is exhausted. Stop admitting new bytes to
that segment, drain already admitted bytes, and confirm both final positions
and directional EOF states before retiring it. The local Application stream
keeps these positions across segments and applies backpressure while a
continuation is pending. The next Connection starts its own accounting at
zero. The Publisher consumes the pending transition only after fresh
authentication and a match against the retained absolute positions. Neither
side releases bytes from the next segment to the Application before the
transition is confirmed.

Use the existing Connection recovery for failures within a segment, including
its final control exchange. Failure to confirm that boundary ends the
transfer. Each endpoint owns the cleanup of its original segment before it
hands off. A new segment cannot renew the previous segment's deadline or
recover bytes after the local transfer ends.

## Evidence required before implementation and acceptance

This decision selects the continuation architecture. Implementation starts
from the verified unified byte path under ADR-0134. Before implementing the
wire or staging changes, specify the exact protected continuation records,
the transfer owner's lifetime and resource policy, and the storage reservation
and crash cleanup contract. Review role-visible metadata during successive
admissions against the [threat model](../security/threat-model.md).
Continuation state belongs at endpoints and must stay out of Route-visible
identifiers and diagnostics. Test deployed artifact compatibility before
selecting the cutover.

Qualify a real Ubuntu TCP/TLS pair in both directions above 4 MiB and above
one original JOIN allowance, including a transfer lasting longer than the
current Reader window. Require byte-exact output, one authenticated outcome,
bounded memory, and reconciled admission and Hosting accounting. Exercise
simultaneous traffic across a transition and EOF in one direction before it.
Inject wrong or duplicate continuation, interruption before and after the
boundary, revocation, exhausted authority and failed staging. Verify no
extra Application effects and joined physical cleanup after every refusal.
The earlier installed receipts apply to their original artifacts.
