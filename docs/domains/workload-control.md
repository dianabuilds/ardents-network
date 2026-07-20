# Домен Workload Control

## Назначение

`Workload Control` отвечает за управление жизненным циклом workloads и за
authoritative truth о желаемом и наблюдаемом execution-состоянии.

Инфраструктурный контракт исполнения, поддерживаемые платформы и граница
доверия зафиксированы в `docs/workload-execution-platform.md`. Они не меняют
доменное ownership: Docker Engine и OCI runtime являются адаптерами исполнения,
а не вторым scheduler или источником workload truth.

Это не общий runtime scheduler для всего процесса узла.

## Границы Ответственности

Домен отвечает за:

- регистрацию workloads;
- desired/observed workload state;
- запуск, остановку и рестарт workload;
- policy-aware execution decisions;
- inventory workloads и их runtime status.

Домен не отвечает за:

- network participation;
- discovery ownership;
- data ownership;
- node lifecycle ownership;
- service publication semantics и publication outcome, которые принадлежат
  `Hosted Services` и `Publication`.

## Authoritative Truth

- workload registry;
- desired workload state;
- observed workload state;
- execution results и failure reasons;
- workload inventory and status snapshots.

## Не-Цели

- смешивать workload execution с hosted-service readiness и publication;
- владеть operator boundary contracts;
- хранить вторую параллельную модель runtime truth вне домена.

## Ключевые Инварианты

- у workload один authoritative desired state и один observed state;
- failed execution должна быть объяснимой;
- workload readiness не равна node readiness;
- hosted-service semantics не должны оставаться скрытыми внутри workload как
  permanent ownership.

## Входные Контракты

- `RegisterWorkload`
- `StartWorkload`
- `StopWorkload`
- `RestartWorkload`
- `GetWorkloadStatus`
- `ListWorkloads`

## Команды И Запросы

### Commands

- `RegisterWorkload`
  Регистрирует workload intent и создает canonical workload inventory entry.
- `StartWorkload`
  Запускает workload execution transition.
- `StopWorkload`
  Останавливает workload execution transition.
- `RestartWorkload`
  Выполняет controlled restart для workload.

### Queries

- `GetWorkloadStatus`
  Возвращает workload-owned desired/observed status snapshot.
- `ListWorkloads`
  Возвращает canonical workload inventory.

## Выходные Контракты

- workload status для `Node Runtime` и control surface;
- execution state для `Hosted Services`, если сервис опирается на workload;
- explainable workload failures для diagnostics.

## Authoritative Results

- `WorkloadRegistrationResult`
- `WorkloadStartResult`
- `WorkloadStopResult`
- `WorkloadRestartResult`
- `WorkloadStatusSnapshot`
- `WorkloadInventorySnapshot`

## Публикуемые События И State Outputs

### Domain Events

- `WorkloadRegistered`
- `WorkloadStarted`
- `WorkloadStopped`
- `WorkloadRestarted`
- `WorkloadFailed`
- `WorkloadStatusChanged`

### State Outputs

- `WorkloadStatusSnapshot`
  Потребители: `Node Runtime`, `Hosted Services`, `Diagnostics`, control surface.
- `WorkloadInventorySnapshot`
  Потребители: canonical local control surface, transport bindings.

## Фасад Домена

Фасад должен предоставлять:

- register;
- start;
- stop;
- restart;
- workload inventory/status snapshots.

## Что Не Должно Протекать Через Фасад

- scheduler queues, executor handles и runtime worker internals;
- internal workload state machines, retry bookkeeping и controller loops;
- persisted execution journals и adapter-specific storage rows;
- legacy workload DTO и compatibility orchestration wrappers;
- прямые infrastructure executor/network/data adapter types.

## Привязка К Boundary Слоям

- canonical local control surface использует `Workload Control` facade для
  workload commands, status и execution summaries.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings переводят
  workload requests в те же workload-owned контракты и не вызывают scheduler,
  executor или persistence internals напрямую.
- transport bindings не читают internal queues, worker state или legacy
  execution DTO напрямую.

## Целевая Структура Каталогов

```text
internal/workload/
  api/
  workload/
  desiredstate/
  observedstate/
  execution/
  registry/
  controller/
```

## Правила Внутренней Структуры

- `internal/workload/api` является обязательным каноническим публичным
  контрактом домена.
- `workload`, `desiredstate`, `observedstate` и `execution` содержат только
  workload-owned truth.
- `registry` и `controller` управляют inventory и execution transitions без
  создания обязательной параллельной формы `domain/application/infrastructure`.
- runtime runner details не становятся частью публичного workload contract.

## Правила Зависимостей

- `internal/workload` не зависит от `boundary/*`;
- `internal/workload` не зависит от `Hosted Services` model как от собственной
  truth;
- `Hosted Services`, transport bindings и local control surface читают workload
  status только через `internal/workload/api`.

## QA Этапы

Unit:

- desired/observed transitions;
- start/stop/restart rules;
- failure reason mapping;
- registry behavior.

Integration:

- workload controller корректно взаимодействует с policy, data и diagnostics;
- node runtime получает достоверный workload status.

E2E:

- оператор регистрирует и управляет workloads через canonical control surface;
- сломанный workload отображается как explainable degraded/failure state.

## Правила Миграции

## Актуальное Состояние

`Workload Control` уже владеет:

- workload registry;
- execution lifecycle;
- workload status ownership.

В домен не должны возвращаться:

- скрытая ownership-зона hosted services внутри workload, если она не относится
  к execution truth;
- старые mixed models desired/observed state;
- compatibility paths, которые удерживают legacy runtime shape.
