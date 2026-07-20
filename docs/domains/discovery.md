# Домен Discovery

## Назначение

`Discovery` отвечает за knowledge domain о существующих узлах и сервисах в сети и за
разрешение этого знания в usable results.

В варианте `C` `Discovery` больше не владеет локальной publication policy и publication
status. Он владеет record model, intake, validation, trust-aware resolution и remote/local
knowledge state.

## Границы Ответственности

Домен отвечает за:

- discovery record model;
- intake внешних записей;
- validation, merge и conflict handling;
- freshness, source и trust semantics;
- разрешение nodes и services из discovery knowledge;
- inventory локальных и удалённых discovery facts.

Домен не отвечает за:

- transport delivery и raw Waku participation;
- publication intent и publication outcome локального узла;
- hosted-service readiness;
- node lifecycle orchestration;
- presentation DTO.

## Authoritative Truth

- canonical discovery record model;
- inventory discovery records;
- source/freshness/trust metadata;
- resolve/useability decisions;
- local knowledge state после intake/publish sync.

## Не-Цели

- хранить сетевой мусор без trust semantics;
- быть local publication orchestrator;
- дублировать hosted-service truth;
- дублировать transport readiness truth.

## Ключевые Инварианты

- каждая используемая запись должна иметь верифицируемое происхождение;
- stale или untrusted запись не может считаться canonical usable result;
- local publication facts и resolved remote truth не должны смешиваться без source semantics;
- `Discovery` не становится owner publication status только потому, что records публикуются
  через discovery-shaped data.

## Входные Контракты

- `IngestRemoteRecord`
- `UpsertLocalRecord`
- `ResolveNode`
- `ResolveService`
- `ListDiscoveryState`

## Команды И Запросы

### Commands

- `IngestRemoteRecord`
  Выполняет verify, merge и trust-aware intake удалённой записи.
- `UpsertLocalRecord`
  Принимает validated local publication fact и синхронизирует его с discovery-owned record model.

### Queries

- `ResolveNode`
  Возвращает usable node result из discovery knowledge.
- `ResolveService`
  Возвращает usable service result из discovery knowledge.
- `ListDiscoveryState`
  Возвращает inventory discovery facts и их metadata.

## Выходные Контракты

- resolved node presence для `Node Runtime` и local control surface;
- resolved service endpoints для `Hosted Services`, `Publication` и clients;
- explainable discovery state для diagnostics.

## Authoritative Results

- `RemoteRecordIngestResult`
- `LocalRecordUpsertResult`
- `ResolvedNodeResult`
- `ResolvedServiceResult`
- `DiscoveryStateSnapshot`

## Публикуемые События И State Outputs

### Domain Events

- `RemoteRecordIngested`
- `RemoteRecordRejected`
- `LocalRecordUpserted`
- `DiscoveryStateChanged`
- `ServiceResolved`

### State Outputs

- `ResolvedNodeResult`
  Потребители: `Node Runtime`, local control surface, diagnostics.
- `ResolvedServiceResult`
  Потребители: `Hosted Services`, `Publication`, client-facing flows.
- `DiscoveryStateSnapshot`
  Потребители: diagnostics, local control surface.

## Фасад Домена

Фасад должен предоставлять:

- intake и merge удалённых записей;
- sync локальных publication facts в discovery model;
- resolve узлов и сервисов;
- выдачу discovery state snapshots.

Фасад не должен раскрывать:

- persisted storage format как публичный контракт;
- transport payload details;
- publication policy internals;
- compatibility catalog DTO.

## Привязка К Boundary Слоям

- canonical local control surface получает resolve/discovery state только через
  `Discovery` facade;
- `Publication` поставляет validated local publication facts в `Discovery`, но не reverse-owns
  discovery knowledge;
- boundary transports не обходят `internal/discovery/api`.

## Целевая Структура Каталогов

```text
internal/discovery/
  api/
  record/
  source/
  freshness/
  intake/
  resolution/
  state/
  trust/
```

## Правила Внутренней Структуры

- `internal/discovery/api` является единственным публичным контрактом домена;
- `record`, `source`, `freshness`, `intake`, `resolution`, `trust` содержат только
  discovery-owned semantics;
- `state` допустим как support package для persistence и knowledge snapshot machinery,
  пока он не становится отдельным ownership-слоем;
- publication coordination не живёт внутри `internal/discovery` как скрытый owner.

## Правила Зависимостей

- `internal/discovery` не зависит от `boundary/*`;
- `internal/discovery` не использует transport adapter implementation как доменную модель;
- boundary transports и local control surface зависят только от `internal/discovery/api`;
- `Publication` и `Node Runtime` получают только outputs, принадлежащие `internal/discovery/api`.

## QA Этапы

Unit:

- verify и reject невалидных record shapes;
- freshness semantics;
- merge/conflict handling;
- resolve и usability rules.

Integration:

- discovery принимает network-fed records и выдаёт trust-aware resolved state;
- publication sync корректно превращается в discovery-visible local knowledge;
- diagnostics получает explainable причины unusable records.

E2E:

- оператор видит peers, local presence knowledge и service resolution через canonical
  control surface;
- stale или invalid records не приводят к ложной ready-state картине.

## Правила Миграции

## Актуальное Состояние

После завершённого structure convergence `Discovery` уже владеет:

- record model;
- intake и merge ownership;
- trust-aware resolution semantics;
- local knowledge state и его persistence support.

В домен не должны возвращаться:

- local publication ownership внутри `Discovery`;
- legacy catalog/read-model DTO;
- compatibility publication helpers, удерживающие старую широкую node/publication форму.
