# Домен Hosted Services

## Назначение

`Hosted Services` отвечает за продуктовую модель сервисов, которые узел способен
экспортировать в сеть.

В варианте `C` этот домен больше не владеет publication outcome. Он владеет
service inventory, readiness, exposure eligibility и связью сервиса с runtime backing.

## Границы Ответственности

Домен отвечает за:

- каноническую модель hosted service;
- inventory hosted services;
- service readiness;
- exposure eligibility;
- связь сервиса с supporting workload/data/network conditions;
- service-facing snapshots для local control surface.

Домен не отвечает за:

- discovery record ownership;
- сетевую публикацию presence/service records;
- transport delivery;
- node lifecycle orchestration.

## Authoritative Truth

- inventory hosted services;
- desired service exposure intent;
- observed service readiness;
- exposure eligibility;
- причины непубликуемости сервиса из-за отсутствия runtime backing.

## Не-Цели

- быть спрятанной частью `Workload Control`;
- владеть discovery records;
- владеть publication transport details;
- дублировать data или transport truth.

## Ключевые Инварианты

- у сервиса есть канонический readiness state;
- сервис не может считаться eligible for publication без runtime backing;
- service readiness должна быть explainable через зависимости;
- publication outcome не должен снова размазываться между `Hosted Services`,
  `Discovery` и `Node Runtime`.

## Входные Контракты

- `RegisterService`
- `UpdateServiceBacking`
- `WithdrawServiceExposure`
- `GetServiceStatus`
- `ListServices`

## Команды И Запросы

### Commands

- `RegisterService`
  Регистрирует service spec в canonical hosting inventory.
- `UpdateServiceBacking`
  Обновляет runtime-backed readiness и exposure eligibility сервиса.
- `WithdrawServiceExposure`
  Снимает exposure intent на уровне hosting truth.

### Queries

- `GetServiceStatus`
  Возвращает service readiness и exposure eligibility snapshot.
- `ListServices`
  Возвращает canonical hosting inventory.

## Выходные Контракты

- service readiness и exposure eligibility для `Publication`;
- service status для `Node Runtime`, diagnostics и local control surface.

## Authoritative Results

- `ServiceRegistrationResult`
- `ServiceBackingUpdateResult`
- `ServiceWithdrawResult`
- `ServiceStatusSnapshot`
- `ServiceInventorySnapshot`

## Публикуемые События И State Outputs

### Domain Events

- `ServiceRegistered`
- `ServiceBackingChanged`
- `ServiceReadinessChanged`
- `ServiceExposureWithdrawn`
- `ServicePublicationEligibilityChanged`

### State Outputs

- `ServiceStatusSnapshot`
  Потребители: `Publication`, `Node Runtime`, `Diagnostics`, local control surface.
- `ServiceInventorySnapshot`
  Потребители: local control surface и runtime assembly.

## Фасад Домена

Фасад должен предоставлять:

- register service;
- update service backing;
- withdraw service exposure;
- list and inspect service status.

Фасад не должен раскрывать:

- publication pipeline internals;
- process controllers;
- discovery record adapters;
- persistence rows как публичный shape.

## Привязка К Boundary Слоям

- canonical local control surface получает inventory и readiness только через
  `Hosted Services` facade;
- `Publication` получает из этого домена service publication inputs, но не reverse-owns
  hosting truth;
- boundary transports не читают hosting internals напрямую.

## Целевая Структура Каталогов

```text
internal/hosting/
  api/
  service/
  readiness/
  exposure/
  registry/
```

## Правила Внутренней Структуры

- `internal/hosting/api` является единственным публичным контрактом домена;
- `service`, `readiness`, `exposure`, `registry` содержат только hosting-owned truth;
- publication coordination и discovery publication не живут внутри `internal/hosting`
  как скрытый owner.

## Правила Зависимостей

- `internal/hosting` не зависит от `boundary/*`;
- `internal/hosting` не владеет discovery record model;
- boundary transports и local control surface зависят только от `internal/hosting/api`;
- `Publication` потребляет outputs `Hosted Services`, но publication truth не возвращается
  в `hosting` как owner-owned state.

## QA Этапы

The concrete `v1` readiness/liveness protocol, generation ownership proof,
thresholds, stale-result rules, and diagnostics contract are defined in
`docs/hosted-service-probe-model.md`.

Unit:

- readiness rules;
- exposure eligibility rules;
- withdraw behavior;
- dependency-based denial reasons.

Integration:

- hosted service корректно зависит от workload/data/network conditions;
- `Publication` получает publication inputs от `Hosted Services`, а не генерирует service
  meaning самостоятельно.

E2E:

- оператор видит service inventory, readiness и publication eligibility через
  canonical control surface.

## Правила Миграции

## Актуальное Состояние

`Hosted Services` уже владеет:

- service model;
- service readiness ownership;
- exposure eligibility ownership.

В домен не должны возвращаться:

- ownership-зона между `internal/workload/services` и `internal/node/publication`;
- старые service publication facades без доменного владельца;
- bridge-слои, сохраняющие hosted services скрытыми внутри workload/runtime.
