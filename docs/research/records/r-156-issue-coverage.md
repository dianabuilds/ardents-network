# Полный объём последующих работ R-155/R-156

Дата фиксации: 2026-09-14. Это карта покрытия согласованных изменений и ссылок на GitHub, а не второй журнал исполнения. Актуальные статусы, выбранная работа и результаты приёмки принадлежат Issues.

## Граница очереди

Сначала полностью завершается итерация #50–#62 по её исходным критериям и интеграции. Все перечисленные здесь задачи самостоятельны и идут после неё. Подготовка этой очереди не запускает реализацию и не добавляет требований в текущую итерацию.

Объём охватывает текущий поддерживаемый проект: удаление прежних исполняемых путей, консолидацию владельцев состояния, исправления поведения, решения о повторном использовании и восстановлении, проверку непрозрачных байтов, эксплуатационные измерения и квалификацию изменённого кандидата. Будущие неизвестные дефекты не спрятаны в задаче «доделать остальное»: новое наблюдение получает собственный ограниченный результат.

## Что считается завершённым результатом

- У поддерживаемого продукта остаётся один принятый защищённый runtime. Прежние входы дают проверяемый отказ; их исключительные dial/listen/forward реализации, положительные runtime-тесты и инструкции удалены.
- Каждый оставленный compatibility reader имеет конкретного потребителя, владельца и основание сохранения. Он не запускает прежнюю сеть и не снижает authority/conflict/resource/adoption floors.
- Нужные AAI3/stream, half-close, Instance/Continuity и terminal механизмы сохранены. Содержание выбирает Application; общая сеть переносит непрозрачные байты в доверенно заданных пределах.
- Issuance, Reader/Publisher prefixes, publication/registration и Job имеют явных закрываемых владельцев. Route владеет законченными admission и Carrier lifecycle; Node сохраняет State/duty и процессные решения.
- Исправления доказаны через настоящих consumers, включая отказы, отмену, поздние результаты и завершение. Изменённые тесты и текущая документация входят в каждый PR.
- Итоговые P1–P11 свидетельства относятся к точному изменённому кандидату и нужным средам на обоих Carriers. Прежний успешный прогон не считается автоматической приёмкой нового состава.
- Каждое открытое consequential решение либо принято и реализовано в своём объёме, либо завершилось явно принятым ограничением. Условная реализация без выбранного контракта не считается готовой.

## Как читать зависимости

Вся таблица имеет общее условие начала: завершённая #50–#62. В колонке зависимостей указаны дополнительные технические предшественники. Это граф, не разрешение параллельной реализации: сохраняется один выбранный implementation WIP и не более одного выбранного активного research question.

Design-задача заканчивается конкретным принятым контрактом и oracles; её implementation-задача не выбирает политику по ходу. Измерение может честно выявить неудовлетворительный результат. Если выбранный контракт не требует изменения schema или восстановления, условная задача закрывается как не требующая реализации со ссылкой на решение, а не как якобы выполненный код.

Перед выбором каждой задачи сверяется итоговый код #50–#62. Полностью выполненный эквивалент подтверждается точным commit/test/evidence; остаток сужается. Уже выполненный перенос или проверка не воспроизводится вторым способом.

## Порядок исполнения

По поручению Product Owner задачи созданы и выполняются по возрастанию номеров
GitHub: #68–#73, #75, затем #76–#136. Все дополнительные технические
предшественники имеют меньший номер. Блоки и строки ниже уже стоят в этом порядке;
номера не являются только порядком публикации. Прежние альтернативные перестановки
отменены. Пропуски номеров принадлежат другим объектам GitHub.

Сначала завершается вся #50–#62. Условная задача получает реализацию по принятому
контракту либо явный disposition о ненужности изменения со ссылкой на решение;
без разрешения её условия нельзя считать зависимую работу принятой.
## Полнота покрытия

На дату публикации: **68 самостоятельных задач** — семь ранее созданных и 61 добавленная. Все 36 карточек R-155 и 26 кандидатов R-156 сопоставлены с Issues; дополнительно выделены одна условная migration-задача из решения M и пять задач полной чистки документации. Y/Z распределены по моменту отказа: #72 включает initial-admission путь до Node, #75 — post-admission handoff.

Это полный известный объём данного архитектурного пересмотра, включая условные варианты. Число не означает 68 обязательных изменений production-кода и не является оценкой длительности. Нерешённый контракт не помечен implementation-ready.

