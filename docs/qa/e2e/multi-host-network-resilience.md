# NFM-001 — Segmented Multi-Host Network Resilience

## Scenario ID

`NFM-001`

## Layer

`e2e`

## Domain

`network-foundation`

## Category

Self-formation, segmented topology, partition recovery, churn, and retained
opaque Waku Store history.

## Purpose

Prove that Ardents remains an operational multi-node Waku network across two
isolated network segments instead of degrading into local-only registries or
single-process simulations.

## Layer And Environment

- layer: `e2e`
- environment: Docker/Linux with host orchestration only
- topology: seed, dual-homed bridge, two peers per segment, one constrained
  recovery client, and one isolated Docker workload engine
- tags: `network`, `waku`, `wss`, `partition`, `recovery`, `churn`, `store`

## Scenario

1. Build or select the committed testnet image and create two isolated bridge
   networks.
2. Start the seed and dual-homed bridge, then derive bootstrap endpoints from
   their signed runtime records.
3. Start both segments and a constrained client whose only cross-segment path
   uses WSS; require joined network truth for every node.
4. Kill a peer process with `SIGKILL`, recreate the container, and require
   recovery without replacing its persisted node identity.
5. Recreate another peer at a new address and require the new signed endpoint.
6. Lose the seed while the remaining healthy mesh stays operational.
7. Partition and rejoin one segment; require local steady participation,
   explicit restricted defense for an isolated peer, and eventual steady
   recovery.
8. Repeat bounded peer churn.
9. Publish opaque retained content, restart a fresh constrained client, and
   fetch the content through Waku Store.
10. Retain topology, versions, diagnostics, results, and resource snapshots;
    remove the disposable project unless evidence retention was requested.

## Related Tests

- `tests/ci/multihost-gate.ps1`

## False Positive Risk

Container health could pass while peers are local-only, a partition is not
observed, or recovery uses stale records. The gate requires signed endpoint
changes, joined/active-mode truth at every transition, intentional Docker
network disconnect/reconnect operations, and a fresh-client Store fetch.

## False Negative Risk

Waku convergence and Docker scheduling are asynchronous. Every wait is bounded;
the runner retains step results, compose logs, topology snapshots, versions,
and resource evidence so infrastructure slowness can be distinguished from a
product failure.
