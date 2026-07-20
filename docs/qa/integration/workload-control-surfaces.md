# Scenario WKI-003

- `Layer`: `integration`
- `Domain`: `Workload Control + Hosted Services`
- `Category`: `functional`

## Goal

Проверить, что workload flows через operator-facing transport surfaces остаются
thin control surfaces над workload-owned truth: register/start/stop/disable/remove
и error mapping должны отражать фактическое runtime state без скрытой domain
logic в binding layers.

## Preconditions

- node runtime запущен и доступен через CLI/Proto operator surfaces;
- test harness может выполнять authenticated workload commands;
- для discovery-backed service resolution доступен remote node с workload-owned
  service publication.

## Steps

1. Зарегистрировать workload через operator-facing surface.
2. Запустить workload и считать snapshot через тот же surface.
3. Проверить observable `running` outcome и downstream query shape.
4. Выполнить stop/disable/remove или error path через surface.
5. Проверить, что surface возвращает итоговую domain truth, а не только success ack.
6. Для discovery path импортировать remote workload-backed service и разрешить
   его через operator-facing surface.

## Expected Result

- operator-facing surfaces отражают `running`, `stopped`, `disabled` и `removed`
  состояния синхронно с workload owner;
- rejected start/register paths возвращают explainable API errors;
- service resolution через operator-facing surface видит workload-backed service type;
- Connect workload round-trip не искажает desired/observed state.

## Failure/Degraded Variant

- если node stopped, register path не должен тайно мутировать workload state;
- если workload start fails, surface должен вернуть error и observable `failed`;
- если binding layer держит собственную domain truth, test должен падать на
  расхождении между mutation ack и `Get/List` snapshot.

## Related Tests

- `tests/integration/workload/control_surfaces_test.go::TestLocalWorkloadManagementFlow`
- `tests/integration/workload/control_surfaces_test.go::TestLocalWorkloadStartFailureIsObservable`
- `tests/integration/workload/control_surfaces_test.go::TestLocalWorkloadRegisterFailsWhenNodeStopped`
- `tests/integration/workload/control_surfaces_test.go::TestLocalResolveWorkloadServiceType`
- `tests/integration/workload/control_surfaces_test.go::TestConnectAPIWorkloadRoundTrip`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeDuplicateRegistrationPrefersConflictOverPolicy`

## False Positive Risk

- тест проверяет только отсутствие API error и не читает итоговый workload snapshot;
- stop/remove paths не подтверждают observable state после mutation;
- Connect path проверяет только transport success без domain state verification.

## False Negative Risk

- тест зависит от неуправляемого runtime timing вместо чтения фактического state;
- remote discovery setup оставляет незавершённые node instances и создаёт соседний шум;
- control-surface assertions проверяют transport детали, не относящиеся к
  workload scenario.

## Notes

Код integration tests для этого сценария должен явно разделять:

- `precondition(start node and surface)`
- `step(1, register/start workload)`
- `step(2, assert observable workload snapshot)`
- `step(3, stop/disable/remove or trigger error path)`
- `step(4, assert surface reflects owner truth)`
