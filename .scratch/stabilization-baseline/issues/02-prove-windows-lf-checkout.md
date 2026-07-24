# R0-002: Prove the Windows LF checkout contract

Status: ready-for-agent
Labels: ready-for-agent
Research class: R0

## Parent

`../PRD.md`

## User story

As a developer using Windows, I want a fresh checkout to contain gofmt-compatible
Go source regardless of `core.autocrlf` so that the canonical formatting gate
is reproducible without rewriting an existing worktree.

## What to build

Validate the checked-in Go LF policy from a fresh Windows checkout of the exact
R0-001 commit. Exercise the repository formatting entrypoint in the same source
representation a contributor receives with `core.autocrlf=true`.

The validation must use a disposable checkout or worktree. It must not mass
normalize the existing shared worktree to manufacture a passing result.

## Acceptance criteria

- Evidence identifies the full R0-001 commit SHA and Windows Git configuration.
- A fresh checkout with `core.autocrlf=true` materializes tracked Go files with
  LF line endings.
- The canonical formatting entrypoint passes without changing tracked files.
- A post-check status confirms the disposable checkout remains clean.
- A regression check demonstrates that removing or violating the LF policy
  would make the formatting contract fail.
- Temporary checkout material is removed after evidence is retained.

## Blocked by

- R0-001

## Comments

None.
