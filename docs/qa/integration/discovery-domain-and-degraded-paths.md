## Scenario ID

`DKI-002`

## Layer

`integration`

## Domain

`Discovery`

## Category

Domain truth, stale/expired persistence, degraded bootstrap, local refresh.

## Goal

Проверить domain-owned discovery semantics вне mixed package form:
resolve/import invariants, expired persisted records, degraded bootstrap state и refresh local publication.

## Preconditions

- узел запускается на реальном runtime path без fake discovery foundation;
- degraded cases используют реальные transport/bootstrap inputs;
- persisted discovery records загружаются из фактического state path.

## Steps

1. Проверить, что query-only discovery calls не мутируют route truth.
2. Импортировать удалённую node/service record и подтвердить expected resolve outcomes.
3. Загрузить expired persisted record и убедиться, что она даёт `expired`, а не usable route.
4. Проверить, что static service без runtime backing не публикуется.
5. Проверить degraded startup при недоступном bootstrap peer и при invalid bootstrap record.
6. Перезапустить узел после persisted degraded discovery state и подтвердить, что snapshot восстанавливает тот же explainable `state/reason`.
7. Проверить refresh локальных discovery records до истечения TTL.
8. После stop узла проверить, что record/service resolution не возвращает usable routes.

## Expected Result

- discovery truth остаётся explainable и согласованной;
- stale/expired inputs не превращаются в usable runtime routes;
- degraded bootstrap отражается в snapshot/diagnostics и сохраняется через restart, пока owner truth не изменилась;
- local discovery publication обновляется до TTL expiry.

## Failure/Degraded Variant

- bootstrap peer недоступен: node/transport/boot переходят в `degraded`;
- bootstrap payload невалиден: primary reason указывает на `discovery`;
- stale import не должен считаться успешной публикацией.

## Related Tests

- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveQueriesDoNotMutateRouteTruth`
- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveQueriesDoNotMutateTrustTruth`
- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveImportedRecord`
- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveRecordRejectsExpiredPersistedRecord`
- `tests/integration/discovery/domain_test.go::TestDiscoveryDoesNotPublishStaticServiceRecordWithoutRuntimeBacking`
- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveServiceType`
- `tests/integration/discovery/domain_test.go::TestDiscoveryImportRecordRejectsStaleRecordWithoutPublishingSuccess`
- `tests/integration/discovery/domain_test.go::TestDiscoveryResolveRecordAndServiceDoNotReturnUsableRoutesAfterStop`
- `tests/integration/discovery/degraded_test.go::TestDiscoveryDegradesWhenBootstrapPeerIsUnavailable`
- `tests/integration/discovery/degraded_test.go::TestDiscoverySnapshotTracksBootstrapPeerLossAfterStartup`
- `tests/integration/discovery/degraded_test.go::TestDiscoveryDegradesWhenBootstrapRecordIsInvalid`
- `tests/integration/discovery/degraded_test.go::TestDiscoveryPersistsDegradedStateAcrossRestart`
- `tests/integration/discovery/degraded_test.go::TestDiscoveryRefreshesLocalPublicationBeforeTTLExpiry`

## False Positive Risk

- тесты не должны ограничиваться проверкой `state != failed`; нужны явные assertions по `Outcome`,
  `PrimaryReason`, refreshed expiry и сохранённым endpoints.

## False Negative Risk

- degraded tests не должны падать из-за слишком коротких дедлайнов; ожидание должно быть ограничено, но устойчиво к runtime scheduling.

## Notes

- сценарий забирает single-node и degraded discovery coverage из `internal/node/node_discovery_test.go`.

