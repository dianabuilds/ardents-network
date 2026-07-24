# R0-003: Prove the tagged scenario catalogue

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R0

## Parent

`../PRD.md`

## User story

As the CI owner, I want the static job to inspect every declared integration and
E2E scenario and reject an empty catalogue so that a green static gate cannot
silently omit tagged suites.

## What to build

Run the canonical static catalogue contract from the exact R0-001 source and
retain its machine-readable output. Verify both build-tag selection and the
fail-closed empty-result behavior through the supported CI entrypoints.

The observed scenario count is evidence, not a permanently hard-coded product
contract; metadata validity and non-emptiness are the stable requirements.

## Acceptance criteria

- Evidence identifies the full R0-001 commit SHA.
- The catalogue is generated with both `integration` and `e2e` build tags.
- The current baseline produces 142 valid scenario entries.
- Every entry satisfies the catalogue metadata rules.
- The workflow or entrypoint fails when either required tag is omitted.
- The workflow or entrypoint fails when catalogue output is empty.
- Tooling and entrypoint negative-matrix tests pass without retry.

## Blocked by

- R0-001

## Comments

- Completed on 2026-07-25 against
  `75471a6c08bf0c8a130db65d64c7f37dc33f03b5`.
- The tagged catalogue contains 142 valid entries.
- The entrypoint negative matrix passed, including missing-tag and empty-result
  rejection.
- Local JSON evidence:
  `tests/.artifacts/reports/catalog/r0-003-75471a6-validation.json`.
- Durable evidence:
  `../../../docs/engineering/evidence/stabilization-baseline-75471a6.md`.
