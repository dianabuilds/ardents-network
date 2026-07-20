---
name: ardents-code-size-guard
description: Guardrail skill for keeping handwritten Go files and functions small in the Ardents Network repo. Use when adding, reviewing, or refactoring code in `internal/*`, `cmd/ardd`, or other root `v1` Go packages and you need to check or enforce LOC limits before a change grows into an oversized file or function.
---

# Ardents Code Size Guard

Use this skill to keep handwritten root Go code small enough to preserve clear ownership and cheap refactors.

Read [limits.md](references/limits.md) only when you need the full policy, exceptions, or refactor patterns.

## Scope

- Apply this skill to handwritten Go code in the root `v1` implementation.
- Treat `aim-core` as legacy and out of scope unless the task explicitly edits it.
- Skip generated files by default, including `*.pb.go` and files with `Code generated`.
- Skip `_test.go` by default unless the task is mostly test refactoring.

## Limits

- Production file soft limit: 300 LOC
- Production file hard limit: 450 LOC
- Production function soft limit: 40 LOC
- Production function hard limit: 70 LOC
- `_test.go` review threshold: allow more room, but split obvious helpers before a test file becomes the package dump.

Treat hard limits as blockers. Treat soft limits as a prompt to split unless there is a tight, defensible reason not to.

## Workflow

1. Identify the handwritten Go files touched by the task.
2. Run `go run ./skills/ardents-code-size-guard/scripts/check_go_size.go <paths>`.
3. If a file or function crosses a hard limit, split it before considering the task complete.
4. If a file or function crosses a soft limit, prefer extracting helpers, types, or subflows inside the same package before changing package boundaries.
5. Re-run the checker after the refactor.
6. If a split would cross a domain boundary, use `ardents-architecture-guard` before restructuring.

## Split Strategy

- Split by responsibility first, not by arbitrary suffixes.
- Keep cohesive helpers in the same package when ownership does not change.
- Extract state transitions, validation, mapping, formatting, and snapshot building into separate files before inventing new top-level packages.
- Do not preserve a large file by pushing unrelated code into `aim-core` or by creating legacy-style layer multiplication.

## Output

When this skill is active, produce:

- the checked paths;
- file and function LOC breaches, if any;
- the intended split plan for every soft or hard breach;
- confirmation that the final shape still matches the owning domain.
