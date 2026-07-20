# AGENTS

## Project Rule

Этот репозиторий разрабатывает `v1` Ardents поверх актуальных документов в `docs/`.
Legacy и reference-материалы можно использовать только как источник механизмов и инвариантов:

- их нельзя копировать как архитектурную форму;
- их нельзя игнорировать там, где они фиксируют системообразующие механизмы.

## Mandatory Source Of Truth

Перед любым нетривиальным изменением сначала сверяйся с:

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/canonical-network-foundation.md`
- `docs/engineering-constraints.md`
- `docs/development-contract.md`
- `docs/process/process-template/README.md`, если работа требует нового process loop или execution plan
- `docs/qa/test-model.md`, если работа затрагивает тесты, QA scenarios или test-layer split
- релевантным доменным документом
- `docs/reference-invariants.md`, если изменение затрагивает network/discovery/messaging/publication foundation

Если кодовое изменение конфликтует с этими документами, сначала обновляй документы, потом код.

## Non-Negotiable Rules

- Не реализовывать critical domains как прототипы, заглушки или временные skeletons.
- Не вводить fake foundation в network, messaging, data, workload execution и других critical planes.
- Не писать свой substrate с нуля там, где документы требуют зрелую зависимость.
- Не обходить `Waku` как canonical network foundation для `v1`.
- Не вводить новую несущую зависимость без проверки её роли, support posture и уязвимостей.
- Не считать slice завершённым, если его обязательные свойства отложены "на потом".
- Не останавливать development loop на промежуточном прогрессе, если у активного process document есть хотя бы одна задача со статусом `pending` или `in_progress`, кроме случая реального `blocked`.
- Не подменять непрерывную разработку отчётом о проделанной работе.
- После завершения любой задачи немедленно возвращаться к активному process document, проверять gate текущей фазы и переходить к следующей задаче.
- Не выдавать final response, пока у активной фазы не пройден gate и не исчерпаны допустимые кодовые задачи текущего loop, кроме случая реального `blocked`.
- Не переходить к следующей фазе, пока transition gate текущей фазы не пройден полностью.

## Available Skills

### ardents-architecture-guard

- Description: Guardrail skill for architecture-aligned changes in the Ardents Network repo. Use when changing domain boundaries, repo structure, local API shape, product surfaces, or when deciding whether legacy/reference logic should be reused, rewritten, or retired.
- Path: `skills/ardents-architecture-guard/SKILL.md`

### ardents-code-size-guard

- Description: Guardrail skill for keeping handwritten Go files and functions small in the root `v1` implementation. Use when adding, reviewing, or refactoring code in `internal/*`, `cmd/ardd`, or other root Go packages and you need to enforce LOC limits before a file or function grows too large.
- Path: `skills/ardents-code-size-guard/SKILL.md`

### ardents-development-guard

- Description: Guardrail skill for enforcing the Ardents development contract. Use when starting or reviewing any substantial change to ensure the work is product-grade, not prototype-first, and does not introduce fake foundations or deferred critical behavior.
- Path: `skills/ardents-development-guard/SKILL.md`

### ardents-dependency-safety

- Description: Dependency and security review workflow for Ardents. Use when selecting, adding, replacing, or upgrading dependencies that affect network, storage, crypto, observability, or other critical product foundations.
- Path: `skills/ardents-dependency-safety/SKILL.md`

### ardents-acceptance-gate

- Description: Final validation workflow for Ardents slices and substantial code changes. Use when implementation is believed complete and must be checked for tests, diagnostics visibility, code-size compliance, and absence of deferred critical behavior.
- Path: `skills/ardents-acceptance-gate/SKILL.md`

### ardents-runtime-security-guard

- Description: Runtime security review workflow for Ardents. Use when a change touches encryption, key handling, retained data, relay behavior, diagnostics redaction, local secrets, or other runtime security-sensitive behavior.
- Path: `skills/ardents-runtime-security-guard/SKILL.md`

### ardents-release-code-review

- Description: Release-focused code review workflow for Ardents. Use when preparing a release or reviewing a release-sized change set to find bugs, regressions, unsafe assumptions, missing tests, and document drift before shipping.
- Path: `skills/ardents-release-code-review/SKILL.md`

### ardents-release-bug-hunt

- Description: Failure-oriented release bug-hunting workflow for Ardents. Use before a release or after risky merges to search for latent bugs, broken edge cases, stale runtime truth, and cross-domain regressions that ordinary review may miss.
- Path: `skills/ardents-release-bug-hunt/SKILL.md`

### ardents-release-vulnerability-review

- Description: Release-time vulnerability review workflow for Ardents. Use before shipping when you need a dedicated sweep for dependency vulnerabilities, runtime exposure risks, plaintext leakage, unsafe defaults, diagnostics leaks, and other security blockers.
- Path: `skills/ardents-release-vulnerability-review/SKILL.md`

### ardents-release-error-handling-review

- Description: Release-time error handling review workflow for Ardents. Use before shipping or after risky changes to verify that errors are not ignored, failure paths are explicit, degraded states are explainable, and runtime behavior does not silently continue after broken operations.
- Path: `skills/ardents-release-error-handling-review/SKILL.md`

### ardents-release-regression-sweep

- Description: Final cross-cutting release sweep for Ardents. Use near the end of a release to re-check startup, recovery, diagnostics, local API truth, network truth, publication truth, tests, and document alignment across the changed domains.
- Path: `skills/ardents-release-regression-sweep/SKILL.md`

### ardents-legacy-extraction

- Description: Legacy extraction workflow for mining old implementations without importing their architecture. Use when inspecting old code to extract logic, signatures, heuristics, runtime flows, or proven mechanisms into the new root implementation.
- Path: `skills/ardents-legacy-extraction/SKILL.md`

### ardents-v1-slice-delivery

- Description: Delivery workflow for implementing or expanding a `v1` slice in the root codebase. Use when adding functionality to `internal/*`, `cmd/ardd`, or control surfaces while preserving the current system documents and domain ownership.
- Path: `skills/ardents-v1-slice-delivery/SKILL.md`

## How To Use Project Skills

- Trigger a skill when the task clearly matches its description or when the skill name is mentioned directly.
- Use the minimal number of skills needed for the task.
- For substantial code changes, prefer sequencing: `ardents-development-guard` -> `ardents-architecture-guard` -> domain-specific delivery or extraction skill.
- For substantial handwritten Go edits, also use `ardents-code-size-guard` before considering the change complete.
- Use `ardents-acceptance-gate` near the end of a slice, not at the start.
- Use `ardents-runtime-security-guard` for runtime security behavior; use `ardents-dependency-safety` for library/security posture of dependencies.
- For release preparation, prefer sequencing: `ardents-release-code-review` -> `ardents-release-bug-hunt` -> `ardents-release-vulnerability-review` -> `ardents-release-error-handling-review` -> `ardents-release-regression-sweep`.
- Prefer the root implementation as the active product, but always validate critical behavior against the current reference invariants where they apply.
- Do not recreate legacy layering such as parallel contracts, runtime facades, or domain multiplication by default.
