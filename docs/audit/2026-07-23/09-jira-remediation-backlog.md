# Jira-like backlog по итогам аудита

Источник: аудит `docs/audit/2026-07-23` на commit
`52af3b2480b62da60ae82c7f1d43f45cd5778230` от 2026-07-23.

Документ предназначен для переноса задач в Jira или совместимый трекер. Он
включает не только дефекты из реестра findings, но и самостоятельные работы по
удалению legacy, консолидации дублей и приведению repository structure к
принятому источнику истины. Ключи `ARD-*` являются локальными
placeholder-идентификаторами. При импорте следует сохранить Audit ID в
отдельном поле или label, например `audit:SEC-001`; задачи без finding ID
помечаются `audit-derived`.

## Правила планирования

- `P1` блокирует production/release-ready статус.
- Сначала восстанавливаются CI, backup и fail-closed release gates, затем
  удалённо достижимые security/availability paths.
- Задача считается закрытой только вместе с deterministic regression evidence.
- Для изменений wire, persisted-state или rollout contract требуемый ADR входит
  в задачу и должен быть принят до реализации.
- Оценка: `S` — до 1–2 дней, `M` — несколько дней, `L` — несколько подсистем
  или миграция. Это относительная сложность, не календарное обязательство.

## Эпики и порядок выполнения

| Epic | Название | Findings | Порядок |
|---|---|---|---:|
| EPIC-01 | Восстановить release gates | CI-001, CI-002, OPS-001, SUP-002 | 1 |
| EPIC-02 | Закрыть удалённые availability-векторы | SEC-002, SEC-003, REL-001, REL-002 | 2 |
| EPIC-03 | Восстановить identity и resource invariants | SEC-001, IAM-001, IAM-002 | 3 |
| EPIC-04 | Ограничить lifecycle и rollout | REL-003, REL-004, OPS-002, OPS-003, OPS-004 | 4 |
| EPIC-05 | Исправить distributed failure semantics | SEC-004, SEC-005 | 5 |
| EPIC-06 | Укрепить release provenance | SUP-001 | 6 |
| EPIC-07 | Закрыть пробелы тестовой архитектуры | QLT-001, TST-001 | сквозной |
| EPIC-08 | Консолидировать архитектуру и документацию | CLI-001, ARCH-001, DOC-001 | 7 |
| EPIC-09 | Удалить legacy и дубли после миграций | Рекомендации раздела 05 | 8 |

## EPIC-01 — Восстановить release gates

### ARD-001 — Починить canonical static CI gate

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | CI-001 |
| Components | CI, Go static analysis, test catalog |
| Estimate / Risk | S / низкий |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Устранить `go vet` copylock в enrollment test helper, переключить workflow на
актуальный `tests/tooling/testcatalog` и зафиксировать поддерживаемый CLI-контракт
каталога тестов.

**Acceptance criteria**

- [ ] `go vet ./...` проходит в clean LF checkout.
- [ ] Canonical static job вызывает существующий testcatalog с актуальными
  аргументами.
- [ ] Валидный каталог принимается, а отдельный некорректный fixture отклоняется.
- [ ] Все downstream jobs могут стартовать после успешного static job.

**Проверка:** clean canonical static run и negative catalog fixture.

### ARD-002 — Сделать native-install gate владельцем evidence directory

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | CI-002 |
| Components | CI, native install, release evidence |
| Estimate / Risk | S / низкий |
| Blocked by | ARD-001 |

**Что сделать**

Создавать report directory идемпотентно внутри native-install gate до smoke-test
и записи evidence.

**Acceptance criteria**

- [ ] Gate проходит в fresh checkout без `tests/.artifacts/native-install`.
- [ ] `passed.txt` создаётся только после успешного systemd smoke.
- [ ] Evidence upload находит и публикует ожидаемый файл.
- [ ] Повторный запуск не требует ручной подготовки каталога.

**Проверка:** удалить только task-owned output, запустить gate и проверить
созданный evidence.

### ARD-003 — Восстановить backup перед Compose upgrade

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | OPS-001 |
| Components | Deployment, Compose, backup |
| Estimate / Risk | S / низкий |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Заменить ссылку на отсутствующий `cluster-data.ps1` на поддерживаемый backup
entry point, проверить путь внутри distribution bundle и сделать backup
обязательным до первого recreate.

