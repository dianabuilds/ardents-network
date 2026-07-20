# Scenario WKE-001

- `Layer`: `e2e`
- `Domain`: `Workload Control + Hosted Services`
- `Category`: `non-functional`

## Goal

Проверить полный lifecycle workload-backed service через реальный node-managed
runtime path: startup, publication, graceful node stop, restart recovery и
observable diagnostics без рассинхронизации между
`application/workload/runtime`, `application/hosting` и downstream
surfaces.

## Preconditions

- node runtime запущен в container-based environment;
- workload with hosted service задан в node configuration или зарегистрирован
  через control surface;
- diagnostics и local surface доступны для чтения состояния;
- publication path активен;
- test harness умеет различать graceful node stop и restart recovery path.

## Steps

1. Запустить node с workload-backed service.
2. Дождаться observable state `running`.
3. Подтвердить publication hosted service.
4. Считать diagnostics explanation для `running` path.
5. Выполнить graceful `node.Stop()`.
6. Подтвердить завершение workload process и withdrawal publication без ложного
   recovery marker.
7. Поднять новый node/runtime поверх того же persisted state.
8. Подтвердить, что persisted workload truth и runtime recovery возвращают
   workload в согласованное `running` состояние.
9. Подтвердить повторную публикацию hosted service только после фактического
   восстановления runtime backing.
10. Считать diagnostics explanation после restart recovery.
11. Выполнить реальный HTTP request к advertised endpoint и проверить, что
    listener возвращает proof текущей workload generation.
12. Принудительно завершить workload, проверить degraded diagnostics и
    withdrawal, затем выполнить restart и дождаться новой readiness proof перед
    повторной publication.
13. В Docker product path остановить ingress proxy, подтвердить потерю
    доступности, восстановить ancillary runtime из persisted generation identity
    и удалить proxy-сироту, не принадлежащую текущей generation.

## Expected Result

- lifecycle проходит через explainable states без скрытых переходов;
- publication truth соответствует runtime truth на каждом этапе;
- diagnostics и local surface показывают operator-visible reasons;
- recovery возвращает workload и hosted service в согласованное состояние;
- graceful restart path не оставляет ложный recovery marker и не поднимает
  publication преждевременно.
- реальный request проходит только при текущих `running + ready + published`;
- crash и потеря ingress proxy не оставляют stale publication, а recovery не
  переиспользует старое readiness evidence.

## Failure/Degraded Variant

- если recovery неуспешен, publication не должен вернуться преждевременно;
- если node shutdown был graceful, test не должен требовать crash-style
  degraded marker;
- если diagnostics path недоступен, test должен явно фиксировать degraded
  observability, а не silently pass.

## Related Tests

- `tests/e2e/workload/lifecycle_test.go::TestWorkloadHostedServiceLifecycleAcrossNodeRestart`
- `tests/e2e/workload/lifecycle_test.go::TestWorkloadHostedServiceObservedExitWithdrawsPublicationAndDegradesDiagnostics`

## False Positive Risk

- тест проходит только по факту успешного startup/shutdown call, без проверки
  runtime state, publication state и diagnostics;
- recovery считается успешным без проверки повторной публикации;
- graceful stop не завершает workload process, но тест этого не замечает;
- restart path поднимает publication до восстановления runtime backing.

## False Negative Risk

- тест нестабилен из-за незафиксированных ожиданий runtime convergence;
- тест падает на временном сетевом шуме, который не относится к сценарию;
- test setup не отделяет graceful restart path от environment failure;
- assertions не различают graceful restart path и interrupted recovery path,
  который покрывается integration scenario `WKI-002`.

## Notes

Код e2e test должен явно разделять:

- `precondition(start node with workload-backed service)`
- `step(1, assert running and published)`
- `step(2, stop node and assert process exit plus withdrawn publication)`
- `step(3, restart node from persisted state)`
- `step(4, assert recovered state, publication and diagnostics)`
- `degraded(assert unexpected workload exit withdraws publication and degrades diagnostics)`
