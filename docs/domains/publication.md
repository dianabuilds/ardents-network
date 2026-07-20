# Домен Publication

## Назначение

`Publication` отвечает за локальную публикацию node presence и hosted-service presence
в сеть Ardents.

Это отдельный продуктовый домен варианта `C`. Он владеет publication intent,
publication outcome и explainable state локальной сетевой публикации.

## Границы Ответственности

Домен отвечает за:

- publication intent локального presence;
- publication intent локальных hosted services;
- publication eligibility на основе inputs из `Node Runtime`, `Hosted Services`,
  `Discovery`, `Policy` и `Network Foundation / Messaging`;
- publication outcome и его explainable status;
- withdraw и compensation flows при потере runtime truth или network capability.

Домен не отвечает за:

- discovery record ownership как knowledge domain;
- transport substrate ownership;
- hosted-service model;
- node lifecycle ownership;
- boundary-local control transport.

## Authoritative Truth

- local node publication intent;
- local service publication intent;
- publication status;
- publication outcome;
- publication denial и withdraw reasons;
- last successful publication / withdrawal state.

## Не-Цели

- быть подфункцией `Node Runtime`;
- быть спрятанным helper-слоем внутри `Discovery` или `Hosted Services`;
- владеть remote discovery knowledge;
- заменять network transport reality локальными эвристиками.

## Ключевые Инварианты

- presence/service publication не существует без runtime backing;
- publication outcome должен быть explainable;
- publication denial должен быть operator-visible;
- publication truth отделена от hosted-service readiness truth и discovery knowledge truth;
- withdraw выполняется при потере runtime truth, policy permission или transport capability.

## Входные Контракты

- `PublishLocalPresence`
- `PublishLocalService`
- `RefreshPublication`
- `WithdrawPublication`
- `GetPublicationStatus`
- `ListPublishedServices`

## Команды И Запросы

### Commands

- `PublishLocalPresence`
  Публикует локальное presence в сеть на основе node-owned и policy-allowed truth.
- `PublishLocalService`
  Публикует hosted service в сеть на основе service readiness и policy/network eligibility.
- `RefreshPublication`
  Переоценивает publication intent и приводит network-visible состояние к актуальной truth.
- `WithdrawPublication`
  Снимает presence/service publication при operator command или потере backing truth.

### Queries

- `GetPublicationStatus`
  Возвращает текущий publication status и причины ограничений.
- `ListPublishedServices`
  Возвращает локальные services с publication outcome.

## Выходные Контракты

- publication status для `Node Runtime`, diagnostics и local control surface;
- discovery-ready local publication facts для `Discovery`;
- transport publication actions для `Network Foundation / Messaging`.

## Authoritative Results

- `PresencePublicationResult`
- `ServicePublicationResult`
- `PublicationRefreshResult`
- `PublicationWithdrawResult`
- `PublicationStatusSnapshot`
- `PublishedServiceSnapshot`

## Публикуемые События И State Outputs

### Domain Events

- `PresencePublicationRequested`
- `PresencePublished`
- `ServicePublicationRequested`
- `ServicePublished`
- `PublicationWithdrawn`
- `PublicationDenied`
- `PublicationDegraded`

### State Outputs

- `PublicationStatusSnapshot`
  Потребители: `Node Runtime`, `Diagnostics`, local control surface.
- `PublishedServiceSnapshot`
  Потребители: local control surface, diagnostics, runtime assembly.

## Фасад Домена

Фасад должен предоставлять:

- publish presence;
- publish service;
- refresh publication;
- withdraw publication;
- query publication status.

Фасад не должен раскрывать:

- transport adapter handles;
- discovery store internals;
- runtime wiring;
- compatibility DTO из старой node-publication формы.

## Привязка К Boundary И Соседним Доменам

- `Publication` потребляет node readiness из `Node Runtime`;
- `Publication` потребляет service readiness и eligibility inputs из `Hosted Services`;
- `Publication` использует record model и validation rules `Discovery`, не становясь owner
  remote discovery knowledge;
- `Publication` использует transport capability `Network Foundation / Messaging`, не
  становясь owner transport substrate;
- canonical local control surface получает publication truth через `Publication` facade.

## Целевая Структура Каталогов

```text
internal/publication/
  api/
  intent/
  status/
  refresh/
  withdraw/
  sync/
```

## Правила Внутренней Структуры

- `internal/publication/api` является единственным публичным контрактом домена;
- `intent`, `status`, `refresh`, `withdraw`, `sync` содержат publication-owned truth
  и логику;
- текущая v1-реализация ещё может содержать root-level файлы внутри `internal/publication`
  до отдельного package-level convergence, но ownership и API truth уже принадлежат
  самому домену `Publication`;
- publication coordination не живёт внутри `internal/node`, `internal/hosting`
  или `internal/discovery` как скрытый owner.

## Правила Зависимостей

- `internal/publication` не зависит от `boundary/*`;
- boundary transports и local control surface зависят только от `internal/publication/api`;
- `internal/publication` зависит от API соседних доменов, а не от их внутренних subpackages.

## QA Этапы

The concrete `v1` service eligibility, endpoint-pairing, reachability,
withdrawal, and recovery gate is defined in
`docs/hosted-service-publication-gate.md`.

Unit:

- publication eligibility rules;
- withdraw rules;
- compensation rules;
- status and denial reason assembly.

Integration:

- publication корректно реагирует на loss of workload backing;
- publication корректно реагирует на transport degradation;
- discovery получает локальные publication facts через publication-owned flow.

E2E:

- оператор видит publication status и причины withdraw/denial через canonical local
  control surface.

## Правила Миграции

## Актуальное Состояние

`Publication` уже является owner для:

- local presence publication ownership;
- local service publication ownership;
- publication status ownership;
- compensation и withdraw ownership.

В домен не должны возвращаться:

- `internal/node/publication`;
- смешанные publication helper-слои в `internal/node/process` и `internal/node/authority`;
- скрытая publication ownership внутри `Hosted Services` и `Discovery`.
