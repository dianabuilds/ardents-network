# Exploratory Testing

## Назначение

Exploratory testing - это управляемый исследовательский процесс поиска слабых
мест системы там, где формальные сценарии, existing assertions и заранее
описанные checks могут еще не покрывать реальный риск.

Цель exploratory testing в Ardents:

- находить узкие места функционала;
- находить дефекты в interaction design и operator experience;
- находить слабые места в коде и error handling;
- находить архитектурные tension points, где system shape формально не нарушена,
  но уже ведет к drift, fragile behavior или непонятной эксплуатации.

Exploratory testing не заменяет canonical `unit` / `integration` / `e2e`
model. Он используется как guided search mechanism, который обязан завершаться
formal follow-up.

## Что считается объектом поиска

### Functional Weaknesses

- broken edge cases;
- неучтенные state transitions;
- скрытые coupling между доменами;
- случаи, где happy path работает, а degraded path не объясним;
- inconsistent behavior между local API, runtime truth и diagnostics.

### Design Weaknesses

- confusing operator flows;
- недообъясненные degraded states;
- misleading readiness or health signals;
- surfaces, где пользователь может принять ложное решение из-за ambiguous status;
- места, где процесс управления системой требует неочевидного знания.

### Code Weaknesses

- brittle branching;
- слабые assertions;
- отсутствующий error propagation;
- hidden assumptions about timing, ordering or defaults;
- logic, которая выглядит корректной, но плохо переживает variation.

### Architecture Weaknesses

- domain boundary erosion;
- duplicate ownership of decisions;
- hidden facade drift;
- runtime truth, зависящая от косвенных или несогласованных источников;
- design choices, которые еще не ломают систему напрямую, но системно
  увеличивают fragility или support cost.

## Исследовательский процесс

1. Выбрать domain, runtime flow или operator journey.
2. Сформулировать hypothesis о возможной слабости.
3. Выбрать probing technique:
   mutation, chaos, perturbation, manual journey probing, boundary sweep,
   diagnostics review, contract inconsistency review.
4. Зафиксировать expected signal:
   failure, degraded state, confusing UX, missing diagnostics, inconsistent API
   truth, unexpected recovery behavior.
5. Провести experiment в ограниченном и обратимом виде.
6. Зафиксировать observation.
7. Классифицировать finding.
8. Перевести finding в follow-up artifact.

## Методики exploratory probing

### Scenario Walkthrough Probing

Инженер проходит реальный operator or user flow и ищет:

- нестыковки между шагами;
- места, где результат нельзя понять без чтения кода;
- отсутствующую explainability;
- diverging behavior между intended и observed path.

### Boundary And Envelope Probing

Инженер проверяет границы значений, последовательностей и состояний:

- near-limit configuration;
- incomplete preconditions;
- reordered steps;
- repeated or interrupted commands;
- stale or partially available runtime conditions.

### Diagnostics-First Probing

Инженер исходит не из "что должно работать", а из "сможет ли оператор понять,
что произошло".

Проверяет:

- есть ли explainable degraded state;
- совпадает ли diagnostics truth с runtime truth;
- не скрыт ли critical failure за generic status;
- достаточно ли evidence для remediation.

### Architecture Tension Review

Инженер сознательно ищет не единичный bug, а structural weakness:

- непонятное ownership boundary;
- повторяющуюся mapping logic;
- surfaces, которые маскируют domain drift;
- decisions, которые ведут к накоплению скрытого operational cost.

Такой finding тоже является valid exploratory result, если он конкретен и
переводится в actionable follow-up.

## Finding Classification

Каждая находка должна быть отнесена хотя бы к одной категории:

- functional bug;
- design/usability weakness;
- diagnostics/explainability gap;
- code robustness gap;
- architecture drift risk;
- test coverage gap;
- blocked concern requiring decision.

## Обязательный follow-up

Exploratory testing считается завершенным только если finding переведен в одно
из формальных действий:

- новый или обновленный scenario doc;
- новый или усиленный `unit`, `integration` или `e2e` test;
- новый active-testing experiment doc, если finding требует отдельной методики;
- remediation task в active process document;
- decision-log entry, если риск пока принимается или блокирует дальнейшее движение.

## Недопустимые практики

- считать свободное ручное исследование достаточным без фиксации finding;
- оставлять finding в форме "что-то тут странно";
- использовать exploratory testing как замену acceptance;
- подменять исследование генерацией большого числа ad-hoc действий без hypothesis;
- делать архитектурные выводы без привязки к конкретному runtime или operator impact.

## Связанные документы

- [Active Testing](./README.md)
- [Mutation Testing](./mutation-testing.md)
- [Chaos Testing](./chaos-testing.md)
- [Fault Injection And Parameter Perturbation](./fault-injection-and-parameter-perturbation.md)
- [Test Model](../test-model.md)
