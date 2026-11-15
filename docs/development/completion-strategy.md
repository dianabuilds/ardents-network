# Стратегия завершения закрытого продукта и удаления legacy

Дата анализа: **2026-11-15**. Это предложение порядка работы и критериев
готовности по запросу Product Owner, а не новая спецификация или второй backlog.
Статус, исполнители и выбор работ остаются в GitHub C0. Этот запрос разрешает
только документацию, скиллы и системные инструкции; код, зависимости и тестовые
раннеры в рамках подготовки стратегии не меняются.

## Результат, к которому идём

Работающий результат — выбранный [закрытый Ubuntu text-Service](../product/scope.md):
проверенные артефакты, независимое первоначальное доверие, поддерживаемая
подготовка и установка ролей, публикация Service Instance, Target Link, чтение
вторым установленным Endpoint, понятные отказы, Stop/restart и разрешённое
восстановление без потери расходов и обязательных floors. У каждого сценария
один действующий набор доменных владельцев и проверяемый операторский путь.

Ближайший продукт использует TCP/TLS по ADR-0122. QUIC исследуется отдельно и
возвращается только после доказанного решения его блокера и принятия границы
Carrier. Полный journey требует предусмотренной контрактом структуры Role
Domains и различных effective Operator Families. Дополнительные машины одного
владельца её не создают: при отсутствии prerequisites результат остаётся
компонентным/диагностическим, а полная приёмка — незавершённой.

«Удалить legacy» означает снять заменённые реализации и их последние реальные
потребители. Remote `old`, история Git, нужные свидетельства и пользовательские
данные сохраняются. ADR-0124 уже разрешает независимые новые установки без
переноса predecessor keys/history/rights; история и recovery самой новой
установки остаются обязательными. Переименование `successor` и удаление бинарника
не доказывают ни архитектурную, ни продуктовую готовность.

## Основания и границы обзора

Проверены текущие owners, исходники, Makefile/CI/профили, локальный DDD-скилл и
GitHub ledger. Снимок: HEAD `6b8e9495309daae7d8314f4797a859f7a9d3012e`, ветка
`codex/559-introduction-installed`, с уже существовавшими изменениями Introduction
и старого Route. Они сохранены. Исходники показывают рабочее дерево, а не
автоматически принятый результат. Полные тесты и native-эксперименты в этом
обзоре не выполнялись; чужие acceptance receipts прочитаны, не воспроизведены.

| Наблюдение | Проверяемое основание | Следствие |
|---|---|---|
| Новая доменная модель уже содержательная | `internal/successor/admission/stock/stock.go`, `endpoint/participant_linux.go`, `node/doc.go`; [domain map](domain-map.md) | Проверять и завершать существующие границы, без новой тотальной DDD-перестройки |
| Старый Endpoint ещё имеет действующего command consumer | `cmd/ardents/endpoint_text_linux.go` вызывает старый Endpoint; `cmd/ardents-next/installed_endpoint_linux.go` — новый | Новый пакет сам по себе не разрешает удалить старый; нужен выбор судьбы каждого сценария |
| Остались старые tool/Node/Control consumers | `cmd/ardents-qualification/{main.go,plan_linux.go}`, `cmd/ardents-node/node_mode.go`, `cmd/ardents-control/closed_issuer_inspection.go` | Удаление проверяется по полной closure, включая процессы, recovery и сборку |
| Есть точные копии физических механизмов | `installation/{forwarding,join,descriptor,introduction}/{account_linux.go,principal_linux.go,initial_readback_linux.go}` одинаковы кроме package в просмотренном снимке | Подтверждён риск расхождения исправлений; ошибки полномочий из сходства не следуют |
| Есть расхождения документации | Installation README описывает Introduction как initial-only, тогда как новый start уже присутствует; `testing.md` содержит 5670 строк и поздний вход в ordinary checks | Исправлять факты у владельца и сокращать маршрут чтения, не выдавая dirty работу за acceptance |
| Проверки уже разделены по средам и затронутым потребителям | `scripts/select-pr-checks.go`, `tests/profiles/profiles.json`, `ownership.json`, `.github/workflows/quality.yml` | Развивать существующую систему и измерить стоимость до изменения gates |

