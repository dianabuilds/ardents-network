# STB-701 Evidence — Scenario And Invariant Coverage Audit

## Accepted Coverage State

- 151 repository integration/E2E/external-gate test bindings are formal.
- 49 scenario documents have runnable bindings.
- Every scenario has canonical layer/domain metadata and non-empty
  false-positive and false-negative risk analysis.
- 55 mandatory requirement sets cover core system properties, seven reference
  invariants, all ten domain invariant sets, communication contracts, privacy,
  persistent-state security, operator access/configuration, workload/data,
  observability, deployment, and dependency safety.
- 55 requirements are `covered`; zero are `blocked`; zero have validation
  issues.

## Gaps Found And Closed

The previously green catalog omitted the executed segmented multi-host scenario
and did not validate scenario risk analysis or canonical document metadata.
`NFM-001` now has its own E2E document and catalog-visible CI wrapper. The
validator now rejects missing/malformed layer, domain, false-positive, and
false-negative fields. Sixteen stale scenario documents were normalized or
completed.

The catalog also previously proved only test-to-scenario binding, not
requirement-to-evidence traceability. `docs/qa/requirements-coverage.json` and
the requirement-aware validator close that gap and require every mandatory
source to appear in the matrix.

## Validation Evidence

| Check | Result |
| --- | --- |
| `go test ./tests/cmd/testcatalog` in Docker | passed |
| Requirement-aware catalog validation | 151 tests, 49 scenarios, 0 issue |
| Requirement matrix | 55 covered, 0 blocked, 0 issue |
| Code-size guard | passed; no soft or hard breach |
| Current product fast suite with coverage | passed before QA-only changes |
| Current product integration baseline | 132/132 passed before QA-only changes |
| Current product E2E | 17/17 passed before QA-only changes |
| Current segmented multi-host runtime | 13/13 steps passed |

No product runtime code changed after the accepted integration/E2E/multi-host
reports. Repeating those network suites would not increase confidence in the
QA parser/document changes, so validation remained targeted to the catalog
package and static inventory.

Primary artifacts:

- `docs/qa/requirements-coverage.json`
- `docs/qa/requirements-coverage.md`
- `docs/qa/e2e/multi-host-network-resilience.md`
- `tests/ci/multihost-gate.ps1`
- `tests/cmd/testcatalog/inventory_requirements.go`
- `tests/.artifacts/reports/stb606-e2e-final/summary.json`
- `tests/.artifacts/reports/stb606-multihost-final/summary.json`

## Acceptance Decision

`passed` on 2026-07-20. No mandatory source lacks traceable evidence or a
formal decision, and catalog validation reports no binding, document, risk, or
requirement issue.

