# STB-004 Evidence — Catalog And Canonical Failure Reports

## Catalog Repair

`TestDiscoveryStatusCountsExpiredRecordAsStaleAndRejected` is owned by the
Discovery `DKI-002` scenario. Its accidental duplicate listing in the
Diagnostics `DII-001` document was removed.

Final catalog validation:

- tests: 110;
- scenarios: 26;
- formal bindings: 110;
- missing bindings/docs/orphan scenarios: 0;
- issues: 0.

## Runner Repair

`tests/run.ps1` now:

- propagates a non-zero exit from direct fast `go test` and from `go list`;
- writes an explicit failed raw result when `go test -c` fails;
- writes an explicit failed raw result when a test binary fails without
  producing its own testkit report;
- catches tagged-suite failure, generates canonical JSON and JUnit in `finally`,
  and then returns the original failure;
- emits a meaningful fallback JUnit message when a failed test has no step list.

## Fault-Injection Evidence

A temporary integration fixture was created, exercised, and removed. It proved:

| Mode | Process exit | Raw | JSON | JUnit | Canonical result |
|---|---:|---:|---:|---:|---|
| selected success | 0 | yes | yes | yes | 3 passed, 0 failed |
| intentional test failure | non-zero | yes | yes | yes | 0 passed, 1 failed |
| intentional compile failure | non-zero | yes | yes | yes | explicit `[build]` failure |

Retained artifacts:

- `tests/.artifacts/reports/stb-004-success/`;
- `tests/.artifacts/reports/stb-004-build-failure/`;
- `tests/.artifacts/reports/stb-004-test-failure/`.

The last directory was deliberately reused after the red run. Its final state
contains exactly three current passing raw reports and a `3/0` summary, proving
that reset removed the stale failure while preserving the new run's complete
evidence.

## Final Checks

- PowerShell parser errors: 0;
- canonical fast runner: pass and returns success;
- catalog validation: pass with zero issues;
- temporary fixture source files remaining: 0.
