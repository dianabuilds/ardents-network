# STB-005 Evidence — Mandatory Repository Gates

## Contract

The authoritative local/CI gate matrix is documented in `tests/README.md` and
uses the existing `tests/run.ps1` suite runner. `tests/check-format.ps1` is a
single-purpose fail-closed formatting guard, not a competing test runner.

The matrix defines formatting, vet, fast/import-boundary, integration, E2E,
catalog, reachable vulnerability, and verbose vulnerability-evidence rows with
explicit commands and pass conditions.

Windows behavior is explicit:

- the tagged runner adds `gowaku_no_rln` because of the upstream/toolchain
  limitation;
- this does not skip real Relay, Store, Filter, Lightpush, bootstrap, peer-loss,
  and recovery execution;
- non-elevated firewall pre-approval warnings do not bypass suite execution.

Failure artifacts must be retained for at least 30 days. Release-candidate
artifacts and checksums are retained for the supported lifetime of the release.
Artifact publication failure is a gate failure.

## Executed Matrix Evidence

- formatting: pass, zero listed files;
- `go vet ./...`: pass;
- canonical fast/import guard: pass;
- canonical integration: 97 passed, 0 failed, 97 raw reports, JSON and JUnit;
- canonical E2E: 13 passed, 0 failed, 13 raw reports, JSON and JUnit;
- catalog: 110 tests, 26 scenarios, 0 issues;
- runner failure semantics: build/test red evidence retained by `STB-004`.

Tagged evidence:

- `tests/.artifacts/reports/stb-005-integration/`;
- `tests/.artifacts/reports/stb-005-e2e/`.

The vulnerability rows are executable and intentionally remain red against the
baseline's 11 reachable findings. That is assigned Phase 1 security work and is
not an infrastructure waiver.

## Phase 0 Gate

Phase 0 passed on the supported Windows environment:

- fast, integration, E2E, and catalog are green;
- failure reports are fail-closed and retained;
- remaining red signals are classified product/security work with later task
  ownership.