**Acceptance criteria**

- [ ] Upgrade из clean distribution bundle находит backup script.
- [ ] Backup создаётся и верифицируется до mutation любого узла.
- [ ] Ошибка backup завершает upgrade без recreate.
- [ ] Позитивный и отрицательный deployment tests сохраняют evidence.

**Проверка:** bundle-level upgrade smoke плюс injected backup failure.

### ARD-004 — Связать manual release `-Commit` с исходным Git tree

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P2 |
| Audit ID | SUP-002 |
| Components | Release, provenance |
| Estimate / Risk | S–M / низкий |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Либо запретить `-Commit`, отличный от `HEAD`, либо собирать exact archive/tree
указанного commit. В обоих вариантах dirty и untracked source должны иметь
явную политику.

**Acceptance criteria**

- [ ] Сценарий `HEAD=B`, `-Commit=A` прекращается до build либо действительно
  собирает tree A.
- [ ] Embedded metadata и verifier ссылаются на реально собранные bytes.
- [ ] Dirty/untracked state обрабатывается fail-closed и документирован.
- [ ] Официальный CI release path продолжает проходить.

**Проверка:** negative mismatch test и clean exact-tree build.

## EPIC-02 — Закрыть удалённые availability-векторы

### ARD-005 — Ограничить unauthenticated Waku Store persistence

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | SEC-003 |
| Components | Waku, storage, observability |
| Estimate / Risk | M / средний |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Ввести обязательные max-size/max-age limits, передать retention policy в
upstream Waku Store, добавить disk-pressure telemetry и определить
degraded/fail behavior для каждого node profile.

**Acceptance criteria**

- [ ] Service node не запускает persistent Store без конечных лимитов.
- [ ] Adversarial relay growth стабилизируется в установленной квоте.
- [ ] Retention сохраняется после restart и cleanup.
- [ ] Disk pressure наблюдаем и переводит узел в документированное состояние.
- [ ] Constrained, restricted и service profiles покрыты тестами.

**Проверка:** bounded retention pressure test с restart.

### ARD-006 — Изолировать ошибки ingress-соединения и супервизировать proxy

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | REL-001 |
| Components | Ingress proxy, workload runtime |
| Estimate / Risk | M / средний |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Классифицировать copy/reset/half-close failures как ошибки соединения, не
процесса, и добавить постоянное наблюдение и восстановление proxy lifecycle.

**Acceptance criteria**

- [ ] RST, copy error и half-close закрывают только затронутое соединение.
- [ ] Listener продолжает принимать новые соединения.
- [ ] Принудительно завершённый proxy автоматически восстанавливается.
- [ ] Supervision имеет bounded backoff и диагностируемое degraded state.
- [ ] Fault scenarios покрыты deterministic tests.

**Проверка:** RST/half-close matrix и proxy-kill recovery test.

### ARD-007 — Ввести timeout и fair admission для ingress

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | REL-002 |
| Components | Ingress proxy, resource admission |
| Estimate / Risk | M / средний |
| Blocked by | ARD-006 |

**Что сделать**

Определить idle/read/write deadlines и заменить единый исчерпаемый budget на
fair admission по порту и/или источнику, сохранив глобальную защиту ресурсов.

**Acceptance criteria**

- [ ] Idle connections освобождаются по явной политике.
- [ ] 128 idle clients не блокируют независимый легитимный поток.
- [ ] Один порт или источник не может монополизировать весь budget.
- [ ] Лимиты конфигурируются или имеют документированные безопасные defaults.
- [ ] Насыщение и rejection наблюдаемы через metrics/events.

**Проверка:** idle exhaustion и cross-port fairness stress tests.

### ARD-008 — Перенести durable replay mutation после авторизации

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | SEC-002 |
| Components | Private messaging, replay protection |
| Estimate / Risk | M / высокий |
| Blocked by | ADR: replay authentication order |

**Что сделать**

Принять короткий ADR о порядке decrypt/class/signature/Publish/replay checks.
Перенести durable mutation после authentication/authorization либо реализовать
bounded pre-auth reservation с atomic promotion.

**Acceptance criteria**

- [ ] Subscribe-only participant не расходует durable replay budget invalid
  envelopes.
