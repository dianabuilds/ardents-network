# Дорожная карта устранения

## Принцип порядка

Сначала восстанавливаются проверяющие и аварийные контуры, затем закрываются удалённо достижимые P1, после этого — identity/persistence и lifecycle изменения с высокой ценой ошибки. Документация и structural acceptance обновляются по фактически принятым решениям, а не раньше них.

Оценки ниже — относительные: S (до 1–2 дней), M (несколько дней), L (несколько подсистем/миграция). Они не являются календарным обещанием.

## Этап 0 — Зафиксировать release stop-conditions

До исправления не считать текущий commit production/release-ready и не полагаться на зелёный CI:

- static job не проходит (CI-001);
- release dependency native gate сломан (CI-002);
- Compose upgrade не выполняется (OPS-001);
- rollback/readiness не обеспечивают заявленную транзакционность (OPS-002/OPS-003);
- существуют удалённо достижимые availability paths (SEC-003, REL-001, REL-002).

Критерий завершения: owners согласовали, какие release/deploy пути официально поддерживаются до закрытия этапов 1–4.

## Этап 1 — Быстро восстановить gates и fail-closed операции

| Порядок | Finding | Изменение | Размер / риск | Критерий завершения |
|---:|---|---|---|---|
| 1 | CI-001 | Исправить vet copylock и актуальную testcatalog invocation | S / низкий | Clean LF static job проходит и отрицательный catalog fixture отклоняется |
| 2 | CI-002 | Создавать report directory внутри native gate | S / низкий | Gate в fresh checkout пишет/upload evidence |
| 3 | OPS-001 | Вызывать существующий `data.ps1`, проверить bundle path | S / низкий | Upgrade создаёт verified backup до первого recreate |
| 4 | SUP-002 | Запретить `-Commit != HEAD` либо строить exact Git tree | S–M / низкий | Negative mismatch test падает до build |
| 5 | DOC-001 | Пометить старые token/loopback инструкции как недействующие и направить к Principal contract | S / низкий | Оператор не может выбрать legacy path из entry docs |

Зависимости: этап не требует ADR и должен предшествовать любому release evidence.

## Этап 2 — Закрыть удалённо достижимую availability поверхность

### 2.1 Waku Store — SEC-003

- Ввести обязательные max-size/max-age limits и передать upstream retention policy.
- Добавить disk-pressure metrics и явный degraded/fail behavior.
- Проверить constrained/restricted/service profiles.

Размер M, риск средний. Критерий: adversarial relay growth стабилизируется в установленной квоте после restart/cleanup.

### 2.2 Ingress — REL-001 и REL-002

- Не поднимать connection errors до process-fatal.
- Добавить continuous proxy supervision/restart.
- Определить idle/read/write policy и fair per-port/per-source admission.

Размер M, риск средний. Критерий: RST/half-close не убивает listener; 128 idle clients не блокируют независимый легитимный поток; proxy kill автоматически восстанавливается.

### 2.3 Replay poisoning — SEC-002

- Переместить durable mutation после signature/class/Publish checks либо спроектировать bounded pre-auth reservation с atomic promotion.
- Сохранить защиту от concurrent duplicate delivery.

Размер M, риск высокий. Критерий: invalid inner envelopes не расходуют durable budget, а одновременные дубликаты по-прежнему принимаются не более одного раза.

Для 2.3 требуется короткий protocol ADR: порядок authentication/replay mutation является wire/lifecycle contract.

## Этап 3 — Восстановить identity и resource invariants

### 3.1 Canonical Object/Manifest identity — SEC-001

Нужно решение ADR:

- является ли identity `(Owner, ID)` либо globally unique `ID`;
- как мигрировать существующие collisions;
- как это отражается в proto target, domain types, store key, replication/transfer и SDK.

Не принимать локальную handler-проверку как окончательное исправление, если storage key остаётся owner-agnostic.

Размер L, риск высокий. Критерий: cross-owner matrix и migration tests проходят; policy/store используют одну identity.

### 3.2 Recovery credential classification — IAM-001

- Разделить integrity parse и temporal eligibility.
- Определить retention/compaction истёкших credentials.
- Сохранить fail-closed на действительно повреждённой записи.

Размер M, риск средний. Критерий: expired retained record не мешает действующему recovery path; corrupt record не открывает revoke.

### 3.3 Recoverable one-time ticket handoff — IAM-002

Нужен ADR/state machine для issued/delivered/acknowledged/expired/reissued. Нельзя логировать plaintext или разрешать два одновременно действующих ticket.

