# R-155 — Атомарные предложения изменений

Уточнение Product Owner 2026-09-14: сначала завершается вся итерация #50–#62; все архитектурные изменения — отдельные последующие задачи. [Сопоставление R-155/R-156 и Issues](r-156-change-sequencing.md) фиксирует этот порядок. 01/02 опубликованы как #68/#69; они не являются новыми условиями приёмки текущей итерации. Принятый ADR-0086 и условия retirement сохраняются.

Дата: 2026-09-14. Основание — [архитектурное решение R-155](r-155-network-core-consolidation.md).
Состав и размер карточек приняты Product Owner 2026-09-14 и закреплены в
[ADR-0086](../../adr/0086-consolidate-protected-network-and-retire-predecessor-runtimes.md).
Это зафиксированное основание GitHub Issues, **не текущий backlog, не список
начатых задач и не дочерние задачи #50**. После публикации GitHub владеет исполнением
и актуальными зависимостями; изменения карточек согласуются в соответствующих Issues. Идентификаторы ниже
локальны этому документу; они не становятся доменными именами и wire полями.

Полная публикация 2026-09-14: все 36 карточек сопоставлены с отдельными GitHub Issues в [карте полного объёма](r-156-issue-coverage.md), вместе со всеми кандидатами R-156 и условной hosting migration. Тексты карточек ниже сохраняют происхождение решения; реальные зависимости и уточнённые границы принадлежат опубликованным Issues. Новая работа начинается после всей #50–#62; эквивалент уже выполненного результата подтверждается evidence и исключается из остатка.
## Условия допуска

- **Fix:** отдельное исправление после завершения всей #50–#62; перед выбором проверить наличие уже сделанного эквивалента. В последующей очереди исправления предшествуют зависимым внутренним переносам. Не обходить единственный C0 WIP.
- **Retirement:** политика совместимости принята в ADR-0086. Исполнение зависит
  от принятого установленного successor, миграции #61 и полной приёмки #50;
  принятие дизайна не заменяет эти условия.
- **Refactor:** сохраняет текущий продуктовый контракт; выполнять после принятия
  successor, на актуальном интегрированном HEAD и вне активной реализации #60.
- **Qualification:** повторная проверка конкретного изменённого кандидата;
  она не содержит неизвестного остатка реализации предыдущих задач.

Условия Retirement задают конкретно предложенную политику, а не задачу
«исследовать, что удалить». Product Owner принял её 2026-09-14; зависимые карточки ожидают доказанного cutover. Если фактическая миграция #61 уже снимает какой-то
вход, соответствующая карточка сокращается до оставшегося результата или
вообще не создаётся. Изменения #60/#61/#62 не клонируются.

## Общий контракт каждой карточки

В GitHub тело каждой согласованной задачи должно включать её собственные пункты
ниже и следующие обязательства, а не ссылаться на абстрактное «потом проверить»:

1. Один законченный результат, переключённый настоящий вызывающий код, тесты
   нормального/отказного/прерванного пути и изменения owning documentation в одном PR.
2. Текущие [product bounds](../../product/protected-service-workload.md),
   [threat model](../../security/threat-model.md),
   [протокол](../../technical/protected-route-protocol.md) и
   [private admission](../../technical/private-admission.md) не ослабляются.
   Нет второго счётчика бюджета, новой identity, повторной выдачи полномочий,
   автоматического peer/Carrier/generation fallback или нового runtime dependency.
3. `make quick-check` при изменении и `make check` перед integration.
   [Профили](../../development/testing.md) выбираются по реально затронутому коду:
   Linux endpoint требует Linux; проверка установленного worker — выбранной Ubuntu.
   Windows не доказывает выполнение Linux-only кода. Отсутствующий prerequisite
   означает invalid environment, не passing skip.
4. Изменённые `doc.go`, package-map, ownership, test profiles и deadcode inventory
   обновляются в том же PR, **если затронуты**. Нельзя расширять allowlist ради
   оставленного без причины старого движка или объявлять gate необязательным.
5. Исторические conformance vectors остаются доказательствами своей версии;
   актуальные тесты и инструкции относятся к новому поведению. Удаление
   положительного старого теста требует принятого снятия поддержки и проверки
   отказа прежнего входа. Тесты приватной раскладки полей не заменяют поведение.
6. Приёмка содержит точный commit/artifact, среду, исполненные команды и результат.
   Если выявлено незаданное изменение authority, privacy, wire или поддерживаемого
   сценария, остановить зависимое изменение и вернуть его в дизайн.

**Признак слишком большой задачи:** чтобы принять описанный результат, приходится
заодно проектировать другую роль, завершать другой пользовательский сценарий,
менять wire или откладывать обязательные тесты/документы. Такую карточку делят
до реализации. Несколько файлов или большой diff удаления сами по себе не
означают несколько самостоятельных результатов.

## Карта зависимости