- [ ] Запись без валидной signature и Publish permission не становится durable.
- [ ] Concurrent одинаковые deliveries принимаются не более одного раза.
- [ ] Crash/restart не открывает повторно уже принятую запись.
- [ ] ADR и regression tests фиксируют порядок операций.

**Проверка:** poisoning capacity test, concurrent duplicate test, restart test.

## EPIC-03 — Восстановить identity и resource invariants

### ARD-009 — Сделать Object/Manifest identity канонической во всех слоях

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | SEC-001 |
| Components | Content API, policy, persistence, SDK |
| Estimate / Risk | L / высокий |
| Blocked by | ADR: canonical identity и migration |

**Что сделать**

Выбрать identity `(Owner, ID)` или globally unique `ID`, провести её через
proto target, handlers, domain types, store keys, replication/transfer и SDK,
а также определить миграцию существующих записей и collision behavior.

**Acceptance criteria**

- [ ] Policy target, lookup и overwrite используют одну identity.
- [ ] Exact grant Bob `(Bob, X)` не читает и не заменяет Alice `(Alice, X)`.
- [ ] Object и Manifest проходят полную cross-owner Get/Publish matrix.
- [ ] Миграция сохраняет неконфликтные записи и детерминированно обрабатывает
  collisions.
- [ ] Старый persisted state имеет проверенный upgrade/rollback path.

**Проверка:** cross-owner matrix, store invariant и migration tests.

### ARD-010 — Разделить integrity и expiry recovery credentials

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | IAM-001 |
| Components | Identity access, recovery |
| Estimate / Risk | M / средний |
| Blocked by | None — можно начинать сразу |

**Что сделать**

Разделить cryptographic integrity parse и temporal eligibility, определить
retention/compaction истёкших credentials и сохранить fail-closed semantics
для действительно повреждённых записей.

**Acceptance criteria**

- [ ] Expired retained credential не блокирует обнаружение действующего recovery
  credential.
- [ ] Device revocation и revoke активного recovery grant работают в этой
  комбинации.
- [ ] Corrupt credential по-прежнему закрывает неоднозначную операцию.
- [ ] Expired, not-yet-valid, revoked и corrupt состояния различаются.
- [ ] Retention/compaction policy задокументирована и протестирована.

**Проверка:** recovery/revocation state matrix.

### ARD-011 — Реализовать восстанавливаемую доставку one-time tickets

| Поле | Значение |
|---|---|
| Type / Priority | Story / P1 |
| Audit ID | IAM-002 |
| Components | Identity enrollment, bootstrap, CLI |
| Estimate / Risk | L / высокий |
| Blocked by | ADR: ticket delivery state machine |

**Что сделать**

Определить состояния `issued/delivered/acknowledged/expired/reissued` и
реализовать recoverable handoff между durable digest и plaintext delivery.
Plaintext нельзя логировать или оставлять два одновременно действующих ticket.

**Acceptance criteria**

- [ ] Failure после DB commit оставляет восстановимый state.
- [ ] Failure audit flush, RPC delivery и client file create допускает безопасный
  retry/reissue.
- [ ] В каждый момент действует не более одного ticket.
- [ ] Restart сохраняет корректный state и не требует лишнего restart после
  expiry.
- [ ] Plaintext отсутствует в logs, metrics и persistent audit trail.

**Проверка:** failure injection на каждой границе и restart/retry matrix.

## EPIC-04 — Ограничить lifecycle и rollout

### ARD-012 — Сделать shutdown bounded при активных event streams

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | REL-003 |
| Components | Daemon, Node API, lifecycle |
| Estimate / Risk | M / средний |
| Blocked by | Решение о shutdown budget |

**Что сделать**

Связать server `BaseContext` с process signal, явно отменять streams, ввести
bounded drain и доказуемый порядок остановки API и domain cleanup.

**Acceptance criteria**

- [ ] Active `StreamNodeEvents` + SIGTERM завершается раньше systemd timeout.
- [ ] Новые streams не принимаются после начала drain.
- [ ] Domain cleanup выполняется и завершается до process exit.
- [ ] Timeout path диагностируем и не зависает бессрочно.
- [ ] Signal/lifecycle test детерминирован и не требует sleep-based guessing.

**Проверка:** process-level active-stream shutdown test.

