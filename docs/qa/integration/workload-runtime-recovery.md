# Workload Runtime Recovery

## Scenario ID

`WKI-002`

## Layer

`integration`

## Domain

`Workload Control + Hosted Services`

## Category

Runtime recovery, reconciliation, and removal.

The scenario also includes the Linux Docker Engine acceptance path for
immutable-image create/start/inspect/stop/remove, idempotent duplicate start,
fresh-executor recovery through Ardents ownership labels, crash exit-code
retention, bounded forced termination, orphan removal, restart-budget
exhaustion, and explicit rejection of mutable image tags. That path runs through
`tests/run-workload-docker.ps1` against an isolated Docker-in-Docker daemon and
does not mutate unrelated host containers.

## Goal

Проверить runtime-backed recovery path внутри workload domain без полного
operator e2e harness: persisted workload truth, restart recovery и shutdown path
должны сохранять согласованность между `internal/workload/controller`,
`internal/workload/execution` и workload-owned publication snapshot.

## Preconditions

- workload зарегистрирован с `DesiredRunning`;
- execution owner запускает реальный process-backed workload;
- persisted runtime state пишется на диск;
- test environment умеет различать живой PID, graceful stop и удаление workload.

## Steps

1. Запустить workload и подтвердить observable `running`.
2. Подтвердить, что runtime snapshot публикует hosted service.
3. Создать новый runtime service поверх persisted state и выполнить `Load()`.
4. Подтвердить, что recovery path доверяет живому PID и сохраняет publication truth.
5. Выполнить `Reconcile()` после восстановления.
6. Выполнить graceful `StopAll()` или `DesiredRemoved` path.
7. Перезагрузить persisted state ещё раз и подтвердить отсутствие ложного recovery marker.

## Expected Result

- persisted workload возвращается в `running`, если runtime backing реально жив;
- publication snapshot не остаётся stale и не теряется без причины;
- graceful stop сохраняет `stopped` truth и не возвращает workload в `degraded`;
- removed workload исчезает из persisted runtime state.

## Failure/Degraded Variant

- если PID probing ложно считает живой workload остановленным, тест должен падать;
- если graceful stop оставляет published service или restart recovery marker,
  тест должен падать;
- если recovery зависит от host-specific shell tooling вместо process-level truth,
  container path должен выявлять drift.

## Related Tests

- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeStopClearsRestartRecoveryMarker`
- `tests/integration/workload/runtime_recovery_test.go::TestRuntimeRecoveryPersistsAndPublishes`
- `tests/integration/workload/runtime_recovery_test.go::TestRuntimeStoppedAndRemovedTransitions`
- `tests/integration/workload/runtime_recovery_test.go::TestRuntimeStopAllMarksStoppedAndUnpublished`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorLifecycleIsIdempotentAndRecoverable`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorRetainsCrashOutcomeAndRejectsMutableImage`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorControllerRecoversAndRemovesObservedInstance`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorForceStopsAndControllerRemovesOrphan`

## False Positive Risk

- тест проверяет только успешный вызов `Load()`/`Reconcile()`, но не observable
  `ObservedRunning`/`ObservedStopped`;
- test path не проверяет publication snapshot и потому пропускает stale published
  state;
- graceful stop и remove path не проверяют persisted result после повторной
  загрузки.

## False Negative Risk

- test зависит от platform-specific process probe и падает при живом PID;
- test опирается на неуправляемый timing без проверки фактического runtime state;
- environment убивает helper process вне сценария и создаёт ложный degraded path.

## Notes

Код integration tests для этого сценария должен явно разделять:

- `precondition(register/start runtime-backed workload)`
- `step(1, assert running and published snapshot)`
- `step(2, load persisted runtime state)`
- `step(3, assert recovered running truth)`
- `step(4, graceful stop or remove)`
- `step(5, assert persisted stopped/removed truth)`
