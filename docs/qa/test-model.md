# Test Model

## Назначение

Этот документ задаёт единую тестовую модель проекта.

Он определяет:

- уровни тестирования;
- типы сценариев;
- обязательную структуру scenario docs;
- обязательную структуру integration/e2e tests;
- правила валидации против false positive и false negative behavior.

## Уровни тестирования

### `unit`

Проверяет локальную логику, небольшие инварианты и контракты функций/типов без
полноценного runtime flow.

Форма документации:

- допускается inventory-style запись одной строкой на группу unit tests;
- подробный сценарный документ не обязателен.

### `integration`

Проверяет взаимодействие нескольких компонентов, пакетов или runtime slices
внутри одного потока поведения.

Форма документации:

- каждый integration test path обязан иметь собственный scenario document;
- scenario document хранится в `docs/qa/integration/`.

### `e2e`

Проверяет полный пользовательский, операторский или системный поток от
preconditions до observable outcome через реальные runtime boundaries.

Форма документации:

- каждый e2e scenario обязан иметь собственный document;
- scenario document хранится в `docs/qa/e2e/`.

## Правило работы с уже написанными тестами

Если в домене уже существуют тесты до начала remediation stage, они обязаны
быть сначала разобраны по слоям, а не оставлены как стихийный смешанный набор.

Обязательный порядок:

1. определить, какие из существующих тестов являются `unit`;
2. определить, какие из существующих тестов фактически являются `integration`;
3. определить, какие из существующих тестов фактически являются `e2e`;
4. обновить `docs/qa/unit-tests.md` для unit coverage;
5. создать scenario docs для всех существующих integration/e2e tests;
6. проверить, что integration/e2e tests трассируются к своим scenario docs;
7. вынести integration/e2e tests из domain-local mixed form, если они мешают
   явному layer split.

## Правило размещения тестов по слоям

После layer split действует следующее правило размещения:

- `unit` tests остаются рядом с кодом домена по месту;
- `integration` tests оформляются как отдельный слой и обязаны иметь scenario
  docs в `docs/qa/integration/`;
- `e2e` tests оформляются как отдельный слой и обязаны иметь scenario docs в
  `docs/qa/e2e/`.
- repository-level `integration` paths должны быть opt-in и запускаться через
  build tag `integration`;
- repository-level `e2e` paths должны быть opt-in и запускаться через build tag
  `e2e`;
- общий test harness для cross-package scenarios должен размещаться в
  `tests/testkit/` как единая точка для wait helpers, runtime setup,
  auth/reporting helpers и будущего runner surface.
- repository-level suites должны запускаться через единый runner path внутри
  Linux-контейнера; Windows остаётся только host orchestration surface и не
  является test runtime;
- IDE запускает тот же container runner. Прямой Windows `go test` допустим лишь
  как неканоническая локальная диагностика и не может давать acceptance evidence.

Если старые integration/e2e tests физически остаются в доменных пакетах на
переходном этапе, это должно быть:

- явно зафиксировано как transitional state;
- объяснено в process docs или decision log;
- устранено до acceptance текущего домена, если это мешает явному layer split.

## Formal Metadata Contract

Для repository-level `integration` и `e2e` tests metadata должна жить рядом с
тестом в коде через `tests/testkit`-owned API (`testkit.Spec` или эквивалентную
явную декларацию). Внешний catalog, manifest или generated projection допустим
только как производный артефакт и не может становиться вторым source of truth.

Обязательные metadata fields для каждого repository-level `integration`/`e2e`
test case:

- `Layer`
- `Domain`
- `Scenario ID`
- `Suite`
- `Tags`
- `Speed`
- `Environment`

Допустимые расширения metadata:

- ownership hints;
- capability markers;
- runner-selection hints, если они не дублируют и не подменяют обязательные поля.

Требования к metadata model:

- metadata должна быть доступна runner/reporting path без парсинга имени теста;
- `Scenario ID` должен совпадать со stable identifier из `docs/qa/integration/*`
  или `docs/qa/e2e/*`;
- `Layer` и suite markers не могут противоречить build tags;
- тест без formal metadata не считается завершённым repository-level
  `integration`/`e2e` артефактом.

## Canonical Runner Contract

Каноническим execution engine остаётся `go test`. Repository-level orchestration
строится поверх него и не заменяет его отдельным test DSL или сторонним runtime.

Канонический runner path проекта:

- `tests/run.ps1` для suite-oriented прогонов репозитория;
- `fast` для default non-tagged path;
- `integration` для tagged integration suite;
- `e2e` для tagged e2e suite;
- `all` для полного tagged repository run.

