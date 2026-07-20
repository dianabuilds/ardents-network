# Loop Bootstrap Checklist

Используй этот checklist перед стартом нового process loop.

## 1. Scope

- [ ] Определен точный scope цикла.
- [ ] Явно указан owning domain или owning process area.


## 2. Source Of Truth

- [ ] Перечислены обязательные system docs.
- [ ] Перечислены релевантные domain docs.
- [ ] Добавлен `docs/reference-invariants.md`, если цикл затрагивает
      network/discovery/messaging/publication.
- [ ] Добавлен `docs/qa/test-model.md`, если цикл затрагивает test model или
      QA scenarios.

## 3. Process Shape

- [ ] Есть statuses: `pending`, `in_progress`, `done`, при необходимости
      `blocked`.
- [ ] Есть execution algorithm.
- [ ] Есть phase structure.
- [ ] Есть transition gates.
- [ ] Есть final acceptance gate.

## 4. Decision Path

- [ ] Понятно, нужен ли отдельный decision log.
- [ ] Определено, какие ситуации обязаны попадать в decision log.
- [ ] Определено, как цикл продолжает работу после blocker decision.

## 5. Verification

- [ ] Определены code checks.
- [ ] Определены runtime checks.
- [ ] Определены test expectations.
- [ ] Определено, какие diagnostics или operator-visible outcomes обязательны.

## 6. Operational Use

- [ ] Подготовлен стартовый prompt для агента.
- [ ] В prompt и/или execution plan явно запрещен final response до прохождения gate активной фазы или фиксации реального `blocked`.
- [ ] Новый цикл добавлен в используемый process index или README, если такой индекс ведется в репозитории.
- [ ] Понятно, какой документ является главным управляющим документом цикла.
