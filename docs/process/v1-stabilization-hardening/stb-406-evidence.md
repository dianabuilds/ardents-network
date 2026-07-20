# STB-406 Evidence

Date: 2026-07-19
Status: completed

## Outcome

The supported workload path now proves the complete service lifecycle against a
real generation-bound HTTP service: register, start, readiness, publication,
remote resolution and request, unexpected exit, bounded withdrawal, operator
restart, fresh readiness, and republish. A request succeeds only while the
current workload generation is ready and exposure eligible.

Docker recovery now reconciles both the workload container and its node-owned
ancillary ingress proxy. A daemon restart cannot leave a recovered workload
published through a missing or stale proxy. Reconciliation recreates the proxy
from canonical workload state and preserves the isolation boundary: the
workload has no host port, remains on its internal network, and the proxy exposes
only the admitted service port.

The runtime diagnostics controller was made race-safe while workload state and
publication state change concurrently. Privacy test fixtures now derive
separate deterministic topics for independent scenarios, preventing test-only
topic collisions from hiding or inventing network behavior.

Integration and E2E scenarios that previously used a sleeping process with a
synthetic TCP/QUIC declaration now use an actual HTTP listener and request-based
readiness. This applies to discovery, local-control, policy, workload, and
terminal operator flows.

## Acceptance Checks

- Canonical fast suite passed in a Linux container after all production,
  catalog, and mutation-test changes. The retained container
  `ardents-stb406-fast-final3` exited `0` after 17 seconds.
- Full integration suite passed 116/116 in 316572.687 ms. Report:
  `tests/.artifacts/reports/stb-406-integration-full-final/summary.json`.
- Full E2E suite passed 14/14 in 83452.612 ms. Report:
  `tests/.artifacts/reports/stb-406-e2e-full-green/summary.json`.
- Docker-in-Docker isolation, resource, crash, and recovery gate passed 9/9.
  Report: `tests/.artifacts/reports/stb-406-docker-recovery/compose.log`.
- Focused discovery, local-control, policy, workload, and terminal suites passed.
  Retained reports are under `tests/.artifacts/reports/stb-406-*`.
- The test catalog reports 139 tests, all 139 formally bound, with zero missing
  metadata and zero catalog issues. Helper-process entrypoints are explicitly
  excluded from scenario inventory.
- Import-boundary checks, changed-production code-size checks, `go vet ./...`,
  and `go mod verify` passed in Linux containers.

## Mutation Evidence

Two publication/readiness shortcuts were deliberately introduced one at a time,
proved detectable, and restored before the final green run:

- removing the `ExposureEligible` publication gate caused the focused
  publication test to expose a forbidden service in the allowed set;
- disabling generation-regression rejection caused
  `TestControllerNeverReturnsToAnOlderGeneration` to fail.

Both focused tests passed after restoration. The permanent regression coverage
includes an explicit `Ready=true` and `ExposureEligible=false` case, so process
or probe readiness alone cannot authorize publication.

## Security And Architecture Review

- Waku remains the canonical network carrier; no alternate discovery or
  publication substrate was introduced.
- Product execution remains the selected Docker isolation backend. Raw host
  execution is still explicitly non-product trusted-development behavior.
- Recovered ingress is reconstructed from canonical admitted state and receives
  neither workload secrets nor arbitrary Docker or filesystem authority.
- Readiness, exposure eligibility, reachability, policy, and publication remain
  separate gates. Older workload generations cannot republish.
- No critical workload, hosted-service, publication, or diagnostics behavior is
  deferred behind a stub or fake foundation.

## Resource And Runner Truth

All tests ran in Docker/Linux; Windows was used only to orchestrate containers.
The final snapshot recorded 1367.2 MB for `vmmemWSL`, 31371.8 MB available host
memory, 50.2% total memory use, and 219.02 GB free disk. Top sampled processes
used only single-digit CPU percentages. Evidence:
`tests/.artifacts/resources/stb-406-final.json`.

Several Codex/PowerShell monitoring commands remained displayed as running long
after their Docker containers had exited. Direct `docker inspect` showed the
tests had completed successfully; this was runner-shell wait behavior, not test
or resource exhaustion. Subsequent validation used named detached containers
and immediate `docker inspect`/`docker logs` checks without `Start-Sleep` or a
long-lived synchronous `docker wait`.

## Evidence Surface

- `internal/runtime/authority/controller_workload.go`
- `internal/workload/controller/recovery_service.go`
- `internal/workload/execution/docker_ancillary.go`
- `internal/publication/plan_test.go`
- `internal/hosting/readiness/controller_test.go`
- `tests/e2e/workload/`
- `tests/e2e/discovery/`
- `tests/e2e/local-control-surface/`
- `tests/e2e/terminal-operator/`
- `tests/integration/discovery/`
- `tests/integration/local-control-surface/`
- `tests/integration/policy/`
- `tests/integration/workload/`
- `tests/cmd/testcatalog/`
