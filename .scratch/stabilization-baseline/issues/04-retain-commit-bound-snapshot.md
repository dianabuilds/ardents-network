# R0-004: Retain a clean commit-bound R0 snapshot

Status: ready-for-agent
Labels: ready-for-agent
Research class: R0

## Parent

`../PRD.md`

## User story

As the release owner, I want one retained snapshot of the clean baseline's
local and static gates so that proven repository health is distinct from the
later Docker, native, security, multi-host, and release qualification program.

## What to build

Execute the applicable non-environmental gates against one clean exact commit,
retain their results with source identity, and reconcile the current
remediation ledger from that evidence.

This issue records R0 evidence only. It must not promote findings or
capabilities to `qualified` when their required R3 gates have not run.

## Acceptance criteria

- The snapshot identifies one full commit SHA and a clean starting worktree.
- Formatting, generation checks, vet, architecture acceptance, documentation
  contracts, audit traceability, tagged catalogue, fast unit tests, and the
  applicable critical lifecycle checks have recorded outcomes.
- No successful retry conceals an earlier failing attempt.
- Retained evidence distinguishes supported results from checks skipped because
  they require Docker, native Linux, networking, external security tooling, or
  independent release runners.
- The remediation ledger is updated only where the retained evidence satisfies
  its documented promotion rules.
- README and Changelog still state that no production release or final release
  qualification has been accepted.
- The final worktree is clean and the snapshot is attributable to the same
  source revision throughout.

## Blocked by

- R0-002
- R0-003

## Comments

None.