| Карточка | Один проверяемый результат | Зависимость |
|---|---|---|
| 01 | Ошибка записи останавливает новые Spend | Fix, после всей #50–#62 |
| 02 | Recovery не отбрасывает потенциально подтверждённый Spend | 01 |
| 03 | Прежний Endpoint plan отказывает до сетевых эффектов | Retirement |
| 04 | Удалён прежний путь Application Connection вместе с AAI2 | 03, 28 |
| 05 | Alpha overlay перестаёт быть принимаемым назначением | 03, 04 |
| 06 | Прежние сетевые Name команды больше не запускают обмен | Retirement |
| 07 | Конфигурация Node больше не запускает прежние duties | Retirement |
| 08 | Удалён прежний Initiator relay | 04, 07 |
| 09 | Удалён прежний Rendezvous pairing | 04, 07, 36 |
| 10 | Удалена прежняя Introduction доставка | 04, 07 |
| 11 | Удалён прежний Responder listener | 04, 07 |
| 12 | Удалена прежняя Transit Grant выдача | 04, 07, 08, 10, 11 |
| 13 | Удалены оставшиеся исключительные Route/Carrier механизмы поколения 2 | 08–12 |
| 14 | Выдача контекста имеет отдельного закрываемого владельца | Refactor |
| 15 | Reader prefixes имеют одного владельца | 14 |
| 16 | Publisher prefixes имеют одного владельца | 14 |
| 17 | Публикация и регистрация переключаются одним владельцем | 16 |
| 18 | Job закрывает собственный launch/attach/Grant | 15, 17 |
| 19 | Закрытие контекста состоит из stop/join дочерних владельцев | 14–18 |
| 20 | Forwarding admission закрывается целиком внутри Route | 01, 02; Refactor |
| 21 | Forwarding Carrier сессии закрываются одним владельцем Route | 20 |
| 22 | Общая композиция stream получает доверенные параметры нагрузки | 18 |
| 23 | Нетекстовые байты проходят тот же установленный защищённый путь | 22; принят механизм нагрузки #60 |
| 24 | Сборка и документы подтверждают отсутствие прежних runtime входов | 03–13, 35, 36 |
| 25 | Итоговый установленный TCP/TLS сценарий подтверждён | 19, 21, 23, 24 |
| 26 | Тот же установленный сценарий подтверждён на QUIC | 25, без новых изменений кандидата |
| 27 | Durable admission и migration выдерживают сбои | 25, 26 |
| 28 | Нужные AAI2 stream-сценарии сохранены в AAI3 до удаления | Refactor gate; до 04 |
| 29 | Wire/flow и concurrent lifecycle выдерживают сбои | 25, 26 |
| 30 | Итоговый кандидат укладывается в ресурсные пределы | 25, 26 |
| 31 | Установленный worker сохраняет confinement | 25, 26 |
| 32 | Role observations сохраняют границы сведений и честный correlation verdict | 25, 26 |
| 33 | Криптографические bindings проверены на итоговом кандидате | 25, 26 |
| 34 | Supply-chain evidence соответствует итоговым артефактам | 25, 26 |
| 35 | Source больше не запускается с прежним native profile | Retirement |
| 36 | Contributor сохраняет только безопасный вывод старой установки | Retirement; до 09 |

Это граф происхождения логических зависимостей. Актуальная очередь исполняется после всей #50–#62 по возрастанию номеров GitHub, как указано в полной карте покрытия. Локальные номера карточек ниже не задают порядок Issues. Координационная запись содержит ссылки, без остаточной реализации.

**Уточнение зависимостей R-156 (2026-09-14):** все карточки этой карты,
#68–#73, #75 и решения о pending, credit, lifetime и cleanup относятся к отдельной
работе после завершения всей итерации #50–#62. Они не добавляют ей детей или
условий приёмки. В последующей очереди 14–21 получают уже исправленное состояние
и сокращаются на фактически сделанное. Исходные workload и qualification
обязательства #60/#62 сохраняются. Каждое новое изменение включает свои тесты
и актуальную документацию. Полное сопоставление — в
[уточнённой последовательности](r-156-change-sequencing.md).
## 01 — Запретить Spend после неоднозначной ошибки записи

**Тип:** Fix. **Результат:** получающий Node не допускает следующий токен через
ledger, у которого неопределённо завершилась изменяющая файл операция.

**Что изменить:** ledger под своей блокировкой сохраняет первую storage failure
и прекращает новые мутации/admission до явного закрытия и проверенного открытия.
Одинаковое правило применяется к ошибке append и изменяющего pruning. Вызывающий
протокол возвращает существующий отказ, не переоткрывает ledger автоматически.
Формат токена и journal не меняется; не добавлять общий persistence framework.

**Приёмка и тесты:**

- [ ] После fault в записи A следующий Spend(B) отказывает без записи в файл.
- [ ] Регрессия моделирует именно реальный незавершённый хвост на ещё открытом
  владельце, а не добавление мусора после его закрытия.
- [ ] Проверены частичная запись, ошибки первого/второго sync, commit marker и
  close: ни одна ошибка не превращается в разрешение работы.
- [ ] Ранее успешный Spend остаётся запрещён после открытия; concurrent Spend
  после первой ошибки не обходит запрет; repeated Close присоединяется корректно.
- [ ] Поведение подтверждено через receiving admission seam: отказ не создаёт
  разрешённый channel и освобождает только допустимые временные резервы.

**Документы:** private admission, комментарий ledger об ambiguous write.
**Среда:** обычные route/node checks и race на поддерживаемых ОС.
**Blocked by:** выбранный текущим владельцем слот исправления; не новая кампания.

## 02 — Отказывать при небезопасном хвосте spend journal

**Тип:** Fix. **Результат:** reopening не теряет полную запись после первой
незавершённой записи, выдавая получателю ложное «токен ещё не принят».

**Что изменить:** recovery сначала классифицирует хвост, затем при допустимом
конечном незавершённом record обрезает его один раз до установленной границы.
Если за ним имеется ещё полная запись, owner не открывается и файл не меняется.
Ошибки recovery остаются отказом; automatic repair или перенос в новый root не вводится.

**Приёмка и тесты:**

- [ ] Последовательность incomplete A → complete B отклоняется без мутации файла.
- [ ] Разрешённый конечный частичный/незавершённый хвост восстанавливается с
  сохранением всех предшествующих committed Spend.
- [ ] Некорректный header/binding, committed corruption, ошибка truncate/sync
  не возвращают пригодный owner; повтор открытия не улучшает результат случайно.
- [ ] Receiving startup не объявляет readiness при непригодном journal.

**Документы:** private admission: допустимый crash tail и отказ повреждения.
**Среда:** disk behavior route + receiving startup, race. **Blocked by:** 01.

## 03 — Снять прежний Endpoint runtime plan

**Тип:** Retirement. **Результат:** принятая установка не начинает v1 participant
по старому JSON и не переключается на него при ошибке v2.

**Что изменить:** убрать принимающую диспетчеризацию v1 из команды. Сохранить
короткий явный отказ старой schema; новый plan допускается только при совпадении
действующих generation/authority/migration guards. Не конвертировать root/Grant.
Убрать из текущей справки и операторских примеров способ запуска старого runtime.

**Приёмка и тесты:**

- [ ] Полный ранее допустимый v1 plan отказывает до dial, listen и создания
  новых runtime roots; mixed v1/v2 поля также отказывают.
- [ ] Повреждённый v2 plan, недоступная State и отсутствующая confinement не
  вызывают v1; нижние границы принятой миграции не меняются.
- [ ] Установленная команда по действующему v2 plan доходит до обычного ready
  с теми же prerequisites, без тестовой подмены startup.

