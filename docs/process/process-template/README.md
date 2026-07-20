# Process Template Kit

## Назначение

Каталог `docs/process/process-template/` содержит копируемые шаблоны для новых
циклов непрерывной разработки в стиле:

- delivery-loop;
- remediation-loop;
- release hardening loop;
- domain-focused execution loop;
- другой task-driven process loop, если он должен жить внутри `docs/process/`.

Шаблоны не заменяют system docs и не создают новый source of truth.
Они задают только процессовую форму, совместимую с уже принятыми правилами
Ardents.

## Что копировать

- `execution-plan-template.md`
  Пустой управляющий план цикла с фазами, задачами, gates и execution loop.
- `decision-log-template.md`
  Пустой лог решений для blocker scenarios, compensating paths и смены
  приоритетов.
- `continuous-development-prompt-template.md`
  Стартовый промпт для агента, который должен вести цикл без выхода в
  промежуточный отчет.
- `loop-checklist-template.md`
  Короткий operational checklist для старта нового process loop.

## Рекомендуемый порядок запуска нового цикла

1. Скопировать `execution-plan-template.md` в новый process document.
2. Скопировать `decision-log-template.md` в соседний decision log, если цикл
   требует явной фиксации process decisions.
3. Заполнить scope, source-of-truth documents, phase list и acceptance gate.
4. При необходимости адаптировать `continuous-development-prompt-template.md`
   под конкретный loop.
5. Добавить ссылки на новые process docs в тот индекс или README, который реально используется в репозитории.

## Обязательные свойства шаблона

Любой новый цикл, созданный из этих шаблонов, обязан сохранять:

- непрерывное движение до `done` или явно описанного `blocked`;
- обязательную сверку с source-of-truth docs перед нетривиальными изменениями;
- запрет на prototype-first delivery и fake foundations;
- явные statuses, checks, transition gates и final acceptance;
- обязанность после каждой завершенной задачи немедленно переходить к
  следующей допустимой задаче цикла;
- запрет на final response, пока активная фаза не прошла gate и в цикле
  остается хотя бы одна допустимая кодовая задача, кроме случая реального
  `blocked`.
