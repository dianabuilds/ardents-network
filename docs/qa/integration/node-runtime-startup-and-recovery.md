# Scenario NRI-001

- `Layer`: `integration`
- `Domain`: `Node Runtime`
- `Category`: `startup / shutdown / recovery`

## Goal

Confirm that `Node Runtime` preserves explainable startup and shutdown truth, restores
pending operations after restart, compacts oversized diagnostics ledgers back to bounded
truth, and keeps startup failures operator-visible.

## Preconditions

- the canonical orchestrator is the local runtime path in `internal/node`;
- the scenario can write persisted state into an isolated temp dir;
- bootstrap uses `local://bootstrap` so the scenario stays focused on single-node runtime
  truth instead of network timing.

## Steps

1. Prepare persisted `operations.json` with an unfinished recoverable operation.
2. Start `Node` from the same data dir and read `PendingOperations()`.
3. Separately start and gracefully stop `Node`, then read the persisted operations ledger.
4. Prepare an oversized persisted `operations.json` with many closed operations plus one
   open recoverable operation, start `Node`, stop it cleanly, and read the rewritten ledger.
5. Prepare a corrupt `ardents.db`, start `Node`, and read diagnostics after the explainable
   startup failure.

## Expected Result

- the pending operation becomes `recovering` after restart and stays operator-visible;
- graceful shutdown persists `node.shutdown` in terminal `completed` state;
- an oversized diagnostics ledger is compacted to a bounded closed-operation tail while
  keeping open or recovering truth operator-visible;
- a corrupt startup path returns an explicit error with structured diagnostics reason
  `node.state.load_failed`.

## Failure/Degraded Variant

- if restart hides the unfinished operation, recovery truth is lost;
- if shutdown does not persist a terminal fate, recovery semantics become unprovable;
- if oversized ledgers are never compacted, restart cost and recovery noise grow without
  bound;
- if corrupt startup finishes without a structured reason, runtime stops being explainable.

## Related Tests

- `tests/integration/node/node_startup_test.go::TestNodeStartDegraded`
- `tests/integration/node/node_startup_test.go::TestNodeFailsWhenStateLoadIsCorrupt`
- `tests/integration/node/node_startup_test.go::TestNodeStopReturnsErrorWhenShutdownFails`
- `tests/integration/node/node_startup_test.go::TestNodeRejectsAuthoritativeMutationsWhenFailed`
- `tests/integration/node/node_startup_test.go::TestNodeStartRollsBackRuntimeWhenBlobExchangeStartupFails`
- `tests/integration/node/node_startup_test.go::TestNodeStartRollsBackRuntimeWhenCallerContextCancelsDuringBlobExchangeStartup`
- `tests/integration/node/node_startup_test.go::TestNodeStartupPhasesPersistAsCompletedOperations`
- `tests/integration/node/node_startup_test.go::TestNodeShutdownPhasePersistsAsCompletedOperation`
- `tests/integration/node/node_identity_test.go::TestNodeRestoresPersistentState`
- `tests/integration/node/node_identity_test.go::TestNodeRestoresIdentityAcrossRestart`
- `tests/integration/node/node_identity_test.go::TestNodeStoresPrivateKeyOutsideGeneralState`
- `tests/integration/node/runtime_recovery_test.go::TestNodeRuntimeRecoveryShowsPendingOperationAfterRestart`
- `tests/integration/node/runtime_recovery_test.go::TestNodeRuntimeShutdownPersistsCompletedOperation`
- `tests/integration/node/runtime_recovery_test.go::TestNodeRuntimeRestartCompactsClosedOperationsLedger`
- `tests/integration/node/runtime_recovery_test.go::TestNodeRuntimeStartupFailureRemainsExplainable`

## False Positive Risk

- a test that only checks for absence of panic can miss persisted-ledger drift;
- a test that checks only record count can miss loss of the recovering operation.

## False Negative Risk

- the scenario must not depend on live network connectivity or external Waku timing;
- assertions must avoid depending on exact ordering of unrelated operations beyond the
  required bounded-tail and shutdown facts.

## Notes

- this remains a single-node runtime integration path; multi-node transport and discovery
  behavior are covered by dedicated domain scenarios.

Persistent-state security coverage additionally proves that a stopped data
directory backup restores the same Ardents principal/device and that restoring
`ardents.db` without its matching `identity_key.json` fails closed without
overwriting the retained identity record.
