# Домен Policy

## Назначение

`Policy` отвечает за продуктовые правила допуска, ограничения и enforcement,
которые влияют на execution decisions соседних доменов.

Это не просто config helper.

## Границы Ответственности

Домен отвечает за:

- policy model;
- policy evaluation;
- enforcement decisions;
- deny reasons и explainability;
- policy summaries для control surface и diagnostics.

Домен не отвечает за:

- identity continuity;
- workload execution ownership;
- discovery ownership;
- boundary request parsing.

## Authoritative Truth

- policy set;
- policy evaluation result;
- enforcement decision;
- deny/allow reason model;
- effective policy snapshot.

## Не-Цели

- подменять собой домены, к которым policy применяется;
- хранить policy как неструктурированный config blob без authoritative model;
- размазывать deny logic по соседним доменам.

## Ключевые Инварианты

- policy decision должна быть повторяемой и объяснимой;
- deny reason обязан быть явным;
- соседние домены не должны иметь собственную primary policy truth;
- policy evaluation не должна silently fallback на legacy behavior.

## Входные Контракты

- `SetPolicy`
- `GetPolicy`
- `EvaluatePolicy`
- `ExplainDecision`

## Команды И Запросы

### Commands

- `SetPolicy`
  Обновляет canonical policy set.
- `EvaluatePolicy`
  Вычисляет enforcement decision для domain-relevant action.

### Queries

- `GetPolicy`
  Возвращает effective policy snapshot.
- `ExplainDecision`
  Возвращает explainable reason model для already computed decision.

## Выходные Контракты

- allow/deny decisions для `Identity`, `Workload`, `Hosted Services`, `Data`;
- explainable deny reasons для diagnostics и control surface;
- effective policy summary для node runtime.

## Authoritative Results

- `PolicySetResult`
- `PolicySnapshot`
- `PolicyDecision`
- `PolicyDecisionExplanation`

## Публикуемые События И State Outputs

### Domain Events

- `PolicyUpdated`
- `PolicyEvaluated`
- `PolicyDenied`
- `PolicyAllowed`

### State Outputs

- `PolicyDecision`
  Потребители: `Identity`, `Workload Control`, `Hosted Services`, `Data Substrate`.
- `PolicyDecisionExplanation`
  Потребители: `Diagnostics`, canonical local control surface,
  transport bindings.
- `PolicySnapshot`
  Потребители: `Node Runtime`, control surface.

## Фасад Домена

Фасад должен предоставлять:

- set/get effective policy;
- evaluate domain-relevant action;
- explain decision.

## Что Не Должно Протекать Через Фасад

- internal rule graphs, matcher state и evaluation pipeline steps;
- persisted policy source formats и adapter-specific policy rows;
- raw subject/context normalization intermediates;
- legacy allow/deny DTO и compatibility policy wrappers;
- внутренние caching, indexing и hot-reload mechanics.

## Привязка К Boundary Слоям

- canonical local control surface получает policy decisions и explainable deny
  reasons только через `Policy` facade.
- `Proto/Connect`, `HTTP`, `CLI` и другие transport bindings адаптируют
  policy-related requests к тем же policy-owned контрактам и не вызывают
  evaluation internals, cache layers или persistence adapters напрямую.
- transport bindings не читают rule graphs, source rows или legacy policy DTO
  напрямую.

## Целевая Структура Каталогов

```text
internal/policy/
  api/
  policyset/
  rule/
  decision/
  reason/
  evaluation/
  enforcement/
```

## Правила Внутренней Структуры

- `internal/policy/api` является обязательным каноническим публичным
  контрактом домена.
- `policyset`, `rule`, `decision` и `reason` хранят canonical policy state и
  explainability vocabulary.
- `evaluation` и `enforcement` содержат только policy-owned semantics и не
  создают обязательную параллельную форму `domain/application/infrastructure`.
- deny logic не должна снова размазываться по соседним пакетам.

## Правила Зависимостей

- `internal/policy` не зависит от `boundary/*`;
- соседние домены и transport bindings обращаются к policy только через
  `internal/policy/api`;
- policy result импортируется как decision, а не как доступ к внутренним rule
  structures.

## QA Этапы

Unit:

- evaluation allow/deny logic;
- reason model;
- effective policy assembly.

Integration:

- workload, hosting и data получают согласованные enforcement decisions;
- diagnostics и control surfaces видят одинаковые deny reasons.

E2E:

- оператор меняет policy и видит предсказуемое влияние на runtime behavior через
  canonical control surface.

## Правила Миграции

## Актуальное Состояние

`Policy` уже владеет:

- policy model;
- decision evaluation;
- reason/explainability ownership.

В домен не должны возвращаться:

- deny logic, размазанная по соседним доменам;
- старые config-shaped policy paths без authoritative model;
- compatibility routes, сохраняющие legacy decisions.

## Private Capability Admission

Policy is a mandatory behavior-changing boundary for every private capability
use. It admits or rejects a canonical subject, scope, permission, and use
operation after Identity has validated the signed grant and before selector or
envelope material is derived.

Policy owns operator-configured denial of private capability use and
scope-specific denial. It does not own capability secrets, grant signatures,
selector derivation, revocation persistence, or cryptographic delivery.