**Документы:** команды, Endpoint process contract, installation/adoption owner.
**Среда:** command behavior + установленная Ubuntu.
**Blocked by:** принятие решения A, завершённая миграция и приёмка #50.

## 04 — Удалить прежний Application Connection путь и AAI2

**Тип:** Retirement. **Результат:** в поддерживаемом Endpoint/CLI отсутствует
второй Connection listener/client и его пользовательская Route-композиция.

**Что изменить:** удалить исключительные v1 participant Connection composition,
AAI2 клиент/сервер и файловый CLI клиент этого пути; прекратить экспорт бывшего
RunParticipant как принимающего entrypoint. Общее нужное successor поведение
сохранить у действующего владельца. Administration v1 и его текстовые команды
сохраняются. Исторические AAI2 vectors не превращать в AAI3 vectors.

**Приёмка и тесты:**

- [ ] В production imports/entrypoints нет AAI2 Connection и прежнего
  user-route Application path; architecture inventory это подтверждает.
- [ ] Старый локальный клиент не принимается AAI3 decoder, а accepted text
  read и Administration publish/withdraw работают по прежнему контракту.
- [ ] Cancellation setup, Write/CloseInput race и full Close проверены у
  оставшегося AAI3 владельца; необходимые сценарии не исчезли с v1 тестами.

**Документы:** scope, Endpoint, оба affected interface owners, package/ownership/
profiles, command reference. **Среда:** Linux Endpoint/IPC + portable AAI3 checks.
**Blocked by:** 03, 28. Старые Role wire codecs удаляются позднее, не в этой карточке.
**Обязательное сохранение:** уже общий half-close pipe и native Service Connection,
включая используемые successor wire identities со строкой `v2` — решение F.

## 05 — Вывести alpha overlay из активных назначений

**Тип:** Retirement. **Результат:** `ardents-alpha://` и прежний corpus intake
не образуют альтернативу Target Link в текущем продукте.

**Что изменить:** снять принимающие legacy Service Link поля/ветвь и команды
нового принятия alpha corpus. Удалить исключительные resolution adapters этого
пути. Сохранить нужную проверку уже подписанной истории и обязательных floors
как decode/refusal-only совместимость; она не может открыть сеть или принять
более старый corpus. Канонический `naming`/Namespace не удалять.

**Приёмка и тесты:**

- [ ] Alpha destination и прежняя тройка plan-параметров дают определённый отказ;
  не выполняются resolution, dial или приём нового corpus.
- [ ] Обычный Target Link работает; Name не становится alpha alias.
- [ ] Существующие serial/conflict floors сохраняются; lower/same-conflicting
  corpus не возвращается после рестарта или неудачной миграции.
- [ ] Retained verification не импортирует старый сетевой runtime и имеет
  названного потребителя либо обязательное историческое основание.

**Документы:** naming compatibility, Endpoint, команды control/inspection.
**Среда:** command/alpha persistence checks. **Blocked by:** 03, 04.

## 06 — Снять прежние сетевые Name команды до новой композиции

**Тип:** Retirement. **Результат:** текущие `name resolve/control` больше не
вызывают прежний HTTP/OHTTP client по операторскому plan.

**Что изменить:** команды дают ограниченный `not-selected`/unsupported outcome
до сети; их accepting command adapters удаляются. Локальный canonical encode,
Namespace lifecycle/proofs, custody signing и module Resolution contract
сохраняются. AAI3 Name остаётся запрещённым. Не реализовывать новый Gateway.

**Приёмка и тесты:**

- [ ] Ранее корректный resolution/control input не вызывает transport и не
  меняет Namespace/authority состояние; отказ объясняет отсутствие выбранного runtime.
- [ ] Canonical encode и чистые signed-proof проверки сохраняют байты/результаты.
- [ ] Документы различают module readiness, операторский отказ и отсутствие
  полного публичного сервиса; нет обещания `.ard` или alias fallback.

**Документы:** commands, naming, scope; module test inventory сохраняется.
**Среда:** command + naming unit. **Blocked by:** решение A и приёмка #50.
**Вне задачи:** R-149, публичный producer, новый транспорт private Resolution.

## 07 — Запретить прежние Node duty reservations

**Тип:** Retirement. **Результат:** новый выпуск Node не слушает прежнее поколение
по старым reservations, issuer schema или resource profile.

**Что изменить:** закрыть принимающие legacy branch в command/runtime dispatch;
оставить короткие typed/schema refusals. State по-прежнему выбирает только
допущенную duty; signed old identities не переименовываются в generation 3.

**Приёмка и тесты:**

- [ ] Старые Rendezvous/Initiator/Introduction/Responder/Transit issuer plans,
  их смеси с closed полями и old placement отказывают до listen/root creation.
- [ ] Все действующие closed duties сохраняют корректные startup, потерю
  current State и drain на обоих выбранных Carriers.
- [ ] Старый конфиг не обходит adoption guard; key/floor файлы остаются неизменными.

**Документы:** Node contract, command/operator reference, resource placements.
**Среда:** Node command/process tests. **Blocked by:** решение A и принятая миграция.
**Вне задачи:** удаления отдельных движков 08–12.

## 08 — Удалить прежний Initiator relay

**Тип:** Retirement. **Результат:** legacy Initiator relay/listener и его
исключительная grant/relay composition больше не входят в поддерживаемый код.

**Что изменить:** удалить закрытый в 07 поток целиком до его private helpers;
используемые generation-3 forwarding primitives оставить без изменения.

**Приёмка и тесты:**

- [ ] Нет production вызовов/регистрации прежнего relay; старый plan по-прежнему
  отвергается, а защищённое adjacent forwarding сохраняет admission и cleanup.
- [ ] Тесты исключительной старой роли сняты из active inventory; общие
  аутентификация, replay и failure cases не потеряны при удалении.

**Документы:** Node/Route, package-map, affected profiles. **Среда:** route/node
unit/process/race, оба Carrier для затронутого forwarding. **Blocked by:** 04, 07.

## 09 — Удалить прежний Rendezvous pairing

**Тип:** Retirement. **Результат:** остаётся выбранный generation-3 JOIN; старый
pairing/listener не является вторым способом соединить участников.

**Что изменить:** удалить исключительный прежний Rendezvous runtime и его
положительные qualification lanes; сохранить обязательные исторические receipts.
Новые JOIN bytes, admission, credit и terminal не менять.

