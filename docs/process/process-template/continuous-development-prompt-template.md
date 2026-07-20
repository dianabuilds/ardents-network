# Continuous Development Prompt Template

Используй этот промпт как стартовую форму для нового непрерывного цикла
разработки. Перед запуском замени placeholders на конкретные документы и scope.

## Prompt

```text
Работаем в режиме непрерывного execution loop по документу [PATH_TO_EXECUTION_PLAN].

Обязательные правила:
- перед любым нетривиальным изменением сверяйся с:
  - docs/system-concept.md
  - docs/system-frame.md
  - docs/system-properties.md
  - docs/canonical-network-foundation.md
  - docs/engineering-constraints.md
  - docs/development-contract.md
  - [EXTRA_PROCESS_DOCS]
  - [RELEVANT_DOMAIN_DOCS]
  - docs/reference-invariants.md, если изменение затрагивает network/discovery/messaging/publication
  - docs/qa/test-model.md, если изменение затрагивает тесты, QA scenarios или test-layer split
- если кодовое изменение конфликтует с документами, сначала обновляй документы, потом код;
- не вводи fake foundation, prototype-first path, metadata-only substitute или deferred critical behavior;
- не останавливайся на отчете о прогрессе, пока в [PATH_TO_EXECUTION_PLAN] есть хотя бы одна задача со статусом `pending` или `in_progress`, кроме случая реального `blocked`;
- после завершения любой задачи немедленно:
  1. обновляй статус;
  2. проверяй gate текущей фазы;
  3. если gate не пройден, бери следующую задачу той же фазы;
  4. если gate пройден, переходи к следующей фазе;
- перед любым final response заново открывай [PATH_TO_EXECUTION_PLAN], проверяй активную фазу и, если в ней есть допустимая кодовая задача, продолжай выполнение вместо завершения ответа;
- если возникает blocker или compensating path, зафиксируй решение в [PATH_TO_DECISION_LOG] до продолжения работ;
- не открывай новую фазу или новый домен, пока текущий transition gate не пройден полностью;
- финальный ответ допустим только когда достигнут `done` по [PATH_TO_EXECUTION_PLAN] или когда зафиксирован реальный `blocked` с объяснением и decision entry.

Алгоритм работы на каждом цикле:
1. Открой [PATH_TO_EXECUTION_PLAN].
2. Найди первую задачу со статусом `in_progress`.
3. Если ее нет, возьми первую допустимую `pending` задачу.
4. Переведи задачу в `in_progress`.
5. Выполни задачу полностью.
6. Прогони обязательные проверки и тесты.
7. Если проверки не пройдены, продолжай исправления в рамках той же задачи.
8. Если проверки пройдены, переведи задачу в `done`.
9. Немедленно переходи к следующей допустимой задаче цикла.

Во всех ответах:
- не подменяй продолжение цикла summary-отчетом;
- коротко сообщай текущий активный шаг;
- если нужен decision, фиксируй его в документах, а не только в сообщении;
- сохраняй фокус на product-grade результате в пределах scope текущего цикла.
```

## Когда дополнять шаблон

Добавь в промпт дополнительные ограничения, если цикл:

- затрагивает domain remediation и требует blueprint docs;
- затрагивает release preparation и требует release review skills;
- затрагивает runtime security и требует security exception path;
- затрагивает тестовую модель и требует явной ссылки на scenario docs.
