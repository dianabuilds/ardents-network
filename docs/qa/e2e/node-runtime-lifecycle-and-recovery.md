## Scenario ID

`NRE-001`

## Layer

`e2e`

## Domain

`Node Runtime`

## Category

`startup / shutdown / restart / recovery`

## Goal

Prove that an operator-visible node runtime flow keeps startup, shutdown,
restart, and pending-operation recovery truthful through the canonical local
control surface.

## Preconditions

- the node runs from an isolated temp data dir;
- bootstrap uses `local://bootstrap` so the scenario stays focused on runtime
  truth instead of network timing;
- the temp data dir is pre-seeded with one unfinished recoverable operation.

## Steps

1. Seed `operations.json` with a recoverable startup-owned operation.
2. Start the node and read runtime status plus diagnostics/pending operations
   through the local control surface.
3. Stop the node and inspect the persisted diagnostics ledger.
4. Restart the node from the same data dir.
5. Read runtime status plus diagnostics/pending operations again through the
   local control surface.

## Expected Result

- startup succeeds and the node remains operator-visible as running/ready;
- the seeded unfinished operation is projected as `recovering` through the
  diagnostics surface after startup and after restart;
- graceful shutdown persists `node.shutdown` with terminal state
  `completed`;
- restart preserves recovery truth instead of silently dropping the pending
  operation.

## Failure/Degraded Variant

- if startup hides the unfinished operation, recovery truth is lost;
- if shutdown does not persist terminal fate, restart semantics become
  unprovable;
- if restart clears the recoverable operation without a visible terminal state,
  the local control surface drifts away from runtime truth.

## Related Tests

- `tests/e2e/node/lifecycle_test.go::TestNodeRuntimeLifecycleAcrossRestartPreservesPendingTruth`
- `tests/e2e/network-operator-terminal/terminal_operator_test.go::TestTerminalNodeRuntimeLifecycleAcrossRestartPreservesPendingTruth`

## False Positive Risk

- asserting only `Start()` success could miss the loss of the recoverable
  operation from diagnostics;
- asserting only that `operations.json` exists could miss a missing shutdown
  terminal fate.

## False Negative Risk

- the scenario must avoid transport/discovery timing so that failures map to
  runtime closure, not unrelated network behavior;
- assertions must not depend on exact event ordering beyond the required
  runtime truth.

## Notes

- transport/discovery participation is covered by dedicated network e2e
  scenarios;
- this scenario closes the operator-facing runtime evidence gap that remained
  after `NRI-001` integration coverage.