### ARD-013 — Ограничить Docker control-plane calls

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | REL-004 |
| Components | Docker adapter, API, metrics |
| Estimate / Risk | M / средний |
| Blocked by | Injectable Docker transport |

**Что сделать**

Провести deadlines/cancellation через Docker adapter, не держать mutex во время
I/O и отдавать bounded cached degraded observation при недоступном engine.

**Acceptance criteria**

- [ ] Hung Docker call не блокирует API, metrics или shutdown.
- [ ] Все background calls имеют конечный deadline и реагируют на cancellation.
- [ ] Mutex не удерживается на внешнем I/O.
- [ ] Degraded observation ограничена по возрасту и явно помечена.
- [ ] Timeout/cancel paths покрыты injected transport tests.

**Проверка:** permanently hung Docker transport test.

### ARD-014 — Сделать rolling rollback транзакционным для текущего узла

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P1 |
| Audit ID | OPS-002 |
| Components | Deployment, rolling upgrade |
| Estimate / Risk | M / высокий |
| Blocked by | ARD-003; ADR: transactional rollout journal |

**Что сделать**

Записывать mutation journal до recreate и включать текущий уже изменённый узел в
compensation set при любой последующей ошибке.

**Acceptance criteria**

- [ ] Journal durable до первого destructive step каждого узла.
- [ ] Failure после recreate, start и readiness возвращает текущий узел.
- [ ] Все ранее изменённые узлы возвращаются к единому fallback digest.
- [ ] Interrupted rollback можно безопасно продолжить.
- [ ] Failure injection покрывает каждую mutation boundary.

**Проверка:** rollout compensation matrix по всем шагам.

### ARD-015 — Ввести composite readiness для rollout

| Поле | Значение |
|---|---|
| Type / Priority | Story / P1 |
| Audit ID | OPS-003 |
| Components | Deployment, diagnostics, identity, network |
| Estimate / Risk | M / высокий |
| Blocked by | ARD-012, ARD-013; ADR: composite readiness |

**Что сделать**

Определить readiness contract, включающий protected API, network, Diagnostics и
retained identity/grants, и использовать его как единственный rollout
acceptance signal.

**Acceptance criteria**

- [ ] Network-only success недостаточен для принятия версии.
- [ ] Degraded API, Diagnostics, identity или grants останавливает rollout.
- [ ] Probe имеет bounded deadline и понятную диагностику причины.
- [ ] Contract одинаков для runtime и deployment tooling.
- [ ] Каждая subsystem degradation покрыта failure-injection test.

**Проверка:** composite readiness degradation matrix.

### ARD-016 — Использовать effective observability endpoint в native rollout

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P2 |
| Audit ID | OPS-004 |
| Components | Native deployment, observability |
| Estimate / Risk | S / низкий |
| Blocked by | ARD-015 |

**Что сделать**

Получать effective configured endpoint конкретного daemon instance и проверять
composite readiness на нём, не доверяя жёстко заданному `127.0.0.1:9090`.

**Acceptance criteria**

- [ ] Alternate loopback port проходит native upgrade/rollback.
- [ ] Посторонний healthy service на `9090` не принимается за целевой daemon.
- [ ] Probe однозначно связан с запускаемым instance/version.
- [ ] Default endpoint сохраняет обратную совместимость.

**Проверка:** alternate-port test и decoy-9090 negative test.

## EPIC-05 — Исправить distributed failure semantics

### ARD-017 — Исправить terminal semantics multi-provider fetch

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | SEC-004 |
| Components | Private transfer, distributed protocol |
| Estimate / Risk | M / высокий |
| Blocked by | ADR: multi-provider fetch semantics |

**Что сделать**

Связать response/error с requested resource и candidate, не считать первый
candidate error глобально terminal и определить deadline, exhaustion и
duplicate-success semantics.

**Acceptance criteria**

- [ ] Ошибка одного trusted candidate не отменяет последующий honest success.
- [ ] Response и error принимаются только для соответствующего request/candidate.
- [ ] All-candidates-fail завершается bounded и диагностируемо.
- [ ] Late и duplicate successes имеют детерминированное поведение.
- [ ] Wire/lifecycle contract отражён в ADR и tests.

**Проверка:** racing malicious-error/honest-success и exhaustion tests.

