# Limits

## Default Policy

- Apply the limits to handwritten Go code in the root implementation.
- Use physical code lines, excluding blank lines and comment-only lines.
- Count a function from its `func` line through its closing brace.

## Thresholds

- Production file soft limit: 300 LOC
- Production file hard limit: 450 LOC
- Production function soft limit: 40 LOC
- Production function hard limit: 70 LOC

## Exceptions

- Skip generated files by default: `*.pb.go`, `*.gen.go`, `zz_generated*.go`, and files containing `Code generated`.
- Skip `aim-core` by default.
- Skip `_test.go` by default for automated checks. When editing tests directly, use the same checker but treat results as review thresholds instead of automatic blockers.

## Refactor Heuristics

- If a file is large because it owns multiple cohesive responsibilities, split the file inside the same package first.
- If one function is large because it mixes validation, normalization, state mutation, and diagnostics, extract those phases into named helpers.
- If a file is large because the package boundary is wrong, stop and use `ardents-architecture-guard` before moving code across domains.
- Do not create `domain/usecase/service/contracts` multiplication to satisfy the limit mechanically.

## Commands

Check one file:

```bash
go run ./skills/ardents-code-size-guard/scripts/check_go_size.go internal/workload/workload.go
```

Check a package tree:

```bash
go run ./skills/ardents-code-size-guard/scripts/check_go_size.go ./internal/workload ./internal/node
```

Include tests when the task is test-heavy:

```bash
go run ./skills/ardents-code-size-guard/scripts/check_go_size.go --include-tests ./internal/workload
```
