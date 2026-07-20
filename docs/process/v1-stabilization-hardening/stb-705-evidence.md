# STB-705 Evidence — Bounded Soak Gate Readiness

Date: 2026-07-20

## Accepted scope

STB-705 accepts the reproducible bounded soak mechanism and representative
cross-domain smoke evidence. The uninterrupted 48-hour run is retained as a
separate pre-release qualification after review and refactoring freeze; it is
not claimed here.

## Candidate and plan

- candidate commit: `b89b8c002bec15fcd3278817b0054e4913592141`
- candidate image: `ardents/ardd-testnet:dev`
- image ID: `sha256:acedf6a9fbf62ac3f28a4f2b7ac2f83891a93c6c63a8134fdb5ac501c0f8bb16`
- OCI revision and build date matched the Git candidate;
- `SOAK-001 -PlanOnly` validated 48 hourly checkpoints;
- every child command has a hard timeout, retained stdout/stderr, exact-PID
  cleanup, atomic state, and JSONL lifecycle events.

## Observed execution

Retained reports:

- `tests/.artifacts/reports/stb-705-soak-12620bf`: exposed missing child exit
  status in Windows `Start-Process`; driver failed fast and cleaned up;
- `tests/.artifacts/reports/stb-705-soak-0cfc86d`: proved the replacement process
  wrapper, then exposed use of `.NET Path.GetRelativePath` under Windows
  PowerShell 5.1 before DAI-003 reached Docker;
- `tests/.artifacts/reports/stb-705-soak-b89b8c0`: corrected run, intentionally
  stopped when the 48-hour gate was moved outside the active goal.

The corrected driver completed before interruption:

- `multihost-full`: exit `0`, including controlled fault and recovery;
- `DAI-003`: exit `0`, encrypted chunked fetch/resume over private Waku;
- `DAI-004`: exit `0`, encrypted availability/repair behavior;
- `WKE-001`: exit `0`, workload lifecycle.

`E2E-NPI-001` had started but was intentionally interrupted with the driver. The
driver and its exact child process tree were stopped, and no matching Docker
container remained. No product-test failure or resource exhaustion was observed.

## Decision

The bounded soak gate is ready for later use against an immutable release
candidate. The stabilization loop proceeds to STB-706. Passing a partial run is
not represented as passing the required future 48-hour qualification.