### ARD-018 — Исключить несколько replay snapshots одного physical store

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P2 |
| Audit ID | SEC-005 |
| Components | Messaging replay, configuration, filesystem |
| Estimate / Risk | M / средний |
| Blocked by | ADR ARD-008 можно расширить ownership-решением |

**Что сделать**

Canonicalize/resolve same-file paths на поддерживаемых ОС либо перейти на один
process-owned store с изолированными namespaces и координированными
transactions.

**Acceptance criteria**

- [ ] Dot, case и symlink aliases отклоняются или безопасно изолируются.
- [ ] Два ledger не заменяют snapshots друг друга.
- [ ] Restart не забывает MessageID ни одного namespace.
- [ ] Реально разные files продолжают поддерживаться.
- [ ] Поведение одинаково определено для поддерживаемых ОС.

**Проверка:** alias matrix и two-ledger restart replay test.

## EPIC-06 — Укрепить release provenance

### ARD-019 — Сделать release materials immutable и проверяемыми

| Поле | Значение |
|---|---|
| Type / Priority | Security / P1 |
| Audit ID | SUP-001 |
| Components | CI, release, supply chain |
| Estimate / Risk | M / высокий для pipeline |
| Blocked by | ARD-001; ADR: release materials policy |

**Что сделать**

Pin builder/runtime base image digests, включить digests source tree, toolchain и
base images в provenance, не отключать provenance без проверяемой замены и
сравнивать double-builds в независимых clean environments/caches.

**Acceptance criteria**

- [ ] Все builder и runtime images адресуются immutable digest.
- [ ] Provenance однозначно перечисляет source, toolchain и base materials.
- [ ] Подмена одного материала ломает verification.
- [ ] Independent builds не используют общий mutable build cache.
- [ ] Release verifier проверяет, а не только отображает provenance.

**Проверка:** clean double-build и material-substitution negative test.

## EPIC-07 — Закрыть пробелы тестовой архитектуры

### ARD-020 — Добавить fault contracts для критичных lifecycle seams

| Поле | Значение |
|---|---|
| Type / Priority | Test / P2 |
| Audit ID | QLT-001 |
| Components | Test architecture, lifecycle, infrastructure |
| Estimate / Risk | L / средний |
| Blocked by | Выполняется сквозно вместе с ARD-005—ARD-019 |

**Что сделать**

Добавить injectable transports, clocks и process supervisors и закрывать
найденные P1 минимальными deterministic negative tests, а не общим coverage
chase. Тесты входят в соответствующие продуктовые задачи; эта карточка
закрывает общую инфраструктуру и полноту матрицы.

**Acceptance criteria**

- [ ] Есть fault contracts для ingress, stream shutdown и hung Docker.
- [ ] Есть regression tests для cross-owner identity, recovery и ticket handoff.
- [ ] Есть tests для replay, multi-provider fetch и Waku pressure.
- [ ] Есть rollout mutation/readiness matrix.
- [ ] Critical-file diff coverage и race/lifecycle gates машинно проверяются.
- [ ] Каждый P1 связан с deterministic regression evidence.

**Проверка:** audit-to-test traceability matrix и полный quality gate.

### ARD-021 — Локализовать и устранить writer-after-stop в discovery test

| Поле | Значение |
|---|---|
| Type / Priority | Bug / P2 |
| Audit ID | TST-001 |
| Components | Integration tests, discovery, lifecycle |
| Estimate / Risk | M / средний |
| Blocked by | Диагностическая фаза; вероятно ARD-012/ARD-013 |

**Что сделать**

Инструментировать ownership goroutines и stores вокруг daemon stop, определить
процесс, продолжающий писать в `TempDir`, устранить lifecycle defect и отделить
targeted stress investigation от итогового release evidence.

**Acceptance criteria**

- [ ] Конкретный writer и незакрытая lifecycle boundary установлены evidence.
- [ ] `Stop` дожидается прекращения background writes.
- [ ] Targeted repeated test не воспроизводит cleanup failure.
- [ ] Нет sleep/retry, маскирующего дефект.
- [ ] После targeted stress один полный integration run проходит без повторов.

**Проверка:** TempDir watcher, goroutine diagnostics, targeted repeat, затем full
integration run.

## EPIC-08 — Консолидировать архитектуру и документацию