**Приёмка и тесты:**

- [ ] Старое wire/plan не запускает pairing; нет active old listener/callers.
- [ ] Новый JOIN соединяет точную пару и отказывает неправильному binding;
  cancellation, одновременный close и peer loss проходят существующие проверки.
- [ ] Dedicated-host исторические числа не выдаются за новую qualification.

**Документы:** Node, qualification/profile inventory, operations.
**Среда:** JOIN process/race на обоих Carriers. **Blocked by:** 04, 07, 36.

## 10 — Удалить прежнюю Introduction доставку

**Тип:** Retirement. **Результат:** Introduction обслуживает только выбранную
registration/capsule композицию, без старого slot/control runtime.

**Что изменить:** удалить исключительные old listener/control/delivery adapters.
Service-only capsule, current/previous overlap и существующий replay owner
generation 3 сохраняются, общая reviewed crypto не переписывается.

**Приёмка и тесты:**

- [ ] Старый вход отказывает; successor registration → capsule delivery
  выполняется через штатные роли без старого slot adapter.
- [ ] Повтор, expired capsule, withdrawal и отмена доставки не создают второго
  exchange и не восстанавливают предыдущую публикацию.

**Документы:** Node/Route, private reachability, inventories.
**Среда:** Introduction + Endpoint process/race. **Blocked by:** 04, 07.

## 11 — Удалить прежний Responder listener

**Тип:** Retirement. **Результат:** Publisher достигается только через выбранную
защищённую сторону Route и authenticated Service Connection.

**Что изменить:** удалить исключительный legacy listener/attachment path;
generation-3 Responder forwarding и Service TLS/Instance проверку сохранить.

**Приёмка и тесты:**

- [ ] Legacy plan/wire не открывает Service; successor Publisher stream
  получает Application I/O только после нужной аутентификации.
- [ ] Прерванный attach, чужой Instance и потеря Publisher отказывают с
  исходными terminal outcomes и без возвращения старого пути.

**Документы:** Node, Endpoint/Service, package/profiles.
**Среда:** Publisher/Service process/race, оба Carrier. **Blocked by:** 04, 07.

## 12 — Удалить прежнюю выдачу Transit Grant

**Тип:** Retirement. **Результат:** новый runtime не создаёт и не выдаёт старые
Transit Grants; closed admission issuance остаётся единственным выбранным путём.

**Что изменить:** удалить исключительный старый issuer runtime, командную
инициализацию и неиспользуемую acquisition composition. Сохранить проверки
исторического подписанного State и обязательных root/floor guards. Не удалять
closed token wallet, permission signing или их custody purpose.

**Приёмка и тесты:**

- [ ] Старый issuance request/config не создаёт key/root и не выдаёт Grant.
- [ ] Closed issuance, duplicate/restart, budget и receiver spending проходят
  текущие проверки; bound roots нельзя перепривязать новой authority.
- [ ] Для каждого оставшегося старого decoder назван decode-only потребитель;
  нет legacy `Issue`/`Serve` за новым facade.

**Документы:** private admission, Node, custody/command owner при изменении входа,
dependency/ownership inventories при фактическом сокращении.
**Среда:** credential/node/endpoint disk/process/race. **Blocked by:** 04, 07, 08, 10, 11.

## 13 — Удалить исключительные Route и Carrier пути поколения 2

**Тип:** Retirement. **Результат:** активный Route не содержит прежний движок
Attach/relay/LegBinding transport, оставшийся после снятия его потребителей.

**Что изменить:** удалить его исключительную композицию и codecs из runtime;
сохранить TCP/TLS и QUIC нового поколения и необходимое чтение прежних signed
State identities. Общие используемые TLS/bounds primitives остаются у одного
действующего владельца. Не объединять две wire grammar через negotiation.

**Приёмка и тесты:**

- [ ] Поддерживаемые command closures не содержат старых dial/listen/forward
  entrypoints; profile/generation mismatch даёт отказ.
- [ ] Один и тот же защищённый сценарий работает на обоих Carriers, включая
  частичный frame, cancellation и bounded cleanup.
- [ ] Бывшие исключительные профили удалены из active тестов; нужные migration
  fixtures остаются и не запускают прежние сессии.

**Документы:** Route `doc.go`, Network/Route/Node, transport/compatibility owner,
package-map и профили. **Среда:** command closure + route/node process/race.
**Blocked by:** 08–12. **Вне задачи:** новые транспорты, перенос старых wire в v3.

## 14 — Выделить владельца получения permission/token

**Тип:** Refactor. **Результат:** контекст поручает одному закрытому объекту
issuance operation и ожидает её cleanup, не меняя из разных методов её поля.

**Что изменить:** перенести текущий flight, cancellation, handoff и error outcome
под одного владельца внутри Endpoint. Существующие durable wallet/permission
formats и их authority остаются прежними. Context даёт существующую авторизацию
и закрывает admission, а не сам правит flight.

**Приёмка и тесты:**

- [ ] Получение и последующее использование токена идут через штатный Endpoint.
- [ ] Одновременные requests не создают лишнюю issuance; отмена до/во время
  handoff и поздний ответ после revoke не выдают второй usable token.
- [ ] Restart/durable outcome и отсутствие reset при новом job сохранены.

**Документы:** private admission, Endpoint ownership/lifecycle.
**Среда:** Linux Endpoint credential integration/race. **Blocked by:** Refactor gate.

## 15 — Выделить владельца Reader prefixes

**Тип:** Refactor. **Результат:** открытие, удержание и закрытие Reader Source
prefixes выполняются через один объект с собственным admission и completion.

**Что изменить:** перенести prefix state, opening flight и соответствующий
retained selection; Context больше не управляет полями prefix напрямую.
Entry/Interior persisted выбор и его срок не меняются; ошибкой не выбирается
новый peer и не возобновляется bootstrap allowance.

**Приёмка и тесты:**

- [ ] Обычный read получает ту же защищённую acquisition через новый owner.
- [ ] Cancellation opening, idle retirement, deadline и concurrent close
  освобождают каждый lease один раз; next job не обходит retained selection.
- [ ] Медленный/пропавший role не порождает unbounded retry или второй prefix.

