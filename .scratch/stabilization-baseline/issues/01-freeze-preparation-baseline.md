# R0-001: Freeze the preparation baseline

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R0

## Parent

`../PRD.md`

## User story

As a repository maintainer, I want the preparation work represented by one
exact clean source commit so that every subsequent investigation and evidence
run starts from an immutable baseline.

## What to build

Review and land the coherent preparation change that:

- establishes the global feature-research plan, Wave 0 baseline, capability
  register, Application Discovery research packet, and current remediation
  ledger;
- establishes the local Markdown tracker and its triage/domain conventions;
- declares deterministic LF checkout behavior for Go source;
- makes the static integration/E2E catalogue include tagged suites and reject
  an empty result;
- removes the agreed obsolete audit narratives while retaining the historical
  coverage and finding registers;
- keeps audit traceability backed by the current remediation ledger.

Do not combine unrelated product implementation with this baseline.

## Acceptance criteria

- The preparation change is reviewed as one intentional source revision.
- The retained audit directory contains only the historical coverage and
  findings registers.
- No active code, test, workflow, or documentation link references a deleted
  audit file.
- Audit traceability reports 21 mapped findings and 5 gates.
- Tooling and entrypoint contract tests pass.
- `git diff --check` reports no whitespace errors before the revision is
  finalized.
- The resulting source revision is identified by its full commit SHA.
- The worktree used to start R0-002 and R0-003 is clean.

## Blocked by

Nothing.

## Comments

- Prepared from the locally verified Wave 0/1 research worktree.
- Completed on 2026-07-25.
- Baseline commit:
  `75471a6c08bf0c8a130db65d64c7f37dc33f03b5`.
- Before the commit, full unit/tooling tests, vet, audit traceability,
  entrypoint contracts, and staged whitespace validation passed.
- Durable evidence:
  `../../../docs/engineering/evidence/stabilization-baseline-75471a6.md`.
