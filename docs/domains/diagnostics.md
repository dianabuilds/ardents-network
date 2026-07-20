# Домен Diagnostics

## Назначение

`Diagnostics` отвечает за explainability runtime-состояния системы.

Домен делает состояния, события, деградации и отказные причины наблюдаемыми для
оператора и соседних доменов.

## Границы Ответственности

Домен отвечает за:

- health model;
- operation/event recording;
- explainability snapshots;
- причинные связи degraded/failed states;
- diagnostics-facing history и summaries.

Домен не отвечает за:

- владение предметной логикой соседних доменов;
- node lifecycle ownership;
- policy ownership;
- boundary transport ownership.

## Authoritative Truth

- diagnostics event stream;
- operation ledger;
- health summaries;
- degraded/failed reason snapshots;
- explainability projections.

## Не-Цели

- становиться логгером общего назначения;
- подменять authoritative truth соседних доменов;
- держать health как набор несвязанных строк без модели причин.

## Ключевые Инварианты

- значимая деградация должна быть наблюдаемой и объяснимой;
- diagnostics не может silently терять critical failure path;
- diagnostics projection должна ссылаться на реальные domain-owned причины, а не
  придумывать их сама.

## Входные Контракты

- `RecordEvent`
- `RecordOperation`
- `UpdateHealth`
- `GetHealthSnapshot`
- `GetDiagnosticsTimeline`

## Команды И Запросы

### Commands

- `RecordEvent`
  Записывает domain-relevant diagnostics event в canonical diagnostics ledger.
- `RecordOperation`
  Записывает operation state transition в operation ledger.
- `UpdateHealth`
  Обновляет diagnostics-owned health projection на основании domain facts.

### Queries

- `GetHealthSnapshot`
  Возвращает canonical health summary.
- `GetDiagnosticsTimeline`
  Возвращает canonical diagnostics timeline.

## Выходные Контракты

- explainability snapshot для control surface;
- operation and failure context для оператора и transport bindings;
- health summary для `Node Runtime` и соседних доменов.

## Authoritative Results

- `DiagnosticsEventResult`
- `DiagnosticsOperationResult`
- `HealthUpdateResult`
- `HealthSnapshot`
- `DiagnosticsTimeline`

## Публикуемые События И State Outputs

### Domain Events

- `DiagnosticsEventRecorded`
- `DiagnosticsOperationRecorded`
- `HealthUpdated`
- `DiagnosticsReasonChanged`

### State Outputs

- `HealthSnapshot`
  Потребители: `Node Runtime`, canonical local control surface,
  transport bindings.
- `DiagnosticsTimeline`
  Потребители: control surface, operator-facing application flows.
- `HealthUpdateResult`
  Потребители: application diagnostics flows и neighboring domains that await
  explainability confirmation.

## Фасад Домена

Фасад должен предоставлять:

- record event;
- record operation;
- update health projection;
- get health and diagnostics summaries.

## Что Не Должно Протекать Через Фасад

- внутренние event ledgers, correlation indexes и projection rebuild mechanics;
- domain-private facts, которые diagnostics только проецирует, но не владеет ими;
- storage rows и retention internals explainability persistence;
- legacy string-only health DTO и compatibility logging envelopes;
- прямые subscription handles и infrastructure observer adapters.

## Привязка К Boundary Слоям

- canonical local control surface использует `Diagnostics` facade для health,
  explainability и operation views.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings экспортируют
  diagnostics reads через те же diagnostics-owned контракты и не подключаются
  к ledgers, projection stores или observer internals напрямую.
- transport bindings не обходят canonical surface ради прямого доступа к event
  ledgers, projection rebuild tools или legacy health DTO.

## Целевая Структура Каталогов

```text
internal/diagnostics/
  api/
  event/
  operation/
  health/
  reason/
  timeline/
  recorder/
  projection/
```

## Правила Внутренней Структуры

- `internal/diagnostics/api` является обязательным каноническим публичным
  контрактом домена.
- `event`, `operation`, `health`, `reason` и `timeline` содержат только
  diagnostics-owned truth и vocabulary.
- `recorder` и `projection` допустимы только как diagnostics-owned machinery и
  не создают отдельный contract world.
- `internal/diagnostics` не использует обязательную целевую форму
  `domain/application/infrastructure`.

## Правила Зависимостей

- `internal/diagnostics` не зависит от `boundary/*`;
- соседние домены поставляют facts, но не владеют diagnostics projection;
- transport bindings и local control surface получают diagnostics summaries
  только через `internal/diagnostics/api`.

## QA Этапы

Unit:

- event recording;
- operation ledger rules;
- health projection updates;
- reason assembly.

Integration:

- node, network, workload, data и policy отдают explainable facts в diagnostics;
- degraded and failed states не теряются при restart/recovery.

E2E:

- оператор видит timeline, health и причины отказов через canonical control
  surface без необходимости читать сырые логи.

## Правила Миграции

## Актуальное Состояние

`Diagnostics` уже владеет:

- health model;
- operation/event recording;
- explainability projections.

В домен не должны возвращаться:

- ad-hoc diagnostics helpers, не имеющие общей reason model;
- старые health snapshots, которые не выражают причинность;
- compatibility diagnostics paths, удерживающие legacy wording и legacy truth.