**Документы:** Endpoint/private admission/protected prefix lifecycle.
**Среда:** Linux reader process/race, оба Carrier. **Blocked by:** 14.

## 16 — Выделить владельца Publisher prefixes

**Тип:** Refactor. **Результат:** Introduction/Responder prefix lifecycle
обслуживается одной закрытой Publisher-композицией.

**Что изменить:** перенести acquisition, current retained prefixes, opening
и retirement. Передавать publication owner только операции acquire/release
с существующей binding, не изменяемые карты и не custody material.

**Приёмка и тесты:**

- [ ] Publish/read/withdraw использует тот же отобранный prefix без новой
  выдачи permissions при смене worker.
- [ ] Сбой открытия второй части закрывает только принадлежащее попытке;
  отмена, stale State и одновременный shutdown не теряют cleanup.
- [ ] Registration не удерживает ссылку на уже retired prefix как usable.

**Документы:** Endpoint, private admission/reachability.
**Среда:** Linux Publisher process/race, оба Carrier. **Blocked by:** 14.

## 17 — Собрать публикацию и регистрацию у одного владельца

**Тип:** Refactor. **Результат:** update/withdraw имеет одну точку смены current
пары Publication/Registration и один порядок завершения.

**Что изменить:** current/previous registration, overlap deadline, refresh,
withdrawal и late completion принадлежат одному объекту внутри Endpoint.
Он использует существующие Publication/Instance владельцы и не присваивает их
ключи. Context не объявляет current независимо от него.

**Приёмка и тесты:**

- [ ] Успешное обновление открывает новую согласованную пару; неуспешное не
  публикует половину пары и не снижает generation/revision floor.
- [ ] Withdrawal немедленно закрывает новые acquisition и завершается только
  после положенного drain; late refresh не воскрешает публикацию.
- [ ] Previous overlap истекает по исходному deadline даже при recovery;
  UI/Administration получают прежние typed outcomes.

**Документы:** Endpoint publication ownership и private reachability.
**Среда:** Linux publication/refresh/process/race. **Blocked by:** 16.

## 18 — Передать Job полное владение launch/attach и Grant worker

**Тип:** Refactor. **Результат:** одна invocation отвечает за запуск, проверенную
передачу attachment и cleanup; контекст не собирает эти части вручную.

**Что изменить:** закрытый job owner сохраняет reservation, qualified launch
handoff, worker Grant и принадлежащие job exchanges. Persistent permissions,
prefix selections и replay/floors остаются контекстными, не пересоздаются в job.
Не обобщать launcher до произвольных приложений.

**Приёмка и тесты:**

- [ ] Reader/Publisher job начинает Application I/O только через существующий
  verified launch и binding; possession nonce не заменяет confinement receipt.
- [ ] Отмена на каждом handoff и worker loss дают один итог и joined cleanup;
  поздний attachment закрывается, не присоединяясь к следующему job.
- [ ] Повторный запуск не возвращает consumed permission и retained resource budget.

**Документы:** Application confinement, Endpoint job lifecycle.
**Среда:** Linux behavior/race + установленная Ubuntu launch boundary.
**Blocked by:** 15, 17.

## 19 — Упростить закрытие авторизованного контекста

**Тип:** Refactor. **Результат:** Context закрывает admission и stop/join своих
владельцев; он не очищает их внутренние maps и flights.

**Что изменить:** заменить ручное управление извлечёнными полями конечной
композицией Stop/Wait. Установить порядок ожидания из решения B, сохранить первый
содержательный outcome и отдельное свидетельство cleanup failure. Stop не ждёт
другого owner под mutex; повторный Close присоединяется к одной операции.

**Приёмка и тесты:**

- [ ] Revoke при issuance, prefix opening, registration refresh и активном job
  прекращает новые эффекты и дожидается всех ранее принятых работ.
- [ ] Concurrent/repeated Close, late callbacks и peer silence не создают
  deadlock, leak или повторного release; исходные deadlines не расширяются.
- [ ] Последний release авторизации не предшествует cleanup зависимых ресурсов.

**Документы:** Endpoint таблица ownership и shutdown; закрытые owner comments.
**Среда:** Linux lifecycle/process/race; детерминированные барьеры вместо sleep.
**Blocked by:** 14–18. **Вне задачи:** изменение recovery/terminal wire.

## 20 — Передать Route владение receiving admission forwarding

**Тип:** Refactor. **Результат:** Node запускает admission forwarding одной
операцией, не создавая и не закрывая отдельно spend/limits/bootstrap объекты.

**Что изменить:** один Route owner принимает уже проверенную receiver binding,
выделенный root, clock и действующие resource inputs; сам открывает связанные
владельцы и закрывает их при любой неуспешной инициализации/остановке.
Node оставляет current State/duty и процессную политику. Остальные роли не переносить.

**Приёмка и тесты:**

- [ ] Штатный forwarding startup, token admission и drain проходят через новый
  seam; каждый init failure после открытия ledger освобождает его lease.
- [ ] Replay, poisoned journal, exhaustion и withdrawal отказывают до работы.
- [ ] Поздний admission после Stop не приобретает новый ресурс; счётчики и
  reserved control budget используют принятый #60 owner.

**Документы:** Network/Route/Node, private admission, package surface.
**Среда:** Node/Route disk/process/race. **Blocked by:** 01, 02 и Refactor gate.

## 21 — Передать Route владение forwarding Carrier сессиями

**Тип:** Refactor. **Результат:** pool/listener и forwarding session cleanup
закрываются одной bounded lifecycle операцией Route.

**Что изменить:** продолжить owner из 20, включив открытие/закрытие существующих
Carrier ресурсов. Node передаёт только текущие проверяемые факты и ограниченные
local settings; готовые pool/listener из Node не передаются. Сохранить функцию
остановки duty, process pressure и bounded usage reporting.

**Приёмка и тесты:**

- [ ] TCP/TLS и QUIC проходят тот же receiver contract без выбора fallback.
- [ ] Смена/потеря duty, peer loss, очередь на пределе и shutdown отменяют и
  join все сессии до освобождения pool/ledger; контрольные резервы не теряются.
- [ ] Node больше не знает порядок закрытия внутренних Route ресурсов;
  ошибка physical cleanup не преобразуется в успешный terminal.

**Документы:** Network/Route/Node, protected lifecycle, Route `doc.go`.
**Среда:** оба Carrier process/race. **Blocked by:** 20.

