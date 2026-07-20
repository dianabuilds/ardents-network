# Домен Network Foundation / Messaging

## Назначение

`Network Foundation / Messaging` отвечает за каноническое сетевое участие узла
на базе Waku и за authoritative truth о transport-состоянии.

Это не абстрактный interchangeable transport core.

## Границы Ответственности

Домен отвечает за:

- участие узла в Waku-сети;
- transport readiness и connectivity truth;
- publish/subscribe/relay behavior в пределах продуктовой модели;
- delivery каналов для discovery, publication и runtime signaling;
- network-facing snapshots и статусы.

Домен не отвечает за:

- discovery semantics;
- data ownership;
- identity ownership;
- hosted service ownership;
- control boundary DTO.

## Authoritative Truth

- transport lifecycle state;
- relay/store/filter/lightpush participation state;
- messaging readiness;
- publish/subscribe outcomes;
- route usability в рамках transport truth.

## Не-Цели

- прятать Waku за fake-neutral adapter как часть target architecture;
- дублировать discovery ownership;
- становиться второй node lifecycle model.

## Ключевые Инварианты

- Waku остается канонической network foundation;
- transport readiness обязан отражать реальное network participation state;
- publish success и delivery readiness не должны подменяться optimistic flags;
- домен не должен владеть record semantics, которые принадлежат `Discovery`.

## Входные Контракты

- `StartTransport`
- `StopTransport`
- `GetTransportStatus`
- `PublishMessage`
- `SubscribeMessages`
- `GetPeerView`

## Команды И Запросы

### Commands

- `StartTransport`
  Поднимает Waku-backed transport participation и инициирует transport
  lifecycle.
- `StopTransport`
  Завершает transport participation и переводит домен в terminal transport
  state.
- `PublishMessage`
  Выполняет publish через canonical Waku-backed delivery path.

### Queries

- `GetTransportStatus`
  Возвращает canonical transport readiness и participation snapshot.
- `SubscribeMessages`
  Открывает доменный subscribe path для network-delivered messages.
- `GetPeerView`
  Возвращает transport-owned peer and route view.

## Выходные Контракты

- transport readiness для `Node Runtime`;
- delivery path для `Discovery`, `Hosted Services` и других network-aware
  доменов;
- explainable messaging status для diagnostics, control surfaces и transport
  bindings.

## Authoritative Results

- `TransportStartResult`
  Возвращает transport participation outcome и достигнутый readiness state.
- `TransportStopResult`
  Возвращает terminal transport shutdown outcome.
- `TransportStatusSnapshot`
  Возвращает authoritative transport readiness, capability, and participation
  summary.
- `PublishResult`
  Возвращает network-owned publish outcome без optimistic replacement flags.
- `PeerViewSnapshot`
  Возвращает canonical peer and route usability snapshot.

## Публикуемые События И State Outputs

### Domain Events

- `TransportStarting`
- `TransportReady`
- `TransportDegraded`
- `TransportFailed`
- `TransportStopped`
- `MessagePublished`
- `MessagePublishFailed`
- `PeerViewChanged`

### State Outputs

- `TransportStatusSnapshot`
  Потребители: `Node Runtime`, `Diagnostics`, canonical local control surface.
- `PeerViewSnapshot`
  Потребители: `Node Runtime`, `Discovery`, control surface.
- `PublishResult`
  Потребители: `Discovery`, `Hosted Services`, другие network-aware domains.

## Фасад Домена

Фасад должен предоставлять:

- start/stop transport participation;
- publish/subscribe use cases;
- peer and readiness snapshots;
- transport diagnostics-friendly status.

## Что Не Должно Протекать Через Фасад

- raw Waku node/relay/store/filter/lightpush adapters и их handles;
- peer manager internals, routing caches и transport wiring details;
- codec/persistence implementation shapes;
- fake-neutral transport abstractions и legacy compatibility envelopes;
- внутренние publish pipeline state и retry bookkeeping.

## Привязка К Boundary Слоям

- canonical local control surface читает transport status, peer view и publish
  outcomes через `Network Foundation / Messaging` facade.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings проксируют
  network-oriented requests в те же network-owned контракты и не зависят от
  transport internals напрямую.
- transport bindings не получают прямой доступ к Waku-specific adapters, peer
  caches или raw subscription internals.

## Целевая Структура Каталогов

```text
internal/network/
  api/
  transport/
  peer/
  route/
  readiness/
  messaging/
  participation/
```

## Правила Внутренней Структуры

- `internal/network/api` является обязательным каноническим публичным
  контрактом домена.
- `transport`, `peer`, `route` и `readiness` хранят transport truth.
- `messaging` и `participation` описывают только network-owned операции и не
  раскладываются в обязательную форму `domain/application/infrastructure`.
- Waku-specific adapter details не являются частью публичного domain contract.

## Правила Зависимостей

- `internal/network` не зависит от `Discovery` model как от собственной truth;
- upstream Waku libraries остаются substrate dependency, а не доменной моделью;
- transport bindings и local control surface зависят только от
  `internal/network/api`;
- transport-local control DTO не должны становиться transport domain model.

## QA Этапы

Unit:

- readiness transitions;
- publish/subscribe result mapping;
- route usability rules;
- peer view rules.

Integration:

- network domain поднимает реальное Waku participation state;
- discovery и hosting получают delivery capability через доменный фасад;
- node runtime корректно видит transport degraded state.

E2E:

- оператор видит реальный network status и messaging readiness через canonical
  control surface;
- broken transport не маскируется под healthy node.

## Правила Миграции

## Актуальное Состояние

`Network Foundation / Messaging` уже владеет:

- transport readiness;
- messaging ownership;
- Waku-backed participation orchestration на доменном уровне.

В домен не должны возвращаться:

- legacy transport facades, которые скрывают реальную substrate роль;
- смешанная route/publication логика, если она не принадлежит network truth;
- compatibility abstractions, делающие вид, что Waku необязателен для `v1`.

## Private Envelope Authority

For `ardents-private/1`, Network Foundation additionally owns:

- fixed outer framing and deterministic protobuf inner wire representation;
- XChaCha20-Poly1305 protection before Relay, Store, Filter, or Lightpush
  carrier entry;
- binding ciphertext to the exact Waku pubsub and opaque content topics;
- size, version, suite, flags, clock, lifetime, padding, and canonical-decoding
  enforcement;
- durable, bounded, fail-closed replay admission across live and retained
  delivery;
- stable privacy failure codes and redaction-safe network outcomes.

Identity remains the authority for local capability resolution and remote
sender grants. Network Foundation does not interpret discovery or data payload
semantics after decryption. Waku substrate logs are routed to a compatible
discarding logger because upstream fields can include raw opaque selectors;
operator truth remains available through Ardents-owned readiness, health,
publish outcomes, and privacy reason codes.
