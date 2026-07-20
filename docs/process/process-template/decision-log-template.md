# [Loop Name] Decision Log

## Назначение

Этот документ фиксирует решения, принятые в ходе `[loop name]`, когда:

- обнаружен blocker или blocker-like scenario;
- требуется compensating path без выхода из execution loop;
- меняется порядок задач, фаз или приоритетов;
- нужно явно зафиксировать отклоненный вариант решения;
- нужно сохранить причину process, architecture или delivery decision.

Этот лог не заменяет audit docs, remediation docs или основной execution plan.
Он хранит именно решения.

## Правило записи

Решение должно быть записано в этот лог до продолжения следующего значимого
этапа, если оно:

- влияет на порядок работ;
- влияет на acceptance interpretation;
- меняет compensating path;
- оставляет тонкий transitional слой на ограниченный период;
- меняет active domain, active phase или следующий безопасный ход.

## Формат записи

Каждая запись должна содержать:

- `Decision ID`
- `Date`
- `Domain / Scope`
- `Stage`
- `Situation`
- `Options Considered`
- `Decision`
- `Reason`
- `Impact`
- `Follow-up`
- `Status`

Допустимые значения `Status`:

- `active`
- `superseded`
- `closed`

## Записи

### DEC-001

- `Date`: YYYY-MM-DD
- `Domain / Scope`: `[domain or loop scope]`
- `Stage`: `[phase / stage name]`
- `Situation`: [какая развилка или blocker возникли]
- `Options Considered`:
  `[option A]`;
  `[option B]`;
  `[option C, если был]`
- `Decision`: [что именно выбрано]
- `Reason`: [почему выбран именно этот путь]
- `Impact`: [как это меняет ход цикла, структуру, acceptance или follow-up]
- `Follow-up`: [какое следующее действие обязательно после решения]
- `Status`: `active`