## 22 — Отделить параметры нагрузки от текстовой композиции stream

**Тип:** Refactor. **Результат:** общая Service-stream композиция получает
проверенные directional limits и lifecycle bounds без импорта текстового codec.

**Что изменить:** доверенный host вычисляет прежний набор границ из выбранного
workload; общая композиция использует эти значения при тех же authorization,
attachment и stream constructors. Применить одновременно к Reader и Publisher.
Размер и UTF-8 проверки snapshot остаются в текстовом приложении/доверенном UI.

**Приёмка и тесты:**

- [ ] Текстовый сценарий имеет те же принятые границы в обоих направлениях,
  включая framing overhead; превышение отвергается в прежней фазе.
- [ ] Caller Application не может выбрать больший бюджет/срок или непроверенный
  attachment; byte limits не сбрасываются при recovery.
- [ ] Stream передаёт допустимые bytes без UTF-8 разбора, а trusted text
  presentation по-прежнему отвергает некорректный текст.

**Документы:** Endpoint ownership, workload boundary, Service Connection.
**Среда:** Linux Endpoint/stream + text Application checks. **Blocked by:** 18.
**Вне задачи:** SDK, DSL, datagram и generic installed Application.

## 23 — Подтвердить непрозрачные байты через квалификационный worker

**Тип:** Refactor verification. **Результат:** принятый в #60 фиксированный
stream-test Application переносит нетекстовый payload по тому же защищённому
пути и с той же launch/Grant boundary.

**Что изменить:** добавить отсутствующий проверяемый binary corpus к уже
выбранной нагрузке. Если #60 уже содержит эквивалентную проверку, новую задачу
не создавать. Не вводить сетевой test bypass или штатный arbitrary-worker режим.

**Приёмка и тесты:**

- [ ] Corpus включает все значения байта, NUL, некорректные UTF-8 последовательности,
  границы frame и несоответствие границ Write/Read; итоговая последовательность точна.
- [ ] Half-close, slow receiver и одна разрешённая attachment recovery не
  дублируют байты и не меняют terminal semantics; workload остаётся в пределах.
- [ ] Оба Carrier проверяются с реально установленным launch path. Текстовый
  worker сохраняет свой текстовый контракт и не принимает binary как snapshot.

**Документы:** qualification workload и execution profile; это свидетельство
байтового ядра, не выпуск второго пользовательского приложения.
**Среда:** установленная Ubuntu, qualified test Application. **Blocked by:** 22 и #60.

## 24 — Закрепить отсутствие прежних runtime в сборке и инструкциях

**Тип:** Retirement verification. **Результат:** возврат старого entrypoint,
qualification lane или production import обнаруживается существующим
архитектурным контролем, а текущий reader route описывает только принятый продукт.

**Что изменить:** сверить и уточнить существующие source/command/profile/deadcode
inventory для удалённых 03–13 путей. Сохранить historical evidence за её явной
границей. Проверить актуальные cross-links и инструкции запуска в чистом
представлении источников. Это финальная cross-check, не отсрочка документации.

**Приёмка и тесты:**

- [ ] Старые runtime отсутствуют в production closures для поддерживаемых ОС;
  `.ard` Browser archive не входит в discovery/build/test/packaging.
- [ ] Позитивный active тест не требует запуска удалённой роли; необходимые
  migration/refusal checks остаются в назначенных профилях.
- [ ] Документы и commands не обещают AAI2/new alpha intake/старые roles/Name
  runtime. Administration v1 и чистый Namespace описаны как сохраняющиеся.
- [ ] `make headless-check` и обязательные архитектурные проверки подтверждают
  актуальный контракт артефактов, без ослабления их ограничений.

**Документы:** owning scope/technical/operations/reference + inventories.
**Среда:** проверяемые source/artifact profiles. **Blocked by:** 03–13, 35, 36.

## 25 — Принять установленный пользовательский сценарий на TCP/TLS

**Тип:** Qualification. **Результат:** конкретный итоговый кандидат выполняет
publish → read → update → read → withdraw → refusal на выбранной Ubuntu.

**Что выполнить:** заморозить commit, build/tool closure, артефакты и среду;
подготовить State/permissions штатным способом и выполнить сценарий командами
на TCP/TLS. Использовать действующий evidence format и внешнее хранилище
observations. Не завершать здесь недостающий продуктовый код.

**Приёмка:**

- [ ] Перед обновлением читается старый snapshot, после — новый; после withdrawal
  новые acquisition отказывают, принятый drain соблюдён.
- [ ] Выдача, launch, registration и read не подменены fixture orchestration.
- [ ] Сохранены complete observations, verdict и точные artifact identities;
  UI текст, текущие инструкции и исполненный путь совпадают.

**Документы:** qualification receipt и актуальная инструкция при найденной
неточности; не добавлять повторный live status в scope.
**Среда:** installed Ubuntu profile. **Blocked by:** 19, 21, 23, 24.

## 26 — Принять тот же установленный сценарий на QUIC

**Тип:** Qualification. **Результат:** тот же кандидат и пользовательские
outcomes подтверждены на QUIC при State-selected Carrier.

**Что выполнить:** повторить заранее заданный сценарий 25, изменив только
разрешённую Carrier конфигурацию. Новых defaults/fallback не добавлять.

**Приёмка:**

- [ ] Полный publish/read/update/withdraw путь и отказы соответствуют 25.
- [ ] Evidence относится к тому же source/build candidate; отличие конфигурации
  задокументировано. Если код изменён, 25 больше не доказывает новый кандидат.
- [ ] Нет TLS/QUIC-specific разницы в Application ordered-stream semantics.

**Документы:** QUIC qualification receipt/operation specifics при необходимости.
**Среда:** installed Ubuntu profile. **Blocked by:** 25.

## 27 — Принять durable admission и migration при сбоях

**Тип:** Qualification. **Результат:** принятые Spend, полномочия и migration
floors не теряются и не воскрешают прежний runtime после сбоя.

**Что выполнить:** disk/issuance/adoption часть P5, P7 и P10 действующей матрицы
на точном кандидате. Исполнить выбранные interrupted transitions и перепроверить
состояние штатным restart, без ручной очистки roots. Verification/bootstrap floods
из P5 относятся к карточке 30, worker/volatile recovery из P7 — к 29 и 31.

