# STB-402 Evidence

Date: 2026-07-19
Status: completed

## Outcome

The product workload path now uses the Moby Docker Engine API on Linux for
immutable-image prepare, create/start, inspect, bounded stop, remove, restart
recovery, and orphan reconciliation. `cmd/ardd` selects this executor by
default. Raw process execution is available only through the explicit
`local_development` profile and cannot become a product fallback.

Containers carry deterministic Ardents ownership, node, workload, generation,
and runtime labels. Duplicate start converges on the existing generation.
Startup recovers the matching observed container and fail-closed removes any
managed container that is absent from, or superseded by, persistent registry
truth. Inspection retains runtime ID, timestamps, exit code/reason, OOM state,
Engine restart count, and the controller's restart/operator-action state.

## Acceptance Checks

- Isolated Docker-in-Docker lifecycle gate: 4/4 passed against Docker Engine
  `29.6.1`, including normal lifecycle, duplicate command, crash exit `7`,
  missing/mutable image failure, real restart-budget exhaustion, fresh
  controller recovery, forced termination, orphan cleanup, and remove.
- Focused workload integration: 18/18 passed; report at
  `tests/.artifacts/reports/stb-402-workload-final/summary.json`.
- Canonical fast suite passed in the Linux test container after the production
  changes.
- Full canonical integration baseline passed 113/113 before the final
  workload-only orphan reconciliation extension; the focused workload and fast
  gates were repeated after that extension. Report at
  `tests/.artifacts/reports/stb-402-integration-accepted/summary.json`.
- Go dependency graph: `go mod verify` passed. `govulncheck` found no new Moby
  issue; only the registered `GO-2026-4479` Waku/WebRTC residual remains.
- Code-size guard passed for every changed production Go file. `git diff
  --check` passed.

## Isolation And Resource Truth

The STB-402 creation baseline already denies networking, drops all
capabilities, disables privilege escalation, requires a numeric non-root user,
uses a read-only root filesystem plus bounded `/tmp`, and sets memory, CPU, PID,
and stop bounds. STB-403 remains responsible for policy-driven limits,
provenance, secrets, writable-storage accounting, denial diagnostics, and
adversarial enforcement tests.

All test execution occurred inside Linux containers. Resource snapshots showed
roughly 48-53% host memory use, at least 29.9 GB available memory, 1.2-4.8 GB
`vmmemWSL`, and about 222.7 GB free disk. No CPU, memory, or disk exhaustion was
observed, and no test containers remained after teardown.

## Evidence

- `internal/workload/execution/docker_*.go`
- `internal/workload/controller/recovery_service.go`
- `tests/integration/workload/docker_executor_test.go`
- `docker/docker-compose.workload-test.yml`
- `tests/run-workload-docker.ps1`
- `tests/.artifacts/reports/stb-402-docker-accepted/compose.log`
- `docs/workload-execution-platform.md`
- `docs/qa/integration/workload-runtime-recovery.md`
