---
status: accepted
date: 2026-09-26
---

# ADR-0099 — Retire the superseded portable Run pump and relocate the replacement crash seam

## Context

Two deadcode-allowlist groups shared the classification "unwired bounded
process tracer": `internal/endpoint/portable.Run` with its private `emit`
helper, and `internal/endpoint/replacement.replaceWithInterruption`. Both
carried the retirement condition "Wire it into an accepted command
contract or remove it with the superseding process-candidate decision."

The audit found two different situations:

- `portable.Run` was a foreground-lifecycle convenience pump (Starting →
  Open → Ready → Wait → Stopped) with zero production callers. The
  accepted command composition in `cmd/ardents/endpoint.go` supersedes
  it: the enrolled flow drives `portable.Open`, its release-decision
  gating, its own stdout event encoding, and `running.Wait(ctx)`
  directly, because `Run` cannot interleave the release gate or the
  encoded events. Its only exercise was a test of the wrapper's own
  event choreography.
- `replaceWithInterruption` was not a superseded candidate but a
  test-only crash-boundary seam: it passes a non-nil `operationControl`
  into the live `replace` engine so a test can stop the real transaction
  just after one durable journal checkpoint. Its consumer,
  `TestReplaceInterruptionLeavesOnlyExplicitRecoveryPaths`, is the only
  end-to-end oracle proving that an interrupted live `Replace` leaves
  program bytes and journal evidence from which the production
  `Recover` returns exactly the documented safe next action
  (`keep-current`, `self-test-required`, `committed-restart-permitted`).

## Decision

Retire the pump, relocate the seam:

1. `Run` and `emit` are deleted from `internal/endpoint/portable`.
   `Open`, `Runtime.Wait`, `Runtime.Attachment`, and `FailureEvent`
   remain with their live command callers; `TestRunReportsStartingReadyAndStopped`
   is deleted with the wrapper it exercised.
2. `replaceWithInterruption` moves from `operation.go` to
   `operation_test.go`, beside its only consumer. The production
   checkpoint machinery inside `replace` (`operationControl`, the four
   `interrupted(...)` branches, `errOperationInterrupted`) stays: it is
   the injected dev-support seam that keeps every file, journal, unit,
   and self-test operation real, and production `Replace`/`Rollback`
   keep calling `replace` with a nil control.
3. The guard
   `internal/architecture/endpoint_process_tracer_retirement_test.go`
   pins the absence of the pump, the retained portable declarations, the
   live command Open/Wait wiring, the seam's absence from production
   `operation.go`, and its presence with the `Recover` oracle in the
   test file.
4. Both allowlist groups are dissolved: common symbols 355 → 352,
   windows-amd64 559 → 556, linux-amd64 356 → 353.

## Consequences

- One portable foreground composition remains — the accepted command
  flow — so lifecycle event choreography cannot diverge between an
  exercised wrapper and the real composition.
- The replacement crash-boundary evidence survives intact and now lives
  honestly in test code instead of masquerading as an unwired production
  candidate.
- The F-26 portable-profile contraction (created directories versus
  actual first-enrollment and current-program owners) remains a separate
  open card; this ADR only removes the `Run` facade from its inventory.