**Приёмка:**

- [ ] Spend fault/restart, duplicate redemption и exhausted voucher не создают
  новое разрешение. Небезопасный хвост не открывается как чистый ledger.
- [ ] Interrupted adoption, старый executable/peer и stale credential не
  включают прежнее поколение; conflict/resource/authority floors сохранены.
- [ ] Disk full/corrupt/leased root даёт отказ без нового root или сброса бюджета.
- [ ] Полное evidence позволяет воспроизвести verdict и привязать failure к
  отдельной реализации, а не дописать её внутри qualification.

**Документы:** qualification evidence/change-impact, admission/migration owner.
**Среда:** действующие installed/durable-failure profiles на обоих Carriers.
**Blocked by:** 25, 26. Existing #62 evidence используется только в пределах
правил точного кандидата и влияния изменений, не как автоматический pass.

## 28 — Сохранить полезную stream-семантику до удаления AAI2

**Тип:** Refactor verification. **Результат:** AAI3 имеет явное поведенческое
покрытие нужных возможностей общего байтового потока, ранее покрытых у AAI2.

**Что изменить:** сопоставить существующие сценарии и добавить только отсутствующее
покрытие у AAI3: binary bytes, fragmentation, half-close, setup cancellation,
terminal и full Close. Зафиксировать компактные AAI3 conformance vectors для
типизированного запроса и его отказов. Не копировать AAI2 decoder или создавать
общий transport framework: основные stream-механизмы уже совпадают.

**Приёмка и тесты:**

- [ ] При разных размерах Write/Read передаются все значения байта без изменения;
  EOF направления не прекращает чтение обратного ответа.
- [ ] Отмена во время initial request/status и гонка Write/CloseInput/Close
  завершаются одним outcome без unowned socket и дублирования bytes.
- [ ] Frame bound, reserved Name, wrong magic и truncated terminal отвергаются;
  AAI2 request не принимается как AAI3.
- [ ] Таблица соответствия existing/new tests показывает, что именно сохранено;
  дублировать уже существующий тест с новым именем не требуется.

**Документы:** Application Interface v2 contract/conformance; положительный список
решения F. **Среда:** portable client и Linux server профили + race.
**Blocked by:** Refactor gate. Выполнить **до 04**, а не после удаления доказательств.
**Вне задачи:** запуск generic Application; это локальный IPC semantic contract.

## 29 — Принять wire/flow и volatile lifecycle при сбоях

**Тип:** Qualification. **Результат:** новый состав владельцев сохраняет P2 и
volatile часть P7: допустимые bytes/terminal и завершение принятой работы.

**Что выполнить:** declared malformed frames, credit/offset нарушения, blocked
lane, поздние callbacks, revoke/withdraw во время exchange и peer loss/recovery.
Использовать тот же принятый candidate и оба Carrier.

**Приёмка:**

- [ ] Неверные/oversize/truncated/unknown records не вызывают запрещённых эффектов.
- [ ] Блокировка одного lane не лишает разрешённого прогресса другие; отмена и
  concurrent Close join принадлежащие им ресурсы в исходные deadlines.
- [ ] Recovery не повторяет Application request, не воскрешает публикацию и не
  выбирает иной Target/профиль. Terminal EOF не теряет последнее допустимое чтение.

**Документы:** P2/P7 evidence и change-impact. **Среда:** selected process/adversarial
profiles. **Blocked by:** 25, 26. Нет доработки новой функциональности внутри задачи.

## 30 — Принять нагрузку, стоимость и flood bounds

**Тип:** Qualification. **Результат:** после рефакторинга сохраняются пределы P8
и bounded receiver work для bootstrap/verification floods из P5.

**Что выполнить:** существующую численную матрицу reference/NET-14AD, cold/warm,
long stream, idle/failed refresh, concurrency/open limits, host allowance и
admission floods. Не подменять её одним benchmark локального codec.

**Приёмка:**

- [ ] На обоих Carriers выполнены все выбранные budget gates с полным
  receive/forward/control/failed-attempt accounting.
- [ ] Нет нового контекстного allowance при job/retry/restart; terminal-control
  запас остаётся доступным при исчерпании data capacity.
- [ ] Все degraded/failed результаты сохранены; во время overload нет
  неограниченного allocation/queue или молчаливого ослабления защиты.

**Документы:** P5/P8 evidence и актуальный cost owner только при принятом изменении.
**Среда:** selected installed/resource/impairment profiles. **Blocked by:** 25, 26.

## 31 — Принять confinement установленного worker

**Тип:** Qualification. **Результат:** P6 и worker-loss часть P7 подтверждены для
артефактов с новым Job/Context составом владельцев.

**Что выполнить:** положительные controls, IPv4/IPv6/UDP/DNS/host IPC, inherited
descriptors, namespace/syscall/path и child-tree попытки из принятой матрицы.

**Приёмка:**

- [ ] Контрольные попытки вне confinement работоспособны, а запрещённые эффекты
  внутри выбранного worker отсутствуют; ошибка установки даёт отказ до Grant.
- [ ] Потеря worker и cancellation launch не оставляют child tree/attachment
  и не сохраняют чужой Grant; новый job не получает прежнюю авторизацию.
- [ ] Квалификационный binary worker не является обходом production launch policy.

**Документы:** P6/P7 evidence, confinement owner. **Среда:** выбранная установленная
Ubuntu, реальные units/cgroup. **Blocked by:** 25, 26.

## 32 — Принять наблюдаемость ролей и correlation report

**Тип:** Qualification. **Результат:** консолидация не расширяет доступные ролям
сведения; сохранены ограничения claims из P3 и диагностического P9.

**Что выполнить:** declared role-state/input captures для startup, resolution,
admission, publication, stream и cleanup; предусмотренные комбинации ролей и
корреляционные воздействия на обоих краях. Хранить чувствительные captures
только в выбранном внешнем evidence root, без публикации raw секретов.

**Приёмка:**

- [ ] Нет запрещённого объединения origin/Target или нового межконтекстного
  идентификатора в состояниях/diagnostics после объединения владельцев.
- [ ] Evidence покрывает данные ролей, не только шифротекст packet capture.
- [ ] Успешная корреляция отражена как принятый предел конструкции, а отсутствие
  успеха в прогоне не объявляется доказательством анонимности.

