# Домен Data Substrate

## Назначение

`Data Substrate` отвечает за authoritative truth о данных, которыми владеет
узел: объектах, blob-ах, manifests, retention и transfer-состоянии.

Normative distributed availability, replica lease, repair, and chunking
semantics are defined in `docs/data-availability-replication-semantics.md`.

Это не просто persistence utility.

## Границы Ответственности

Домен отвечает за:

- регистрацию и адресацию data objects;
- blob и manifest ownership;
- retention semantics;
- data transfer state;
- data availability и integrity status.

Домен не отвечает за:

- discovery ownership;
- network transport ownership;
- workload lifecycle ownership;
- generic shared persistence helpers.

## Authoritative Truth

- object inventory;
- blob and manifest relationships;
- retention policy application на уровне data truth;
- transfer state;
- availability and integrity status.

## Не-Цели

- становиться thin wrapper над BoltDB;
- дублировать hosted service publication state;
- хранить вторую модель object truth вне домена.

## Ключевые Инварианты

- объект имеет единственное authoritative описание;
- manifest и blob relationships должны быть консистентными;
- retention не может silently удалять still-authoritative objects;
- transfer result обязан быть объяснимым.

## Входные Контракты

- `PutObject`
- `GetObject`
- `ListObjects`
- `StartTransfer`
- `GetTransferStatus`
- `ApplyRetention`

## Команды И Запросы

### Commands

- `PutObject`
  Создает или обновляет canonical object/blob/manifest truth.
- `StartTransfer`
  Запускает transfer lifecycle для data object.
- `ApplyRetention`
  Применяет retention decision в границах data-owned truth.

### Queries

- `GetObject`
  Возвращает authoritative object availability and integrity state.
- `ListObjects`
  Возвращает canonical object inventory.
- `GetTransferStatus`
  Возвращает transfer lifecycle snapshot.

## Выходные Контракты

- object availability для workloads и hosted services;
- transfer status для node runtime, diagnostics и control surface;
- data integrity status для соседних доменов, которым это критично.

## Authoritative Results

- `ObjectPutResult`
- `ObjectGetResult`
- `ObjectInventorySnapshot`
- `TransferStartResult`
- `TransferStatusSnapshot`
- `RetentionApplyResult`

## Публикуемые События И State Outputs

### Domain Events

- `ObjectStored`
- `ObjectAvailabilityChanged`
- `TransferStarted`
- `TransferCompleted`
- `TransferFailed`
- `RetentionApplied`

### State Outputs

- `ObjectGetResult`
  Потребители: `Workload Control`, `Hosted Services`, canonical local control
  surface.
- `ObjectInventorySnapshot`
  Потребители: control surface, application data flows.
- `TransferStatusSnapshot`
  Потребители: `Node Runtime`, `Diagnostics`, control surface.

## Фасад Домена

Фасад должен предоставлять:

- object put/get/list;
- transfer start/status;
- retention apply/status.

## Что Не Должно Протекать Через Фасад

- storage engine entities, blob layout и manifest persistence rows;
- transfer workers, chunk pipelines и retention scheduler internals;
- substrate-specific adapter handles и filesystem/object-store details;
- legacy repository DTO и compatibility storage envelopes;
- внутренние repair/reconciliation bookkeeping structures.

## Привязка К Boundary Слоям

- canonical local control surface использует `Data Substrate` facade для
  object, manifest, transfer и retention use cases.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings переводят data
  requests в те же data-owned контракты и не работают с storage engines,
  transfer workers или persistence internals напрямую.
- transport bindings не получают прямой доступ к blob layout, chunk pipelines
  или legacy storage DTO.

## Целевая Структура Каталогов

```text
internal/data/
  api/
  object/
  blob/
  manifest/
  transfer/
  retention/
  catalog/
```

## Правила Внутренней Структуры

- `internal/data/api` является обязательным каноническим публичным контрактом
  домена.
- `object`, `blob`, `manifest`, `transfer`, `retention` и `catalog` содержат
  только data-owned truth и use-case semantics этого домена.
- support packages вроде `model`, `payload`, `observed` и `state` допустимы внутри
  `internal/data`, если они обслуживают data-owned truth и не превращаются в отдельный
  contract world.
- `internal/data` не использует обязательную целевую форму
  `domain/application/infrastructure`; дополнительные subpackages допустимы
  только внутри домена и только по ownership.
- storage engine details не становятся частью доменного публичного контракта.

## Правила Зависимостей

- `internal/data` не зависит от `boundary/*`;
- `internal/data` не использует shared persistence helpers как доменную модель;
- transport bindings и local control surface зависят только от
  `internal/data/api`;
- orchestration с network/workload допустима только без переноса ownership в
  другие пакеты.

## QA Этапы

Unit:

- object catalog rules;
- manifest/blob consistency;
- transfer transitions;
- duplicate transfer IDs are rejected and a failed durable transition restores
  the previously observable transfer state;
- retention decisions.

Integration:

- workloads и hosted services получают authoritative object availability;
- diagnostics видит transfer failures и retention effects без скрытых состояний.

E2E:

- оператор может разместить данные, проверить availability и увидеть
  объяснимые transfer/retention состояния через canonical control surface.

## Правила Миграции

## Актуальное Состояние

`Data Substrate` уже владеет:

- object/blob/manifest ownership;
- transfer ownership;
- retention ownership.

В домен не должны возвращаться:

- трактовка `internal/persistence` как будто это и есть data domain;
- legacy storage shapes, если они не соответствуют новой object model;
- compatibility DTO, удерживающие старую data truth.