Обязательные свойства runner contract:

- build tags `integration` и `e2e` остаются boundary между suite layers;
- suite execution происходит в воспроизводимом Linux/Docker runtime с
  отдельными persistent Go module/build cache volumes;
- host и IDE entry points обязаны делегировать в тот же container runner и не
  создавать параллельную Windows test semantics;
- future selection по suite/domain/tag/scenario расширяет этот runner path, а не
  создаёт competing execution surface.

## Reporting Contract

QA framework обязан выдавать одновременно:

- человекочитаемый console summary для локального запуска;
- machine-readable canonical report для CI и автоматической валидации.

Обязательные report fields:

- run identifier или artifact identity;
- selected suite/profile;
- selected tags, domains и scenario filters, если они использовались;
- test package;
- test name или case id;
- `Layer`;
- `Domain`;
- `Scenario ID`;
- итоговый status;
- duration;
- failed step или assertion stage, если это известно;
- diagnostics artifact references или их отсутствие.

Report contract считается соблюдённым только если:

- report можно использовать локально и в CI без ручного разбора console output;
- failures локализуются минимум до уровня test case;
- scenario-aware summary позволяет выявлять coverage drift и broken bindings.

## Contract For New Integration And E2E Tests

Новые repository-level `integration` и `e2e` tests обязаны одновременно:

- объявлять formal metadata рядом с тестом;
- ссылаться на scenario document по `Scenario ID`;
- явно разделять preconditions, scenario steps, expected assertions и
  failure/degraded assertions;
- использовать `tests/testkit/` как единую cross-package harness surface для
  общего bootstrap, auth, reporting и wait helpers;
- оставлять observable путь для diagnostics/report capture, а не скрывать
  failure path во вспомогательной магии.

## Migration Contract For Existing Repository-Level Tests

Существующие repository-level `integration` и `e2e` tests должны мигрироваться в
formal framework shape по следующим правилам:

1. Сначала тест получает явный `Scenario ID` и scenario binding.
2. Затем тест получает formal metadata рядом с кодом.
3. Затем тест переводится на явную step-oriented structure.
4. Затем test package подключается к canonical runner/reporting path без ad-hoc
   локальных исключений.

Допустимое transitional состояние ограничено:

- mixed-form package допускается только как узкий переходный этап;
- исключение должно быть объяснено в process docs или decision log;
- переходное состояние не может становиться постоянной нормой для новых tests.

## Acceptance Contract For Suite Runs And Coverage Validation

Framework acceptance по repository-level test platform требует, чтобы:

- suite runs шли через единый runner path;
- reports содержали formal metadata и scenario-aware results;
- можно было проверить, какие `Scenario ID` покрыты тестами, а какие нет;
- можно было выявить tests без scenario binding и scenarios без test binding;
- suite/report output был пригоден для локальной диагностики и CI automation.

## Типы сценариев

Тестовая модель обязана покрывать обе группы:

### Functional scenarios

- успешные command/query flows;
- state transitions;
- publication/unpublication flows;
- recovery flows;
- diagnostics visibility flows;
- authorization and policy flows.

### Non-functional scenarios

- recovery after interruption;
- degraded behavior;
- mutation-resistance behavior;
- chaos-resilience behavior;
- timeout and retry behavior;
- startup/shutdown stability;
- observability and diagnostics completeness;
- environment-specific behavior;
- container-based runtime behavior;
- performance/resource-safety scenarios там, где они критичны для домена.

## Mutation And Chaos Testing

Mutation testing and chaos testing are mandatory exploratory QA methods for
finding weak points that ordinary happy-path coverage can miss. They do not
replace the canonical `unit` / `integration` / `e2e` split. They operate as
pressure techniques on top of that model and must end in stronger canonical
coverage, not in ad-hoc one-off experiments.

Detailed method descriptions live in `docs/qa/active-testing/`.

### Mutation Testing

Mutation testing deliberately changes code paths, conditions, return values,
guards, or assertions to prove that the existing test model detects broken
product behavior.

Required expectations:

- the mutation must target a concrete product invariant, scenario step, or
  failure/degraded path;
- the expected detection signal must be known before the experiment starts:
  failing automated tests, failed assertions, degraded diagnostics, policy
  rejection, or another explicit observable;
- surviving mutations are treated as coverage gaps and must produce follow-up
  work: a new regression test, a stronger assertion, an updated scenario doc,
  or a documented blocked decision;
