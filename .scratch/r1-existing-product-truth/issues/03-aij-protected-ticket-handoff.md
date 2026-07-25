# AIJ-01: Complete protected Application Ticket Handoff

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1

## Parent

`../PRD.md`

## What to build

Provide a public Application enrollment journey from the Operator's protected
ticket file. Validate exact private-file identity, retain the file across safe
pre-commit failures, remove it only after enrollment commits, and return a
typed committed-but-cleanup-failed outcome without changing the existing
ticket or wire contracts.

## Acceptance criteria

- [x] Protected regular-file, symlink, replacement, permissions, size, padding, and trailing-input cases fail closed.
- [x] Successful enrollment removes the exact ticket file once.
- [x] Pre-commit failure retains the ticket for safe retry.
- [x] Post-commit cleanup failure is typed and does not retry enrollment.
- [x] Public examples and tests do not import Ardents internal packages.

## Blocked by

None - completed.

## Comments

- Completed in `36eb47d`.