**Документы:** P3/P9 evidence, threat-model claim conditions при нужном уточнении.
**Среда:** выбранный role-observation/adversarial profile. **Blocked by:** 25, 26.

## 33 — Принять Instance/Continuity и cryptographic bindings

**Тип:** Qualification. **Результат:** P4 подтверждён для неизменённых по контракту,
но иначе скомпонованных authentication и admission путей.

**Что выполнить:** существующие standard vectors/independent verification,
wrong SPKI/key/cohort/receiver, altered capsule, replay, wrong Instance и
предусмотренную later-key capture проверку. Не писать новые crypto primitives.

**Приёмка:**

- [ ] Проверки вызываются до Application effects, а повреждённые bindings
  не принимаются после переноса lifecycle ownership.
- [ ] Shared native Service Connection с исторической строкой `v2` сохраняет
  точные выбранные bytes и commitments; новый wire не введён случайно.
- [ ] Evidence не заявляет защиты от захваченного live recipient key там,
  где текущий контракт её не даёт.

**Документы:** P4 evidence, действующий crypto/dependency owner.
**Среда:** selected crypto/role profiles. **Blocked by:** 25, 26.

## 34 — Принять supply-chain closure итоговых артефактов

**Тип:** Qualification. **Результат:** P11 и обязательные repository gates
относятся к точному итоговому выпуску, без старого исполняемого наследия.

**Что выполнить:** build/tool/OS/runtime/worker inventory, актуальную acceptance
проверку зависимостей и advisories, canonical artifact representation и
обязательный `make check`. Не устанавливать инструменты неявно.

**Приёмка:**

- [ ] Для каждого известного finding имеется fix или scoped reproducible
  non-applicability; поддержка не выводится только из exit code scanner.
- [ ] Source/build identities согласованы с 25–33; изменение бинарных inputs
  инвалидирует затронутые qualification результаты.
- [ ] Артефакты и текущие инструкции не включают retired entrypoints/Browser lanes;
  все remaining limitations отражены у своих владельцев.

**Документы:** P11 evidence, dependency/operations owner и artifact inventory.
**Среда:** поддерживаемые build profiles и exact installed artifacts.
**Blocked by:** 25, 26; итоговая приёмка требует также 27 и 29–33, без новой задачи
«дописать остальное». Это может быть закрытие координационной записи по ссылкам.

## 35 — Снять прежний профиль запуска Source

**Тип:** Retirement. **Результат:** новый Source обслуживает только явно
выбранное закрытое поколение под существующей pinned authority.

**Что изменить:** закрыть прежний native selector и неявный выбор старого
профиля в Source command configuration. Проверка signed history, State acceptance,
root lease, refresh и generation guards остаются у State. Не менять Direct-Origin
transport и не вводить новый State authority или public discovery.

**Приёмка и тесты:**

- [ ] Старый native selector, его смесь с closed полями и отсутствие необходимого
  explicit profile/pin дают отказ до Source listener и нового runtime state.
- [ ] Новый Endpoint получает действующую State через штатный Source с прежними
  authentication, conflict/exposure и freshness проверками.
- [ ] Migration/inspection может проверить допустимую старую signed историю,
  но не открыть этим прежний serving runtime или обнулить floors.

**Документы:** Network State/Source owner, command reference, profile/ownership
inventory. **Среда:** Source/State command/process checks и installed startup.
**Blocked by:** решение A и принятая миграция. Если это уже сделано в #61,
отдельная карточка не создаётся.

## 36 — Сохранить безопасный вывод прежней Contributor установки

**Тип:** Retirement. **Результат:** новым выпуском можно диагностировать,
drain/withdraw и явно удалить собственную старую установку; нельзя начать
старую работу через apply, restart или recovery перед диагностикой.

**Что изменить:** снять принимающие apply/restart ветви dedicated Rendezvous.
Сохранить ограниченный retirement control под существующим root lease и
проверенной deployment identity. Interrupted-update запись не разрешает
start/restart: неоднозначность даёт bounded refusal без смены поколения.
Drain/withdraw затрагивают только доказанно принадлежащий owner unit; Remove
сохраняет прежнее подтверждение deployment ID и предварительный withdrawal.
Не переносить старую установку автоматически в closed duty и не удалять чужие roots.

**Приёмка и тесты:**

- [ ] Apply/restart старого bundle отказывают до установки или запуска процесса.
- [ ] Diagnose при interrupted-update marker не вызывает SupervisorStart/Restart;
  опасная неопределённость отражена, а не исправлена запуском старого executable.
- [ ] Drain/withdraw прекращают только свой unit; повторы идемпотентны, чужая
  identity/root отказывает. Remove требует нужного явного подтверждения.
- [ ] Старые generation/conflict floors не сбрасываются и новый closed Node
  не наследует старую duty через переименование profile.

**Документы:** Contributor runbook, Node lifecycle и ownership/profile inventory.
**Среда:** детерминированный Supervisor boundary плюс выбранная Ubuntu/systemd
проверка реальной установки; missing privilege не является skip.
**Blocked by:** решение A и принятая миграция; выполнить до 09. Новая Contributor
программа управления защищёнными duties в эту карточку не входит.

## Что намеренно не превращается в задачу

- «Сделать DSL», «поддержать UDP», «перенести Amnezia», «сделать публичные Names»:
  отсутствуют выбранные контракты; это будущие design вопросы своих владельцев.
- «Обновить все тесты/документы в конце»: owning изменения обязательны в каждой
  карточке; 24–27 и 29–34 проверяют целое по разным конкретным границам.
- «Удалить всё из old»: ветка исключена Product Owner.
- «Убрать все слова legacy/v1/alpha»: persisted identities и исторические
  проверки не являются вторым сетевым решением; удаление определяется поведением.

## Завершение согласования

Предмет согласования — решение о конце совместимости, размер карточек и
зависимости. После него: принять необходимые consequential решения в их
владельцах/ADR, перенести допущенные карточки в GitHub в порядке зависимостей,
подставить настоящие issue IDs и точные принятые requirements links.
Нельзя обозначать зависимую Retirement карточку implementation-ready до этого.
Этот файл затем остаётся неизменяемым обоснованием разбиения; текущие статусы,
обнаруженные остатки и исполнение ведутся в Issues.