- mutated code must never remain in the product path after the experiment.

### Chaos Testing

Chaos testing deliberately changes runtime parameters, timing, topology,
resource limits, connectivity, process lifecycle, or dependency behavior to
prove that the system remains explainable under stress, interruption, and
partial failure.

Required expectations:

- the fault injection must be bounded by explicit scope, blast radius, and
  rollback path;
- the experiment must validate operator-visible truth: health, degraded state,
  recovery outcome, pending operations, and diagnostics explanation;
- chaos experiments must target real runtime behavior and must not rely on fake
  foundation or purely synthetic success criteria;
- any weakness found in chaos runs must be converted into a canonical
  non-functional scenario, regression test, or documented residual risk.

## Exploratory Experiment Contract

Mutation and chaos work may be free-form in how the engineer searches for
failures, but it is not undocumented improvisation. Every meaningful
experiment must record:

- target domain or scenario;
- hypothesis about the weak point being probed;
- exact mutation or injected fault;
- expected observable signal;
- rollback or cleanup path;
- result: detected, survived, inconclusive, or blocked;
- required follow-up artifact.

Allowed follow-up artifacts:

- update to a scenario document;
- new or strengthened `unit`, `integration`, or `e2e` test;
- runner/reporting improvement when detection existed but was not explainable;
- decision-log entry when the weakness cannot be closed immediately.

Mutation and chaos experiments must not create a second test model, a second
runner, or a metadata-only substitute for real regression coverage.

Related methodology docs:

- `docs/qa/active-testing/exploratory-testing.md`
- `docs/qa/active-testing/README.md`
- `docs/qa/active-testing/mutation-testing.md`
- `docs/qa/active-testing/chaos-testing.md`
- `docs/qa/active-testing/fault-injection-and-parameter-perturbation.md`

## Обязательная структура scenario document

Каждый integration/e2e scenario document обязан содержать:

- `Scenario ID`
- `Layer`
- `Domain`
- `Category`
- `Goal`
- `Preconditions`
- `Steps`
- `Expected Result`
- `Failure/Degraded Variant`
- `Related Tests`
- `False Positive Risk`
- `False Negative Risk`
- `Notes`

## Обязательная структура integration/e2e tests

Код integration/e2e tests обязан явно разделять:

```text
test() {
  precondition(...)
  step(1, ...)
  step(2, ...)
  assertExpected(...)
}
```

Это не обязательно буквальный API, но в тесте должны быть ясно видны:

- подготовка preconditions;
- отдельные шаги сценария;
- отдельные assertions по expected outcome;
- отдельные assertions по failure/degraded outcome, если они входят в сценарий.

## Правило трассировки

Host-orchestrated Docker scenarios that cannot safely execute from inside the
test container may use a canonical `tests/ci/*.ps1` gate. Such a gate must have
`# testkit:scenario`, `# testkit:layer`, and `# testkit:domain` metadata, must be
unconditionally runnable on its supported host, and must retain evidence. The
catalog treats that script as the runnable code binding; workflow YAML remains
only a scheduler. A skipped or metadata-only script is not a valid binding.

Для каждого integration/e2e test должно быть возможно однозначно ответить:

- по какому сценарию он написан;
- какой шаг сценария он проверяет;
- какой expected outcome он подтверждает;
- какой degraded/failure outcome он подтверждает или исключает.

Если этой трассировки нет, тест считается документированным недостаточно.

## Валидация против false positive и false negative

Каждый integration/e2e scenario обязан быть провалидирован на два риска:

### False Positive Risk

Тест проходит, хотя реальный сценарий сломан.

Обязательная проверка:

- assertions проверяют не только отсутствие ошибки, но и реальный observable
  result;
- preconditions действительно создают нужное состояние;
- test не проходит за счёт default state, mock drift или непроверенной ветки.

### False Negative Risk

Тест падает, хотя сценарий работает корректно.

Обязательная проверка:

- test не зависит от нестабильного времени, порядка событий или случайной
  среды без явного контроля;
- assertions не проверяют лишние побочные детали, не являющиеся частью
  сценария;
- test environment и scenario document согласованы между собой.

## Acceptance для тестовой модели

Тестовая модель считается соблюдённой, только если:

- unit/integration/e2e явно разделены;
- у всех integration/e2e tests есть собственные scenario docs;
- functional и non-functional scenarios зафиксированы;
- mutation/chaos weakness-discovery path формализован и привязан к canonical
  regression follow-up;
- tests трассируются к сценариям;
- false positive и false negative risks описаны и проверены.
