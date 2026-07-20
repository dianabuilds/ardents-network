## Scenario ID

`NFI-003`

## Layer

`integration`

## Domain

`Network Foundation / Store And Subscriptions`

## Category

Store-backed private-envelope retrieval, discovery withdrawal precedence after
authorized decryption, persistent backing-file creation, and relay subscription
fan-out across multiple content topics.

## Goal

Подтвердить, что canonical transport owner не только поднимает relay/bootstrap
path, но и сохраняет store/subscription truth как самостоятельный observable
runtime contract.

## Preconditions

- transport runtime использует canonical `Waku` relay/store path;
- remote peer может публиковать discovery entries и принимать relay
  subscriptions;
- local peer bootstrap-ится через observed endpoints удалённого peer.

## Steps

1. Запустить удалённый `transport.Service`.
2. Для store path опубликовать discovery entries, включая вариант с
   последующим withdrawal entry.
3. Запустить локальный transport с bootstrap endpoints удалённого peer.
4. Выполнить store-backed fetch с локального peer.
5. Для subscription path подписать удалённый peer на два content topic.
6. Опубликовать оба content topic с локального peer и подтвердить доставку.
7. Для persistence path запустить transport с явным `StorePath` и проверить
   создание backing file.

## Expected Result

- store fetch возвращает опубликованную discovery entry;
- при наличии withdrawal entry store truth отражает последнюю запись, а не
  устаревший endpoint set;
- relay subscription получает payload для всех заявленных content topics;
- persistent store создаёт реальный backing file.

## Failure/Degraded Variant

- store fetch не должен возвращать ложноположительный success без реально
  опубликованных записей;
- withdrawal precedence не должна теряться при чтении store history;
- multi-topic subscription не должна silently drop один из content topics;
- persistence path не должен объявляться рабочим без фактического backing file.

## Related Tests

- `tests/integration/network-foundation/transport_store_test.go::TestTransportStoreFetchesPublishedDiscoveryRecord`
- `tests/integration/network-foundation/transport_store_test.go::TestTransportStoreKeepsLatestWithdrawalEntry`
- `tests/integration/network-foundation/transport_store_test.go::TestTransportPersistentStoreCreatesBackingFile`
- `tests/integration/network-foundation/transport_subscriptions_test.go::TestTransportSubscriptionDeliversMultipleContentTopics`

## False Positive Risk

- проверка только отсутствия ошибки без проверки содержимого fetched records
  может скрыть drift в store truth;
- проверка только факта delivery без проверки обоих content topics может скрыть
  regression в subscription filter path.

## False Negative Risk

- store и relay propagation асинхронны; ожидание должно быть bounded, но
  устойчивым к scheduler jitter;
- persistent file creation должна проверяться после завершения startup, а не до
  готовности runtime.

## Notes

- Сценарий закрывает самостоятельный repository-level integration слой для
  store/subscription поведения, вынесенный из mixed `internal/transport`
  tagged tests.