### ARD-022 — Унифицировать CLI и SDK Principal session handshake

| Поле | Значение |
|---|---|
| Type / Priority | Refactor / P2 |
| Audit ID | CLI-001 |
| Components | CLI, Go SDK, identity session |
| Estimate / Risk | M / средний |
| Blocked by | После identity/lifecycle P1 |

**Что сделать**

Свести CLI и SDK handshake к одной implementation или общей state machine и
исключить использование stale pre-RPC time при проверке session expiry.

**Acceptance criteria**

- [ ] CLI и SDK используют один session validation contract.
- [ ] Expiry проверяется относительно актуального post-RPC времени.
- [ ] Clock skew и boundary timestamps покрыты общей test matrix.
- [ ] Ошибки и retry behavior согласованы между клиентами.
- [ ] Дублирующая реализация удалена либо сведена к thin adapter.

**Проверка:** shared clock/skew contract tests для CLI и SDK.

### ARD-023 — Синхронизировать architecture acceptance с repository tree

| Поле | Значение |
|---|---|
| Type / Priority | Tech debt / P2 |
| Audit ID | ARCH-001 |
| Components | Architecture, repository governance |
| Estimate / Risk | M / низкий |
| Blocked by | После структурных изменений ARD-005—ARD-022 |

**Что сделать**

Принять и автоматизировать актуальные правила для file budget, package docs,
private generated protocol, числа services и допустимости `.agents` в tracked
tree.

**Acceptance criteria**

- [ ] File budget `12` соблюдается либо исключения формализованы.
- [ ] Требования к package docs отражают фактические packages.
- [ ] Generated/private protocol boundary однозначна и машинно проверяется.
- [ ] Service count и architecture map совпадают с composition root.
- [ ] Политика tracked `.agents` зафиксирована.
- [ ] Acceptance gate проходит в clean checkout.

**Проверка:** machine-readable architecture acceptance и clean CI run.

### ARD-024 — Удалить активные legacy token/loopback инструкции

| Поле | Значение |
|---|---|
| Type / Priority | Documentation / P2 |
| Audit ID | DOC-001 |
| Components | README, runbooks, distribution, protocol docs |
| Estimate / Risk | S / низкий |
| Blocked by | Финальные contracts ARD-009, ARD-011, ARD-015 |

**Что сделать**

На раннем этапе явно пометить legacy token/loopback control model как
недействующий, а после стабилизации полностью удалить активные directives и
обновить entry, runbook, distribution и protocol references под Principal
contract.

**Acceptance criteria**

- [ ] Entry docs не предлагают удалённый token/loopback control API.
- [ ] README, runbooks, distribution и protocol docs описывают один runtime.
- [ ] Все legacy links либо удалены, либо ведут на явно архивный материал.
- [ ] Команды из документации проверяются automated docs smoke.
- [ ] Поиск legacy terminology не находит активных инструкций.

**Проверка:** documentation link/command check и repository-wide legacy scan.

## EPIC-09 — Удалить legacy и дубли после миграций

Аудит не обнаружил второго production runtime, старого HTTP control server,
`vendor/`, отдельной v1 domain implementation или иного мёртвого production
кода, который безопасно удалить немедленно. Поэтому cleanup ниже выполняется
только после миграции callers и фиксации нового источника истины.

### ARD-025 — Удалить дублирующую Principal session implementation

| Поле | Значение |
|---|---|
| Type / Priority | Cleanup / P2 |
| Source | `05-duplication-and-legacy.md`: два session clients |
| Labels | `audit-derived`, `legacy-removal`, `duplication` |
| Components | CLI, Go SDK, identity session |
| Estimate / Risk | S / средний |
| Blocked by | ARD-022 |

**Что удалить**

После перевода CLI и SDK на общий protocol component удалить одну из независимых
реализаций Begin/Complete handshake, timestamp validation и result validation.
Transport-, signer- и persistence-specific adapters сохранить как зависимости.

**Acceptance criteria**

- [ ] Все CLI и SDK callers переведены на общую state machine.
- [ ] Differential parity tests проходят до удаления.
- [ ] Одна из двух security-sensitive handshake implementations удалена.
- [ ] В repository остаётся один владелец timing/failure contract.
- [ ] Поиск Begin/Complete orchestration не находит третьей реализации.

