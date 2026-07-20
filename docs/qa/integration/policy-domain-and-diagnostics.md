# Scenario POI-001

- `Layer`: `integration`
- `Domain`: `Policy`
- `Category`: `admission / publication / authorization / diagnostics`

## Goal

Подтвердить, что `Policy` реально управляет admission, publication, retention и
route authorization, а deny outcomes видны через `policy.denied`, snapshot state
и operator-facing surfaces.

## Preconditions

- доступен node-managed runtime path с workload registration и hosted-service
  publication;
- доступен persisted data path для blob retention/pin checks;
- доступен multi-node path для peer blob fetch и imported discovery records;
- policy diagnostics доступны через `DiagnosticsSnapshot()` и `Snapshot()`.

## Steps

1. Запустить node с denied capability policy и попытаться зарегистрировать
   workload с запрещённой capability.
2. Запустить node с `DisableNetworkPublishedServices` и workload-hosted service
   в `NetworkPublished` mode.
3. Проверить `ResolveService`, workload status и hosted-service status.
4. Запустить node с запрещёнными local retention и pin operations, опубликовать
   blob и попытаться выполнить `RetainBlob` и `PinBlob`.
5. Подготовить source node с encrypted blob и policy, запрещающей peer
   re-serving; затем выполнить `FetchBlob` с trusted requester.
6. Подготовить local node с denied route scheme и импортировать remote service
   record с этим route scheme.
7. Проверить diagnostics events и policy snapshot после каждого denied path.

## Expected Result

- workload admission возвращает policy rejection и переводит policy state в
  `enforced`;
- denied service publication убирает service из usable resolution и делает
  operator-visible published status `false` с reason;
- retention/pin denials не меняют data truth и публикуют `policy.denied`;
- peer blob fetch не создаёт local copy, если source policy запрещает re-serving;
- route resolution не даёт usable route при denied scheme;
- deny outcomes наблюдаемы через diagnostics и `Snapshot().Policy`.

## Failure/Degraded Variant

- если admission deny не отражается в diagnostics, оператор теряет runtime truth
  о policy enforcement;
- если denied publication оставляет usable service match, publication truth
  рассинхронизирован с policy owner;
- если retention/pin deny всё равно меняет blob state, policy enforcement
  сломан;
- если source re-serving deny всё равно даёт requester local blob copy, broken
  data-policy boundary;
- если denied route остаётся usable, authorization boundary нарушена.

## Related Tests

- `tests/integration/policy/domain_test.go::TestPolicyRejectsWorkloadByCapability`
- `tests/integration/policy/domain_test.go::TestPolicyBlocksHostedServicePublicationAndSurfaceProjection`
- `tests/integration/policy/domain_test.go::TestPolicyRejectsBlobRetentionAndPinning`
- `tests/integration/policy/domain_test.go::TestPolicyRejectsPeerBlobReserving`
- `tests/integration/policy/domain_test.go::TestPolicyRejectsRouteUse`

## False Positive Risk

- тесты не должны считать policy path пройденным только по тексту ошибки без
  проверки diagnostics и snapshot state;
- publication scenario не должен ограничиваться только `ResolveService`, иначе
  hosted-service projection drift останется незамеченным;
- fetch denial не должен считаться успешным без проверки отсутствия local blob.

## False Negative Risk

- multi-node fetch и route scenarios не должны зависеть от неограниченного
  ожидания без timeout;
- diagnostics assertions не должны полагаться на точный порядок unrelated
  runtime events;
- persisted blob scenario не должен использовать общую data dir между
  независимыми тестами.