Перед исполнением заново проверить выбранную issue, source и owners: этот снимок
не сохраняет активность задач. [#497](https://github.com/dianabuilds/ardents-network/issues/497)
указывает Introduction, затем Issuer и переход consumers. При этом его общая
формулировка удаления `ardents-next` противоречит прямому уточнению Product Owner
в [#561](https://github.com/dianabuilds/ardents-network/issues/561),
[#562](https://github.com/dianabuilds/ardents-network/issues/562) и
[#565](https://github.com/dianabuilds/ardents-network/issues/565): работающие команды
сохраняются, автоматический перенос/удаление снят. Согласовать parent summary с
этим уточнением до зависимого cutover. Здесь GitHub не изменён и новая реализация
не выбрана. Открытая issue также не доказывает отсутствие уже готового компонента.

## Порядок работы и выход каждого этапа

Конкретные границы задач, зависимости, проверки и критерии приёмки собраны в
[плане завершения successor](successor-completion-plan.md). Он уточняет исходный
обзор по текущим #497/#559/#561 и сохраняет existing ledger: завершение Introduction
у текущего исполнителя → установленный Issuer → полный Endpoint/инструментальный
путь → удаление групп по последнему consumer → отдельно выбранные конечные пути
→ итоговый candidate/D1–D9. Нужные producer/tool subsets поставляются до зависимого
installed positive; уже интегрированное выделение Endpoint не повторяется.

Работа выполняется малыми вертикальными срезами. Документация и проверки
обновляются с каждым владельцем; конкретная группа legacy не ждёт закрытия всех
issues, если её собственные условия удаления уже доказаны.
Планирование и read-only review могут идти параллельно; зависимая реализация
остаётся у своего исполнителя. Сохраняется WIP: не более двух явно выбранных
implementation issues и одного research question, с учётом других активных
чатов проекта. Эта стратегия не занимает дополнительный implementation slot.

Не назначать календарную дату готовности до baseline стоимости проверок и
согласования оставшихся acceptance gaps. Первый плановый горизонт — завершение
одной выбранной role boundary и один измеримый результат улучшения feedback.
После него пересчитать критический путь по фактической работе и prerequisites.

## 1. Готовность DDD и архитектуры

Network, Admission, Route, Publication и Connection содержат существенные
инварианты. Installation, Endpoint/Node composition и native adapters требуют
ясных владельцев lifecycle и узких механизмов. Им не нужны обязательные
repository/event-bus/универсальный AggregateRoot или новые микросервисы.

Для каждой выбранной операции использовать существующий
[interface checklist](domain-map.md#interface-checklist-for-development-plans):
правило → semantic/authority/state/resource/terminal owner → настоящий consumer
→ effect/commit/handoff → positive и causal refusal → profile/evidence.
Проверять callbacks и process bridges, а не только импорты. Сохранить R01–R08,
fresh observations, необратимый spend, независимые roots и
deny → interrupt → physical join → release. Никаких чужих I/O под state lock.

Приоритеты обзора:

1. Принимающие операции без полноценного caller/owner/termination proof.
2. Повторённые domain decisions или общий mutable owner между разными правами.
3. Подтверждённые копии native-механизмов Installation: account/principal,
   original readback, затем по evidence startup/retirement/recovery.
4. Командная композиция, удерживающая долгоживущее состояние вне его владельца.
5. Имена и расположение файлов — после поведения и потребителей.

Для копий сравнить не только текст, но admission, ошибки, retention и completion.
Извлечение оправдано, если одинаков механизм, а разные политики могут остаться у
ролей без callback-конструктора полномочий. Сначала рассмотреть уже существующий
cohesive package; новый пакет проходит обычную регистрацию. Если отдельность
намеренная, оставить явную причину и проверить охват конкретных копий drift gate;
при необходимости зарегистрировать их в выбранном изменении. Существующий
gate сам по себе не означает контроль перечисленных четырёх ролей. Дедупликация не
должна объединять учётные записи как authority, live leases, signing purposes,
журналы или Startup разных ролей. Не редактировать текущий незавершённый
Introduction одновременно с его исполнителем.

Готовность означает отсутствие незакрытых consequential findings в выбранной
границе и наличие проверок её поведения. Она не означает доказанное отсутствие
всех архитектурных проблем во всём проекте. Счётчик пакетов/строк и DDD-score
не являются критериями приёмки.

## 2. QUIC: решение с ограниченным исследовательским циклом

Обновлённое исследование и первичные источники находятся в
[R-172](../research/records/r-172-quic-return-path-budget.md#read-only-upstream-screen-2026-11-15).
На дату просмотра latest — quic-go v0.65.0, минимальный Go 1.26.0; проект уже
фиксирует Go 1.27.4. В текущем go.mod quic-go отсутствует: это будущий выбор
зависимости, а не обычный patch-update имеющегося Carrier.

Исходный finding касается cumulative return-path reply budget для ещё не
подтверждённого адреса. Исторические 32 входящих / 1200 исходящих байт получены
в user-space fixture; это не установленный reflection exploit Ardents и не
новое Linux OOB измерение. Source review v0.65.0 не установил поддерживаемое
исправление именно этого механизма. Поэтому простой upgrade сейчас не имеет
достаточного основания для возвращения QUIC; принципиальная невозможность
использовать протокол также не доказана.

Порядок: один короткий поиск конкретного maintained patch/enforced configuration
→ причинный контроль реальной зависимости → caller/config/backend closure
→ native owner/Service/lifetime проверки → решение об admission. Дорогой стенд
не запускать до появления обоснованного кандидата. Составная методика уже
допускает причинный in-memory proof; обязательный новый active native attack
experiment не вводится.

| Вариант | Решение |
|---|---|
| Поддерживаемый upstream fix | Предпочтительный путь; реальный per-address send admission и red/green regression обязательны |
| Документированная enforced configuration | Допустима лишь с доказанной полнотой; Retry, MTU flags, отключённые datagrams или backoff сами по себе не решают finding |
| Другая библиотека | Только отдельная оценка maintenance, security, Go/runtime fit, lifecycle и стоимости миграции после отрицательного candidate gate; присутствие модуля в cache не является admission |
| Private fork/собственный transport | Не предлагается при текущей мощности команды; потребует отдельного решения о постоянном сопровождении, а не временной заплатки в module cache |
| Оставить QUIC отложенным | Корректный результат текущего цикла; TCP-релиз продолжается |

Три выхода: **go для кандидата**, **no-go для проверенного кандидата**,
**inconclusive с точным пробелом**. Последний не превращается в бесконечные
повторы baseline. Повод вернуться — новый patch/release, меняющий найденную
границу, либо новая проверяемая enforcing option. Ни один исход source review
не снимает ADR-0122 и не разрешает fallback.

## 3. Проверки: качество и стоимость feedback

Использовать существующий PR-selector, ownership/profile registries и causal
tests. Сначала измерить, затем менять политику. `quick-check` сейчас запускает
deterministic inventory; `check` повторяет её и добавляет process/race; отдельные
native/Route profiles проверяют другие условия. Структурное повторение не
доказывает ненужности проверки.

На одном точном source и объявленной среде собрать несколько cold/warm запусков:
build/setup/test/teardown, wall time критического пути, CPU, повторную компиляцию,
failure/invalid-environment causes и стоимость диагностики. Сохранять исходные
ошибки. Сначала использовать уже доступные receipts, различая просмотренное и
воспроизведённое; дополнительные прогоны обосновывать недостающим измерением.
Бюджет developer feedback выбрать после baseline; пока нет оснований обещать
сокращение в конкретное число раз.

| Маршрут | Когда и что запускается | Что он доказывает |
|---|---|---|
| Локальный цикл владельца | Узкие package/causal tests, нужный race; штатный quick-check во время работы | Быстрый ответ по изменяемому правилу, без installed claim |
| Проверка дельты | Существующий affected selector, consumers, architecture/ownership, build-tag compilation; выбранный fuzz при изменении грамматики | Проверяемая область влияния PR; не полная приёмка |
| Интеграция | Обязательный `make check` на конечном candidate плюс применимые native profiles | Полный обязательный gate этой интеграции |
| Продукт | Реальные Ubuntu installations, два Endpoints и подлинные владельцы, полный выбранный workload/fault matrix | Только заявленная квалификация точного candidate/environment |

Это маршруты использования имеющихся проверок, а не введённые новые команды.
Quick/full gates сейчас не сокращены. Изменение их состава потребует отдельного
обоснования у testing owner; до него действуют нынешние требования.

Первые конкретные доработки:

- **Root ownership coverage.** `route/selection/root_hardening_linux_test.go`
  пропускает foreign-owner test без root, а обычный Linux gate требует non-root.
  Зарегистрировать реальное выполнение в подходящем root-native профиле и
  проверить discovery. Отсутствие prerequisite — invalid environment, не PASS/skip.
- **Mutation fuzz.** Общий `scripts/run-fuzz-targets.go` выбирает старый Epoch;
  два новых issuer-profile targets запускаются отдельно, другие successor fuzz
  functions не означают автоматическую mutation-кампанию. Свести выбранные
  grammar/property/owner/budget в существующий registry и перенести Epoch oracle
  при переключении owner. Различать seed execution и mutation fuzz.
- **Стоимость.** Проверить повторные builds и дорогие fixtures для pure rules,
  сохраняя самостоятельность oracle. Не убирать сериализацию измерений общих
  loopback/cgroup ресурсов. Различать виртуальный `synctest` sleep и реальное
  ожидание; оптимизировать измеренную причину.
- **Дубликаты тестов.** Удалять только после сравнения requirement, seam, oracle,
  fault/transition, platform/format и независимой доказательной роли. Совпадение
  названия сценария недостаточно. Новый тест должен различать нарушение, а не
  повторять структуру реализации.

Матрица обязательных сценариев связывает: установка и substituted artifact;
fresh/current/conflicting State; выдача/двойной расход/exact retry;
publish/read/withdraw; wrong Target/Instance/authority; cancellation/expiry/late
handoff; concurrency и удержанный Close; partial writes/reopen/restart;
forward replacement и допустимый recovery; confinement и descendants;
overload/finite budgets; QUIC pre-effect refusal. Для каждого выбрать самый
дешёвый достаточный слой, сохраняя end-to-end proof взаимодействия владельцев.
Полные численные bounds остаются в текущих owners, здесь не дублируются.

Среда данного обзора: локальный Go 1.27.1 darwin/arm64 отличается от pinned
1.27.4; staticcheck/govulncheck/deadcode/pwsh не обнаружены в текущем PATH.
Это не падение теста проекта. Перед исполнением проверить tool/profile
prerequisites; установки только явным `make tools-install`. Cross-compile не
заменяет native execution. Переиспользование receipt возможно лишь по принятой
политике применимости source/artifact/environment, не как самовольный пропуск gate.

## 4. Документация: один факт у одного владельца

Существующие product/security/technical/operations/reference/development/archive
разделы уже подходят. Новая иерархия целиком не нужна. Убрать из входного маршрута
хронологию кампаний, копии правил и инструкции, описывающие старое назначение.

| Материал | Текущее место знания и действие |
|---|---|
| AGENTS.md | Инварианты, authority/WIP и короткий reader route; без текущих issues, host paths и назначений |
| agent-execution.md | Повторяемый workflow, evidence и handoff; убрать исторические blanket permissions/holds |
| domain-map.md | Семантические границы и collaboration; точные journal-row recipes доступны по ссылкам на технического владельца |
| testing.md | Политика, выбор профиля, команды и limits; условные подробности — в связанных testing references/profile owners, результаты — в ledger/evidence |
| package-map.md / ownership.json / profiles.json | Сохранить как фактические машинно-проверяемые источники; не писать параллельный реестр |
| README владельца / technical owner | Согласовать описания реализованных возможностей; не копировать acceptance chronology |
| ADR/research/archive | Сохранить решения, уникальные измерения и unresolved findings; пометить исторический контекст и source identity |

Для каждого удаления: назвать canonical owner сохранённого факта, проверить
inbound links и anchors, перенести уникальное обязательство, сохранить нужное
provenance, затем убрать копию. Не архивировать действующую норму лишь за объём.
Не переписывать immutable receipts. Точные пакеты и правила не удаляются вместе
с историей их появления. [Правила входных документов](documentation.md#maintained-entry-documents)
закрепляют разделение политики, условных сценариев и результатов. В
[testing](testing.md) есть маршруты к деталям Installation, Route, Admission,
native stand и остальных профилей; [domain map](domain-map.md) направляет к
точным техническим владельцам вместо повторения их процедур.

Готовность: fresh agent находит scope → affected owner → профиль без чтения
всей истории; оператор выполняет поддерживаемый journey по reference/runbook;
нет двух current owners одного правила и битых ссылок. Сокращение числа строк
само по себе не является успехом.

## 5. Скиллы и системные инструкции

Локальный `ddd` адаптирован к существующему brownfield Go:
режимы audit/design/implementation, текущий glossary/domain map, lifecycle и
causal evidence вместо обязательных новых glossary/class diagrams, event bus
и repository для каждого aggregate. Go-guidance убирает универсальный
AggregateRoot как default. Неиспользуемые generic templates и справочники других
языков удалены; upstream attribution и лицензия сохранены. Lock-файл остаётся
записью исходного импорта, а не проверкой целостности локальной адаптации.

Новые универсальные скиллы пока не нужны. Доступные Go-skills применять по
реальной задаче: API/package design для изменения границ, testing для системы
проверок, diagnostics для измеренной проблемы. Не загружать весь skillset в
каждую сессию. Общую проектную политику держать в owners, не размножать в skills.

Проверять инструкции на реалистичных заданиях: plan-only audit в dirty tree;
возобновление Route после handoff; маленькая локальная правка; противоречие parent
и child; legacy deletion при живом tool consumer; пропавший native prerequisite.
Ожидаемое поведение — сохранение scope и работы, правильный owner/профиль и
один следующий результат, без создания лишних файлов и повторного запроса уже
выданных разрешений. Добавлять новый skill только при повторяющемся маршруте,
который нельзя ясно выразить ссылкой на текущего владельца.

## Приёмка удаления legacy и конечного результата

На каждый cohort до удаления требуются: точный old path/symbol/caller inventory;
настоящий новый consumer либо принятое прекращение; disposition данных;
перенесённый независимый byte/behavior oracle; отсутствие production, test,
build-tag, script, process, config и recovery зависимостей. Отдельно проверить
retired command/format refusal до эффектов. Ни wrapper над старым backend,
ни общий live root не закрывают migration.

После удаления обновить package/ownership/profile/build/docs registrations и
проверить конечный source. Findings старых owners закрывать по исправлению или
доказанной неприменимости к оставшимся consumers, а не из-за исчезновения имени
файла. Не удалять Application, diagnostics или qualification лишь потому, что
они находятся вне `successor`.

Итоговая приёмка связывает каждое актуальное D1–D9 и требование выбранного journey
с owner, genuine consumer, profile, candidate identity и результатом. Сохраняются
все исходные failures и честные limits. Walkthrough с одним Product Owner полезен
для продуктовой гипотезы и операторского пути; он не заменяет независимый аудит,
novice-usability, operator independence или anonymity validation.

Практический следующий шаг после выбора стратегии — согласовать ledger и один
ближайший acceptance boundary, завершить его у текущего владельца и измерить
стоимость связанных проверок. Остальные пункты остаются подготовленными до
своих prerequisites; план не запускает их одновременно.
