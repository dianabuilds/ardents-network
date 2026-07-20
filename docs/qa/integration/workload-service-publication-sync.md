# Scenario WKI-001

- `Layer`: `integration`
- `Domain`: `Workload Control + Hosted Services`
- `Category`: `functional`

## Goal

Проверить, что hosted-service publication truth из
`domain/hosting` / `application/hosting` синхронно следует за runtime truth и не остаётся
published после потери runtime backing.

## Preconditions

- зарегистрирован workload с runtime-backed service;
- service publication enabled policy не запрещает публикацию;
- workload подготовлен к запуску;
- test environment может наблюдать workload status и published service state.

## Steps

1. Запустить workload и дождаться состояния `running`.
2. Считать effective published services.
3. Подтвердить, что runtime-backed service опубликован.
4. Смоделировать failure или loss of runtime backing.
5. Повторно считать effective published services.
6. Считать hosted-service status explanation.

## Expected Result

- после шага 2 workload показывает published service;
- policy-aware effective publication truth фильтруется hosting-owned helpers,
  а не logic inside `internal/node/publication`;
- после потери runtime backing published service withdraws;
- hosted-service status объясняет, что publication removed because runtime
  backing is unavailable.

## Failure/Degraded Variant

- inspect degradation или failed runtime probe не должны оставлять stale
  publication truth;
- rollback/status path должен показывать explainable degraded outcome.

## Related Tests

- `domain/hosting/truth_test.go`
- `tests/integration/workload/publication_boundary_test.go::TestPublicationBoundaryWithdrawsServiceWhenRuntimeStops`
- `tests/integration/workload/publication_boundary_test.go::TestPublicationBoundaryPolicyDeniedStatus`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeFailureWithoutHostedServiceImpactKeepsReady`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeStopFailureDegradesWorkload`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeInspectFailureWithdrawsPublication`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeRollbackOnPublicationFailure`
- `tests/integration/workload/node_domain_test.go::TestWorkloadNodeReadPathsRefreshExitedTruth`

## False Positive Risk

- test проверяет только отсутствие ошибки, но не фактический published/unpublished state;
- failure path не меняет реальное runtime truth, а только флаг в test helper;
- status assertions не проверяют explanation reason.

## False Negative Risk

- test зависит от нестабильного тайминга reconcile без явного ожидания нужного state;
- test падает из-за соседнего sync timing, не относящегося к domain truth;
- test environment не отделяет runtime failure от policy denial.

## Notes

Код теста должен явно разделять:

- `precondition(...)`
- `step(1, start workload)`
- `step(2, assert published)`
- `step(3, remove runtime backing)`
- `step(4, assert unpublished and explained)`
