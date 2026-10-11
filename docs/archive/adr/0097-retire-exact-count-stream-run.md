---
status: accepted
date: 2026-09-26
---

# ADR-0097 — Retire the exact-count Stream.Run pump in favor of the live bounded successor

## Context

The cite-or-die sweep (ADR-0091) retained the deadcode-allowlist group
"unwired service-stream tracer": `internal/service/connection.Stream.Run`,
`Stream.receiveApplication`, `Stream.sendAcknowledgements`,
`Stream.sendApplication`, and `Stream.sendTerminal`. The group's condition
was to connect it through an accepted service command contract or remove it
with the superseding candidate decision.

The audit found the superseding decision already accepted and wired:

- The only production driver of a native Service Connection stream is the
  Linux text Endpoint runtime (`internal/endpoint/text_service_stream.go`),
  and it calls `RunBounded`. The bounded pump has its own exclusive members
  (`sendApplicationBounded`, `receiveApplicationBounded`,
  `sendBoundedAcknowledgements`, `finishBoundedSend`,
  `startTerminalTail`) and treats local Application EOF as the
  authenticated Terminal record — a normal half-close rather than an
  exact-workload failure.
- The exact-count `Run` pump had zero non-test callers. Its only exercise
  was one package test. The c0 reconstruction row for
  `stream_lifecycle.go` already recorded that `Run` implements exact-count
  work with no `cmd`/`internal` caller while its file-neighbors serve the
  live bounded path, and asked for the retirement decision before deletion.
- No accepted C0 command starts the standalone exact-workload service
  candidate; the reconstruction plan's service-connection rows keep the
  bounded half-close stream as the accepted owner behavior.

The shared lifecycle and I/O primitives around the pump are live through
`RunBounded`, recovery, replay, and the terminal tail:
`establishInitialAttachment`, `outcome`, `watchNameOrigin`, `attachment`,
`fail`/`failLocked`/`releaseFailure`, `close`, `sendQueueBlockedLocked`,
`flushAvailable`, `writeRecord`, `acknowledgeLocked`, `acceptData`, and
the received-range/recent-history helpers.

## Decision

Retire the exact-count pump as superseded:

1. `Stream.Run` is deleted from `stream_lifecycle.go`; the file keeps the
   shared lifecycle methods that serve `RunBounded`.
2. `sendAcknowledgements`, `sendApplication`, and `sendTerminal` are
   deleted from `stream_send.go`; `receiveApplication` is deleted from
   `stream_receive.go`. The bounded send/receive/acknowledgement members
   and all shared serialization primitives stay.
3. `TestStreamExchangesInitialContinuityBeforeBidirectionalData` now
   drives the same initial-continuity-before-data behavior through
   `RunBounded` with directional half-close, matching the accepted
   runtime's completion semantics.
4. The guard
   `internal/architecture/service_stream_run_retirement_test.go` pins the
   absence of the five retired members and the presence of the bounded
   successor and shared primitives, including the live Endpoint
   `RunBounded` wiring.
5. The allowlist group "unwired service-stream tracer" is dissolved:
   common symbols 362 → 357, windows-amd64 566 → 561, linux-amd64
   363 → 358.

## Consequences

- The package map no longer claims support for "exact declared workloads";
  the native Service Connection stream supports only orderly half-close
  within the caller's directional byte bounds.
- One pump implementation remains, so recovery, replay, terminal-tail and
  queue-bound behavior cannot diverge between an exercised and an
  unexercised entrypoint.
- If a future accepted command ever needs exact-count workload semantics,
  it arrives with its own contract and ADR rather than inheriting an
  unwired tracer.