**Проверка:** CLI/SDK parity matrix и repository-wide duplication scan.

### ARD-026 — Удалить устаревшие CI/deploy имена после переключения gates

| Поле | Значение |
|---|---|
| Type / Priority | Cleanup / P1 |
| Source | `05-duplication-and-legacy.md`: legacy names/references |
| Labels | `audit-derived`, `legacy-removal`, `ci`, `deployment` |
| Components | CI, deployment scripts |
| Estimate / Risk | S / низкий |
| Blocked by | ARD-001, ARD-003 |

**Что удалить**

После переключения рабочих путей удалить все ссылки на несуществующие
`cluster-data.ps1`, `tests/cmd/testcatalog` и flag `-mode validate`. Добавить
self-validation, чтобы workflow и distribution bundle не могли снова ссылаться
на отсутствующие entry points.

**Acceptance criteria**

- [ ] Поиск не находит `cluster-data.ps1`.
- [ ] Поиск не находит `tests/cmd/testcatalog` и старый `-mode validate`.
- [ ] CI использует только актуальный testcatalog contract.
- [ ] Deployment bundle содержит и вызывает только поддерживаемый backup entry.
- [ ] Self-validation падает на fixture с отсутствующим script/tool.

**Проверка:** legacy-name scan, clean static gate и bundle validation.

### ARD-027 — Решить судьбу repository-local `.agents` и очистить tree

| Поле | Значение |
|---|---|
| Type / Priority | Decision + Cleanup / P2 |
| Source | `05-duplication-and-legacy.md`: tracked `.agents` |
| Labels | `audit-derived`, `repository-cleanup`, `architecture` |
| Components | Repository governance, agent tooling |
| Estimate / Risk | S / низкий |
| Blocked by | Product/engineering decision; синхронизировать с ARD-023 |

**Что сделать**

Выбрать один вариант: официально разрешить repository-local agent tooling в
target tree либо удалить `.agents` из tracked product repository и перенести
нужные инструкции в утверждённое место. Не оставлять одновременно запрет в
architecture acceptance и tracked files.

**Acceptance criteria**

- [ ] Решение зафиксировано в architecture governance.
- [ ] При варианте «удалить» все необходимые инструкции перенесены до удаления
  `.agents`.
- [ ] При варианте «оставить» target tree и acceptance явно разрешают `.agents`.
- [ ] Clean checkout соответствует выбранному правилу.
- [ ] Gate машинно предотвращает повторное расхождение.

**Проверка:** target-tree acceptance из clean checkout.

### ARD-028 — Вынести private protobuf из handwritten domain package

| Поле | Значение |
|---|---|
| Type / Priority | Refactor + Cleanup / P2 |
| Source | `02-architecture.md`, `05-duplication-and-legacy.md`: generated boundary |
| Labels | `audit-derived`, `generated-code`, `architecture` |
| Components | Private messaging protocol, code generation |
| Estimate / Risk | M / средний |
| Blocked by | ADR либо явное решение ARD-023 о generated boundary |

**Что сделать**

Либо оформить текущее размещение `internal/messaging/private.proto` и
`private.pb.go` как допустимое исключение, либо перенести contract/generated
output в утверждённый generated-контур. При выборе переноса старое generated
расположение удалить только после совместимой regeneration и миграции imports.

**Acceptance criteria**

- [ ] Protobuf wire compatibility подтверждена.
- [ ] Generation script создаёт output только в выбранном canonical location.
- [ ] Все imports и tests переведены до удаления старого файла.
- [ ] Старое generated расположение удалено и не восстанавливается generation.
- [ ] Clean generation check и messaging compatibility tests проходят.

**Проверка:** generation drift check и protobuf compatibility test.

### ARD-029 — Удалить устаревшие architecture assertions

| Поле | Значение |
|---|---|
| Type / Priority | Cleanup / P2 |
| Source | `02-architecture.md`, `05-duplication-and-legacy.md`: stale acceptance |
| Labels | `audit-derived`, `legacy-removal`, `architecture` |
| Components | Architecture documentation, acceptance tooling |
| Estimate / Risk | S / низкий |
| Blocked by | ARD-023, ARD-027, ARD-028 |

**Что удалить**