## Исправления #68–#75

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-155/01 | [#68](https://github.com/dianabuilds/ardents-network/issues/68) — Остановить Spend после неоднозначной ошибки записи | Исправление | — |
| R-155/02 | [#69](https://github.com/dianabuilds/ardents-network/issues/69) — Отказать при небезопасном хвосте spend journal | Исправление | #68 |
| R-156/A | [#70](https://github.com/dianabuilds/ardents-network/issues/70) — Сохранить доступ к ready Carrier при чужом заблокированном dial | Исправление | — |
| R-156/X | [#71](https://github.com/dianabuilds/ardents-network/issues/71) — Передавать admission одному владельцу при копировании handle | Исправление | — |
| R-156/Y | [#72](https://github.com/dianabuilds/ardents-network/issues/72) — Сохранить ошибку освобождения при отказе initial admission | Исправление | — |
| R-156/O | [#73](https://github.com/dianabuilds/ardents-network/issues/73) — Разрешить следующую попытку Release после отмены до мутации | Исправление | — |
| R-156/Z | [#75](https://github.com/dianabuilds/ardents-network/issues/75) — Сохранить ошибку освобождения при отказе передачи полученного admission | Исправление | #72 |

## Архитектурные решения, изменения и измерения #76–#97

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-156/B | [#76](https://github.com/dianabuilds/ardents-network/issues/76) — Устранить общую блокировку сессий при outer handshake | Исправление | #70 |
| R-156/C | [#77](https://github.com/dianabuilds/ardents-network/issues/77) — Отделить прогресс независимого child от заблокированного downstream I/O | Исправление | #70, #76 |
| R-156/D | [#78](https://github.com/dianabuilds/ardents-network/issues/78) — Выбрать обеспеченный общей ёмкостью контракт receive credit | Решение | — |
| R-156/E | [#79](https://github.com/dianabuilds/ardents-network/issues/79) — Реализовать выбранное обеспечение receive credit | Условная реализация | #78 |
| R-156/F | [#80](https://github.com/dianabuilds/ardents-network/issues/80) — Выбрать переход живой публикации между parent leases | Решение | — |
| R-156/G | [#81](https://github.com/dianabuilds/ardents-network/issues/81) — Провести живую публикацию через штатное истечение parent | Условная реализация | #80 |
| R-156/I | [#82](https://github.com/dianabuilds/ardents-network/issues/82) — Измерить изоляцию смешанной нагрузки на обоих Carriers | Измерение | #77, #79, #81 |
| R-156/J | [#83](https://github.com/dianabuilds/ardents-network/issues/83) — Измерить стоимость исправленного durable Spend | Измерение | #68, #69 |
| R-156/K | [#84](https://github.com/dianabuilds/ardents-network/issues/84) — Выбрать учёт оставшихся затрат hosting при пополнении | Решение | — |
| R-156/L | [#85](https://github.com/dianabuilds/ardents-network/issues/85) — Применить выбранный учёт hosting exposure одного parent | Условная реализация | #84 |
| R-156/M | [#86](https://github.com/dianabuilds/ardents-network/issues/86) — Выбрать безопасное восстановление hosting в том же периоде | Решение | #84 |
| R-156/M-schema | [#87](https://github.com/dianabuilds/ardents-network/issues/87) — Мигрировать hosting state для выбранного восстановления периода | Условная реализация | #86 |
| R-156/N | [#88](https://github.com/dianabuilds/ardents-network/issues/88) — Восстанавливать hosting в том же периоде штатной командой | Условная реализация | #86, #87, #85, #73 |
| R-156/P | [#89](https://github.com/dianabuilds/ardents-network/issues/89) — Измерить стоимость периодического Observe и его завершения | Измерение | #85, #73 |
| R-156/Q | [#90](https://github.com/dianabuilds/ardents-network/issues/90) — Выбрать область действия ошибки cleanup | Решение | — |
| R-156/R | [#91](https://github.com/dianabuilds/ardents-network/issues/91) — Ограничить распространение parent failure по выбранному cleanup contract | Условная реализация | #90, #72, #75 |
| R-156/S | [#92](https://github.com/dianabuilds/ardents-network/issues/92) — Довести существующие классы отказов до Application caller | Исправление | #90 |
| R-156/T | [#93](https://github.com/dianabuilds/ardents-network/issues/93) — Выбрать конечный бюджет сверки результата issuer после потерь | Решение | — |
| R-156/U | [#94](https://github.com/dianabuilds/ardents-network/issues/94) — Сохранить оплаченный доступ к точным повторам issuer результата | Условная реализация | #93 |
| R-156/V | [#95](https://github.com/dianabuilds/ardents-network/issues/95) — Выбрать жизненный цикл pending issuance между jobs | Решение | #93 |
| R-156/W | [#96](https://github.com/dianabuilds/ardents-network/issues/96) — Освободить следующий job от pending отменённой внутренней операции | Условная реализация | #95, #94 |
| R-156/H | [#97](https://github.com/dianabuilds/ardents-network/issues/97) — Измерить сутки provisioning и восстановление после перезапуска | Измерение | #81, #94, #96, #88 |

## Снятие прежних входов #98–#100

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-155/03 | [#98](https://github.com/dianabuilds/ardents-network/issues/98) — Снять прежний Endpoint runtime plan | Удаление / проверка удаления | — |
| R-155/06 | [#99](https://github.com/dianabuilds/ardents-network/issues/99) — Снять прежние сетевые Name команды до новой композиции | Удаление / проверка удаления | — |
| R-155/07 | [#100](https://github.com/dianabuilds/ardents-network/issues/100) — Запретить прежние Node duty reservations | Удаление / проверка удаления | — |

## Консолидация и сохранение байтового потока #101–#111

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-155/14 | [#101](https://github.com/dianabuilds/ardents-network/issues/101) — Выделить владельца получения permission/token | Консолидация / проверка | #94, #96 |
| R-155/15 | [#102](https://github.com/dianabuilds/ardents-network/issues/102) — Выделить владельца Reader prefixes | Консолидация / проверка | #101, #76, #77, #91 |
| R-155/16 | [#103](https://github.com/dianabuilds/ardents-network/issues/103) — Выделить владельца Publisher prefixes | Консолидация / проверка | #101, #81 |
| R-155/17 | [#104](https://github.com/dianabuilds/ardents-network/issues/104) — Собрать публикацию и регистрацию у одного владельца | Консолидация / проверка | #103, #81 |
| R-155/18 | [#105](https://github.com/dianabuilds/ardents-network/issues/105) — Передать Job полное владение launch/attach и Grant worker | Консолидация / проверка | #102, #104, #91, #96 |
| R-155/19 | [#106](https://github.com/dianabuilds/ardents-network/issues/106) — Упростить закрытие авторизованного контекста | Консолидация / проверка | #101, #102, #103, #104, #105, #91, #72, #75 |
| R-155/20 | [#107](https://github.com/dianabuilds/ardents-network/issues/107) — Передать Route владение receiving admission forwarding | Консолидация / проверка | #68, #69, #71, #72, #75, #85, #91 |
| R-155/21 | [#108](https://github.com/dianabuilds/ardents-network/issues/108) — Передать Route владение forwarding Carrier сессиями | Консолидация / проверка | #107, #70, #76, #77 |
| R-155/22 | [#109](https://github.com/dianabuilds/ardents-network/issues/109) — Отделить параметры нагрузки от текстовой композиции stream | Консолидация / проверка | #105 |
| R-155/23 | [#110](https://github.com/dianabuilds/ardents-network/issues/110) — Подтвердить непрозрачные байты через квалификационный worker | Консолидация / проверка | #109 |
| R-155/28 | [#111](https://github.com/dianabuilds/ardents-network/issues/111) — Сохранить полезную stream-семантику до удаления AAI2 | Консолидация / проверка | — |

## Удаление прежних реализаций #112–#122

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-155/04 | [#112](https://github.com/dianabuilds/ardents-network/issues/112) — Удалить прежний Application Connection путь и AAI2 | Удаление / проверка удаления | #98, #111 |
| R-155/05 | [#113](https://github.com/dianabuilds/ardents-network/issues/113) — Вывести alpha overlay из активных назначений | Удаление / проверка удаления | #98, #112 |
| R-155/08 | [#114](https://github.com/dianabuilds/ardents-network/issues/114) — Удалить прежний Initiator relay | Удаление / проверка удаления | #112, #100 |
| R-155/10 | [#115](https://github.com/dianabuilds/ardents-network/issues/115) — Удалить прежнюю Introduction доставку | Удаление / проверка удаления | #112, #100 |
| R-155/11 | [#116](https://github.com/dianabuilds/ardents-network/issues/116) — Удалить прежний Responder listener | Удаление / проверка удаления | #112, #100 |
| R-155/12 | [#117](https://github.com/dianabuilds/ardents-network/issues/117) — Удалить прежнюю выдачу Transit Grant | Удаление / проверка удаления | #112, #100, #114, #115, #116 |
| R-155/35 | [#118](https://github.com/dianabuilds/ardents-network/issues/118) — Снять прежний профиль запуска Source | Удаление / проверка удаления | — |
| R-155/36 | [#119](https://github.com/dianabuilds/ardents-network/issues/119) — Сохранить безопасный вывод прежней Contributor установки | Удаление / проверка удаления | — |
| R-155/09 | [#120](https://github.com/dianabuilds/ardents-network/issues/120) — Удалить прежний Rendezvous pairing | Удаление / проверка удаления | #112, #100, #119 |
| R-155/13 | [#121](https://github.com/dianabuilds/ardents-network/issues/121) — Удалить исключительные Route и Carrier пути поколения 2 | Удаление / проверка удаления | #114, #120, #115, #116, #117 |
| R-155/24 | [#122](https://github.com/dianabuilds/ardents-network/issues/122) — Закрепить отсутствие прежних runtime в сборке и инструкциях | Удаление / проверка удаления | #98, #112, #113, #99, #100, #114, #120, #115, #116, #117, #121, #118, #119 |

## Квалификация кандидата #123–#131

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| R-155/25 | [#123](https://github.com/dianabuilds/ardents-network/issues/123) — Принять установленный пользовательский сценарий на TCP/TLS | Квалификация | #106, #108, #110, #122, #79, #88, #92 |
| R-155/26 | [#124](https://github.com/dianabuilds/ardents-network/issues/124) — Принять тот же установленный сценарий на QUIC | Квалификация | #123 |
| R-155/27 | [#125](https://github.com/dianabuilds/ardents-network/issues/125) — Принять durable admission и migration при сбоях | Квалификация | #123, #124, #94, #96, #68, #69 |
| R-155/29 | [#126](https://github.com/dianabuilds/ardents-network/issues/126) — Принять wire/flow и volatile lifecycle при сбоях | Квалификация | #123, #124 |
| R-155/30 | [#127](https://github.com/dianabuilds/ardents-network/issues/127) — Принять нагрузку, стоимость и flood bounds | Квалификация | #123, #124 |
| R-155/31 | [#128](https://github.com/dianabuilds/ardents-network/issues/128) — Принять confinement установленного worker | Квалификация | #123, #124 |
| R-155/32 | [#129](https://github.com/dianabuilds/ardents-network/issues/129) — Принять наблюдаемость ролей и correlation report | Квалификация | #123, #124 |
| R-155/33 | [#130](https://github.com/dianabuilds/ardents-network/issues/130) — Принять Instance/Continuity и cryptographic bindings | Квалификация | #123, #124 |
| R-155/34 | [#131](https://github.com/dianabuilds/ardents-network/issues/131) — Принять supply-chain closure итоговых артефактов | Квалификация | #123, #124 |

## Полная чистка документации #132–#136

| Основание | Задача | Вид результата | Дополнительные зависимости |
|---|---|---|---|
| DOC/ADR | [#132](https://github.com/dianabuilds/ardents-network/issues/132) — Привести ADR к фактически сохранённым и заменённым решениям | Документация | #131 |
| DOC/OWNERS | [#133](https://github.com/dianabuilds/ardents-network/issues/133) — Устранить дубли и устаревшие сведения в текущей документации | Документация | #132 |
| DOC/RESEARCH | [#134](https://github.com/dianabuilds/ardents-network/issues/134) — Удалить временные планы и репозиторные дубли задач | Документация | #133 |
| DOC/EXPERIMENTS | [#135](https://github.com/dianabuilds/ardents-network/issues/135) — Удалить завершённые одноразовые эксперименты и старые инструкции запуска | Документация | #134 |
| DOC/CHECK | [#136](https://github.com/dianabuilds/ardents-network/issues/136) — Проверить итоговую документацию и привязку evidence после чистки | Документация | #135 |

## Итоговая приёмка объёма

При завершении очереди сверяются ссылки на принятые результаты всех необходимых задач и решения по условным ветвям. Это сверка evidence, а не новая большая implementation-задача. Для одного кандидата должны согласоваться installed journeys (#123/#124), durable/migration (#125), wire/lifecycle (#126), resources (#127), confinement (#128), role observations (#129), cryptographic bindings (#130) и supply-chain closure (#131). Изменение кандидата инвалидирует затронутые результаты по действующему qualification owner.

Source/profile/deadcode проверка #122 не освобождает удаления #98–#100 и #112–#121 от их собственных тестов и документов. Измерения #82/#83/#89/#97 дают отдельное эксплуатационное evidence; они не подменяют обязательную квалификацию.

После code qualification выполняются #132–#136. Принятые ADR и завершённые research records сохраняются как evidence по documentation policy; заменённые решения сняты с current route, а временные планы, дубли и disposable instructions удалены после promotion. #136 проверяет change-impact документационных изменений на source/artifact/evidence manifests и повтор затронутых gates.

Основания: [принятые карточки R-155](r-155-change-proposals.md), [пересмотр R-156](r-156-runtime-architecture-use-review.md), [порядок и устранение дублей](r-156-change-sequencing.md), [ADR-0086](../../adr/0086-consolidate-protected-network-and-retire-predecessor-runtimes.md). Статусы исполнения обновляются только в GitHub.
