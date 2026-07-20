# Feature Research Prompt Template

Use this prompt when an agent must conduct a feature research loop and end with
an explicit decision.

## Prompt

```text
Работаем в режиме feature research loop по документу [PATH_TO_RESEARCH_EXECUTION_PLAN].

Цель цикла:
- проверить гипотезу о фиче;
- понять, стоит ли её вообще реализовывать;
- если стоит, определить допустимый product-grade путь;
- если не стоит, явно зафиксировать отказ, перенос или reshape.

Обязательные правила:
- перед любым нетривиальным шагом сверяйся с:
  - docs/system-concept.md
  - docs/system-frame.md
  - docs/system-properties.md
  - docs/canonical-network-foundation.md
  - docs/engineering-constraints.md
  - docs/development-contract.md
  - [RELEVANT_DOMAIN_DOCS]
  - docs/reference-invariants.md, если идея затрагивает
    network/discovery/messaging/publication foundation
  - docs/qa/test-model.md, если идея меняет QA или test-layer shape
- не подменяй исследование скрытым implementation path;
- не принимай идею без явной evidence base;
- не скрывай disconfirming evidence;
- если идея конфликтует с системными документами, зафиксируй это как finding,
  а не обходи молча;
- не считай цикл завершённым без явного outcome:
  `accepted`, `rejected`, `deferred`, `reshaped` или `blocked`;
- если outcome = `accepted`, обязательно зафиксируй:
  1. какая документация должна быть создана или обновлена;
  2. какой execution path допустим для реализации;
  3. какие риски и проверки обязательны до кода;
- если возникает blocker или развилка решения, фиксируй её в
  [PATH_TO_DECISION_RECORD_OR_LOG], а не только в сообщении.

Алгоритм работы:
1. Открой [PATH_TO_RESEARCH_EXECUTION_PLAN].
2. Возьми первую задачу со статусом `in_progress`, либо первую допустимую `pending`.
3. Переведи задачу в `in_progress`.
4. Выполни её полностью и зафиксируй evidence в [PATH_TO_EVIDENCE_LOG].
5. Если evidence недостаточно, продолжай исследование в рамках той же задачи.
6. Если evidence достаточно, переведи задачу в `done`.
7. Немедленно переходи к следующей задаче цикла.
8. В конце оформи [PATH_TO_FINAL_DECISION].

Во всех ответах:
- коротко сообщай текущий активный шаг;
- не подменяй цикл summary-отчётом;
- сохраняй фокус на проверке гипотезы, а не на желании "протолкнуть" идею;
- если идея не выдерживает проверку, прямо говори об этом.
```

## Expected Outputs

By the end of the loop the research package must contain:

- filled research brief;
- filled evidence log;
- filled final decision record;
- required implementation docs and allowed next loop, if accepted.