После принятия новых правил удалить или заменить ложные утверждения о «восьми
generated bounded services», безусловном package budget `≤12`, полном package
documentation и запрете tracked `.agents`. Ослабление правила допустимо только
как явно принятое решение, а не как молчаливое удаление gate.

**Acceptance criteria**

- [ ] Service count соответствует фактическим protobuf services.
- [ ] File budget и исключения описаны без ложного статуса «выполнено».
- [ ] Package documentation rule соответствует выбранному target state.
- [ ] `.agents` policy совпадает с итогом ARD-027.
- [ ] Старые assertions отсутствуют в нормативных и acceptance-документах.

**Проверка:** architecture-doc scan и обновлённый acceptance gate.

## ADR checklist

| ADR | Владелец-задача | До какой реализации нужен |
|---|---|---|
| Canonical Object/Manifest identity и migration | ARD-009 | До изменения schema/store |
| Replay authentication order и durable ownership | ARD-008, ARD-018 | До изменения replay mutation/store |
| Recoverable one-time ticket delivery | ARD-011 | До изменения enrollment state |
| Multi-provider fetch terminal/error semantics | ARD-017 | До изменения protocol handler |
| Composite readiness и transactional rollout journal | ARD-014, ARD-015 | До изменения rollout |
| Release materials/provenance policy | ARD-019 | До изменения release pipeline |

## Release gate / Definition of Done программы

- [ ] Все 16 P1 закрыты и имеют regression evidence.
- [ ] Static, native-install, deployment и release jobs проходят из clean
  checkout.
- [ ] Один полный integration run и обязательные E2E/multinode suites зелёные
  без повторных запусков.
- [ ] Upgrade failure на каждой mutation boundary возвращает все узлы к единому
  fallback state.
- [ ] Artifact verification подтверждает immutable source, toolchain и base
  materials.
- [ ] Architecture acceptance и documentation checks воспроизводимы машинно.
- [ ] Реестр findings содержит ссылки на финальные Jira tickets и evidence.

## Матрица трассируемости

| Audit ID | Jira-like key | Epic | Priority |
|---|---|---|---|
| CI-001 | ARD-001 | EPIC-01 | P1 |
| CI-002 | ARD-002 | EPIC-01 | P1 |
| OPS-001 | ARD-003 | EPIC-01 | P1 |
| SUP-002 | ARD-004 | EPIC-01 | P2 |
| SEC-003 | ARD-005 | EPIC-02 | P1 |
| REL-001 | ARD-006 | EPIC-02 | P1 |
| REL-002 | ARD-007 | EPIC-02 | P1 |
| SEC-002 | ARD-008 | EPIC-02 | P1 |
| SEC-001 | ARD-009 | EPIC-03 | P1 |
| IAM-001 | ARD-010 | EPIC-03 | P1 |
| IAM-002 | ARD-011 | EPIC-03 | P1 |
| REL-003 | ARD-012 | EPIC-04 | P1 |
| REL-004 | ARD-013 | EPIC-04 | P1 |
| OPS-002 | ARD-014 | EPIC-04 | P1 |
| OPS-003 | ARD-015 | EPIC-04 | P1 |
| OPS-004 | ARD-016 | EPIC-04 | P2 |
| SEC-004 | ARD-017 | EPIC-05 | P1 |
| SEC-005 | ARD-018 | EPIC-05 | P2 |
| SUP-001 | ARD-019 | EPIC-06 | P1 |
| QLT-001 | ARD-020 | EPIC-07 | P2 |
| TST-001 | ARD-021 | EPIC-07 | P2 |
| CLI-001 | ARD-022 | EPIC-08 | P2 |
| ARCH-001 | ARD-023 | EPIC-08 | P2 |
| DOC-001 | ARD-024 | EPIC-08 | P2 |

## Матрица audit-derived cleanup

| Источник аудита | Jira-like key | Результат |
|---|---|---|
| Два Principal session clients | ARD-025 | Одна handshake implementation удалена |
| Устаревшие CI/deploy references | ARD-026 | Старые имена и flags удалены |
| Tracked `.agents` против target tree | ARD-027 | Tree очищен либо правило официально изменено |
| Private proto в handwritten package | ARD-028 | Старое generated location удалено либо оформлено исключение |
| Устаревшие architecture assertions | ARD-029 | Ложные acceptance-утверждения удалены |
