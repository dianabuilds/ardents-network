# FEC-002: Make evidence promotion and active claims fail closed

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## User story

As a release reviewer, I cannot promote or document a capability as qualified
without one complete commit-bound evidence set.

## What to build

Validate owned evidence gates, canonical testcatalog scenarios, CI jobs,
environments, clean-run snapshots, retained artifact hashes, release gates,
active-document blocks, readiness disclaimers, and negative failure cases.

## Acceptance criteria

- `Q=yes` requires one complete matching-commit clean snapshot.
- Every required result matches its owned environment and retained hash.
- A required release gate and matching snapshot environment are mandatory.
- Active markers are declared, unique, well-nested, current, and confined to
  the `doccontract` allowlist.
- Generation preflights and rolls back the output set on failure.
- README and Changelog cannot overclaim production readiness.

## Blocked by

FEC-001.

## Comments

- Completed in `5cd480d`.
- Actual R3 environment execution and qualification snapshots remain separate.
