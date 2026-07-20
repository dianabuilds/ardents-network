# V1 Stabilization And Hardening Loop

This directory contains the governing process documents for bringing the root
Ardents `v1` implementation from the current technical-alpha state to a
release-candidate state backed by real runtime evidence.

The governing document is:

- `execution-plan.md`

Supporting documents:

- `decision-log.md` records blocker, ordering, scope, and acceptance decisions;
- `continuous-development-prompt.md` is the entry point for an implementation
  agent continuing the loop.

The documents in this directory do not replace system, domain, security, or QA
sources of truth. When they conflict, source-of-truth documents are updated
first, then this process plan, then code.

Current loop state: `done`

No stabilization task remains active. Refactoring and the later immutable-
candidate soak qualification are governed outside this completed loop.
