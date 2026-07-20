## Scenario ID

`NFI-001`

## Layer

`integration`

## Domain

`Network Foundation / Messaging`

## Category

Waku relay/store runtime path, bootstrap connectivity, degraded peer loss.

## Goal

Проверить, что transport owner реально использует Waku relay/store path:
поднимает peer connectivity через bootstrap, доставляет relay envelope, читает
discovery records из store и явно деградирует после потери bootstrap peer.

## Preconditions

- transport runtime использует canonical `Waku` path без fake substitute;
- remote peer стартует с реальным relay/store capability set;
- local peer bootstrap-ится только через observed endpoints удалённого peer.

## Steps

1. Запустить удалённый `transport.Service`.
2. Подписать удалённый peer на relay content topic.
3. Опубликовать discovery entry в remote store.
4. Запустить локальный transport с bootstrap endpoints удалённого peer.
5. Дождаться relay peer readiness и `BootstrapStatus = ready`.
6. Опубликовать relay envelope с локального peer и подтвердить доставку на удалённый.
7. Выполнить store-backed fetch discovery records с локального peer.
8. Остановить удалённый peer и подтвердить degraded bootstrap status у локального.

## Expected Result

- relay publish/subscribe работает через реальный Waku path;
- store-backed fetch возвращает опубликованную remote discovery entry;
- bootstrap status отражает joined/ready до потери peer;
- после peer loss transport state и bootstrap status переходят в `degraded`.

## Failure/Degraded Variant

- transport не должен оставаться `ready`, если bootstrap peer пропал;
- store fetch не должен возвращать ложноположительный success без реально опубликованной записи.

## Related Tests

- `tests/integration/network-foundation/transport_flow_test.go::TestTransportRelayPublishSubscribeAndStoreFetch`
- `tests/integration/network-foundation/transport_bootstrap_test.go::TestTransportBootstrapStatusDegradesAfterPeerLoss`
- `tests/integration/network-foundation/transport_bootstrap_test.go::TestTransportBootstrapDialFailureIsReported`
- `internal/network/transport/dns_discovery_integration_test.go::TestSignedDNSColdStartAndPeerRestartRecovery`
- `internal/network/transport/dns_discovery_integration_test.go::TestSignedDNSReplenishesToRelayPeerTarget`

## False Positive Risk

- тест обязан проверять не только отсутствие ошибки, но и реальный relay payload,
  fetched record subject и явный `degraded` state после потери peer.

## False Negative Risk

- peer join и degradation зависят от асинхронного runtime; ожидание должно быть
  bounded, но устойчивым к scheduler jitter.

## Notes

- сценарий выделяет network-foundation coverage из mixed `internal/transport`
  package tests в explicit integration layer.