Размер L, риск высокий. Критерий: failure injection после DB commit, audit flush, RPC delivery и file create всегда приводит к восстановимому, single-use состоянию.

## Этап 4 — Сделать lifecycle и rollout ограниченными

| Finding | Изменение | Зависимость | Критерий завершения |
|---|---|---|---|
| REL-003 | Signal-bound server BaseContext, явная отмена streams, bounded drain и cleanup order | Согласовать shutdown budget | Active stream + SIGTERM завершается до systemd timeout; domain cleanup доказан |
| REL-004 | Deadline/cancellation для Docker adapter, не держать mutex на I/O, cached degraded observation | Нужен injectable transport | Hung Docker не блокирует API/metrics/shutdown |
| OPS-002 | Journal mutation до recreate и rollback текущего узла | После OPS-001 | Failure после каждого recreate возвращает все узлы к fallback digest |
| OPS-003 | Composite readiness: protected API, network, Diagnostics, retained identity/grants | После REL-003/REL-004 | Каждая injected degraded subsystem останавливает rollout |
| OPS-004 | Использовать effective observability endpoint и связывать probe с daemon | После определения composite probe | Alternate port проходит; decoy 9090 не принимается |

REL-003/REL-004 и OPS-003 лучше решать совместно: deployment не может проверить bounded readiness, пока runtime probe способен зависнуть.

## Этап 5 — Исправить distributed failure semantics

### SEC-004 — Multi-provider fetch

- Bind response/error к requested resource и candidate.
- Не делать первый candidate error глобально terminal.
- Определить exhaustion, deadline и duplicate-success semantics.

Размер M, риск высокий. Желателен protocol ADR. Критерий: malicious trusted error первым не отменяет последующий honest success; all-candidates-fail завершается bounded и диагностируемо.

### SEC-005 — Replay store ownership

- Canonicalize/resolve same-file paths на поддерживаемых ОС либо использовать один process-owned store с namespaces.
- Не полагаться только на строковое сравнение.

Размер M, риск средний. ADR можно объединить с replay ordering/ownership. Критерий: dot/case/symlink aliases отклоняются или изолируются; restart не забывает ID.

## Этап 6 — Укрепить release provenance

### SUP-001

- Pin builder и runtime base image digests.
- Фиксировать digests всех материалов в provenance/metadata.
- Не отключать provenance без эквивалентной проверяемой замены.
- Сравнивать builds в независимых clean environments/caches.

Размер M, риск высокий для pipeline. Критерий: artifact provenance однозначно перечисляет source tree, toolchain и base images; substitution одного материала ломает verification.

## Этап 7 — Тестовая архитектура

Закрыть QLT-001 и TST-001 не общим coverage chase, а fault contracts:

1. ingress RST/idle/fairness/supervision;
2. stream + process signal;
3. hung Docker transport;
4. cross-owner content collision;
5. expired credential recovery;
6. ticket commit/delivery failures;
7. replay capacity и alias restart;
8. trusted multi-responder fetch;
9. Waku retention pressure;
10. rollout mutation/readiness matrix;
11. instrumented discovery shutdown без writer-after-stop.

Критерий: каждый P1 имеет deterministic regression test на минимальном подходящем уровне; integration flake не повторяется в отдельном stress investigation.

## Этап 8 — Консолидация и documentation acceptance

### CLI-001

Свести CLI и SDK session handshake к одной implementation или общей state machine, затем проверить clock/skew boundaries.

### ARCH-001

После изменений решить и автоматизировать:

- file budget 12 либо документированные исключения;
- package docs;
- размещение private generated protocol;
- фактическое число services;
- допустимость `.agents` в tracked tree.

### DOC-001

Полностью удалить активные legacy token/loopback directives, обновить README/runbook/distribution/protocol references.

Критерий этапа: architecture decision matrix, code, tests и entry docs описывают один текущий runtime.

## ADR, необходимые до реализации

1. Canonical identity и migration Object/Manifest.
2. Replay authentication order и durable store ownership.
3. Recoverable one-time ticket delivery.
4. Multi-provider fetch terminal/error semantics.
5. Composite readiness и transactional rollout journal.
6. Release materials/provenance policy.

## Итоговая release gate

Стабилизация завершена, когда:

- P1 закрыты с regression evidence;
- static/native/deployment/release jobs проходят из clean checkout;
- один full integration и требуемые E2E/multinode suites зелёные без повторов;
- immutable source/toolchain/base provenance проверяется;
- upgrade failure на каждом шаге возвращает единое состояние;
- документация и architecture acceptance воспроизводимы машинно.
