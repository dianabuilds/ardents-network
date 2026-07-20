# Continuous Development Prompt

Continue the Ardents `v1` stabilization and hardening loop governed by:

- `docs/process/v1-stabilization-hardening/execution-plan.md`
- `docs/process/v1-stabilization-hardening/decision-log.md`

Execution rules:

1. Read the governing plan and applicable source-of-truth documents.
2. Find the first `in_progress` task. If none exists, select the first `pending`
   task whose dependencies and phase gate allow execution.
3. Change only that task to `in_progress` before implementation.
4. Use the project skills required by the task, including dependency, runtime
   security, code-size, delivery, and acceptance skills where applicable.
5. Implement a runtime-real vertical result. Do not add fake foundations,
   prototype critical paths, silent fallbacks, or deferred mandatory behavior.
6. Add or update unit, integration, E2E, scenario, diagnostics, and operator
   evidence required by the task.
7. Run every check named by the task. A task becomes `done` only when all checks
   pass and evidence paths are recorded in the plan.
8. Re-evaluate the active phase transition gate immediately after each task.
9. Continue to the next admissible task or phase without stopping at an
   intermediate progress report.
10. Stop only at the final `done` gate or at a real blocker that has been
    exhausted, classified, and recorded in the decision log.

Do not issue a final response while any admissible task remains `pending` or
`in_progress` in the active phase.
