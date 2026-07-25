# Глобальное исследование возможностей и задач

## 1. Назначение

Этот документ задаёт подготовительный контур для глобального исследования
Ardents. Его задача — не переписать существующий backlog, а отделить:

- уже реализованные возможности, которым нужна только проверка;
- локальные задачи, готовые к реализации;
- ограниченные исследования с понятным результатом;
- глубокие продуктовые, протокольные и эксплуатационные исследования;
- явно отложенные направления, не входящие в текущий `v1`.

Точка подготовки: `main@7c0965c`, 2026-07-24. Рабочая оценка проекта:
`stabilization candidate`, не production release.

Подготовительный R0 baseline зафиксирован 2026-07-25:
`main@75471a6c08bf0c8a130db65d64c7f37dc33f03b5`. Результаты его локальных и
статических gates записаны в
`docs/engineering/evidence/stabilization-baseline-75471a6.md`; это не заменяет
R3 qualification.

Три оставшихся bounded R1-исследования Existing Product Truth выполнены
2026-07-25 на
`main@180decc1b03f94a6115b59a4046b4795308ec235`. Этот research baseline не
заменяет frozen R0 evidence и не является production qualification.

Их выбранные AIJ, OCS и FEC slices реализованы последовательностью до
`main@8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`. Canonical capability
assessment привязан к этому post-R1 source commit; все capability сохраняют
`Q=no`.

## 2. Почему нужен отдельный исследовательский контур

Наличие кода не означает, что возможность доступна пользователю. Для каждой
возможности нужно независимо установить четыре факта:

1. **Implemented** — существует production-реализация.
2. **Reachable** — существует поддерживаемый интерфейс и полный пользовательский
   путь до реализации.
3. **Operable** — состояние, ошибки, восстановление и ограничения наблюдаемы.
4. **Qualified** — обязательные unit, integration, E2E, security и release
   evidence относятся к текущему HEAD.

Исследование не должно называть возможность «готовой», если доказан только
первый или второй факт.

## 3. Источники истины

Источники используются в следующем порядке:

1. production-код, protobuf-контракты, composition root и исполняемые команды;
2. тесты и сохранённое evidence с точной привязкой к commit;
3. действующие ADR и нормативные документы;
4. README и Changelog как входная продуктовая проекция;
5. исторические планы и audit backlog как источник гипотез, но не текущего
   статуса.

Аудит `docs/audit/2026-07-23/` относится к
`main@52af3b2480b62da60ae82c7f1d43f45cd5778230`. После этой точки в HEAD вошло
33 remediation-коммита. Поэтому исходные 24 finding нельзя использовать как
текущий список открытых дефектов без повторной проверки.

## 4. Классы исследовательской готовности

### R0 — Ready

Задачу можно брать в реализацию сразу:

- решение и владелец модуля уже определены;
- не меняется публичный или сетевой контракт;
- нет миграции состояния;
- acceptance проверяется существующим gate или небольшим regression test;
- риск и область изменения локальны.

### R1 — Bounded investigation

Перед реализацией нужен короткий ограниченный spike:

- основной интерфейс уже следует из существующего продукта;
- затрагивается не более двух-трёх модулей;
- нет нового trust model или wire protocol;
- исследование должно закончиться выбранным seam, перечнем изменений и
  acceptance matrix;
- ожидаемый результат — implementable issue, а не новый исследовательский
  проект.

### R2 — Deep research

До реализации требуется отдельный research packet и, вероятно, ADR:

- вводится новая сетевая или security-семантика;
- меняются identity, authorization, privacy или delivery guarantees;
- нужен новый внешний interface;
- затрагиваются несколько доменов и жизненных циклов;
- есть несколько правдоподобных дизайнов;
- ошибка решения создаёт дорогую совместимость или миграцию.

Для R2 сначала проектируются минимум два существенно разных варианта interface.
Выбор оценивается по глубине модуля, leverage для вызывающих сторон, locality
изменений и месту seam.

### R3 — Qualification program

Это не одна feature-задача, а программа доказательства:

- реализация в основном существует;
- завершение определяется матрицей clean build, integration, E2E, security,
  deployment, upgrade и release evidence;
- найденные дефекты возвращаются в R0–R2;
- успех должен относиться к одному точному commit и поддерживаемой среде.

## 5. Карта возможностей

| Возможность | Текущее состояние | Следующая работа | Класс |
|---|---|---|---|
| Node lifecycle и persistent identity | Реализовано; доступно через daemon/CLI | Повторная qualification на текущем HEAD | R3 |
| Operator Interface и CLI/TUI | Реализовано; local Unix и SSH forwarding | UX/evidence review, command smoke | R1/R3 |
| Principal enrollment, sessions, grants, revocation, delegation | Реализовано | Security и recovery qualification | R3 |
| Application identity/session | Реализовано | Проверить полный installation journey | R1 |
| Application Content `Put/Get` | Реализовано и публично доступно | Multi-node ownership/fetch qualification | R3 |
| Waku network foundation | Реализованы Relay, Store, Filter, Lightpush | Churn, hostile peer и multi-host qualification | R2/R3 |
| Node/service discovery | Реализовано для Operator/runtime | Открыть безопасный Application discovery slice | R1 |
| Private discovery/data envelopes | Реализованы фиксированные message classes | Adversarial protocol qualification | R3 |
| Application messaging | Публичного interface нет; `channel.application` зарезервирован | Спроектировать адресацию, channel lifecycle, delivery и receive model | R2 |
| Objects, blobs, manifests | Реализовано через Operator; Content ref доступен Application | Проверить целевые пользовательские сценарии и ограничения размера | R1/R3 |
| Transfer и replication | Реализованы private fetch, sources, commitments и repair | Partition, interruption и scale qualification | R2/R3 |
| Workload lifecycle | Реализован Docker/trusted-process путь | Production Docker failure/soak qualification | R3 |
| Hosted service publication | Runtime и Operator surface реализованы | Спроектировать Application hosting interface | R2 |
| Direct HTTP/HTTPS/TCP service use | Endpoint publication реализована | Определить прикладную auth и client journey | R2 |
| Diagnostics, events, health, metrics | Реализовано | Cardinality/load и operator usability qualification | R1/R3 |
| Configuration reload | Реализовано с restart-required моделью | Failure-matrix qualification | R3 |
| Backup, restore, upgrade, rollback | Реализация и remediation evidence существуют | Полный transactional release drill | R3 |
| Native Linux installation | Qualification candidate | Реальный release/systemd/SSH matrix | R3 |
| Release artifacts и provenance | Реализация gates существует | Полный clean release gate на одном HEAD | R3 |
| Realm Channel Grant authority | Внешняя deployment-owned ответственность | Определить production provisioning/rotation/recovery model | R2 |
| Multi-host deployment | Есть testnet-направление, нет `v1` deployment contract | Выбрать поддерживаемую topology и operations model | R2 |
| Kubernetes/schedulers | Явно unsupported в `v1` | Не исследовать до отдельного product decision | Deferred |
| QUIC/WebTransport/WebRTC | Явно suppressed/unsupported | Не включать в stabilization backlog | Deferred |
| Другие SDK и удалённый Application transport | Не реализовано | После стабилизации Go interface и отдельного mTLS contract | R2, later |

## 6. Задачи, которые можно брать раньше глубокого исследования

### R0 — немедленно готовые

1. Сделать formatting gate воспроизводимым на Windows checkout: Go-файлы
   должны иметь явный LF-контракт независимо от `core.autocrlf`; текущий
   Windows worktree нельзя массово переписывать ради evidence.
2. Запустить обязательные gates на одном clean HEAD и сохранить commit-bound
   evidence.
3. Создать текущий remediation ledger, не изменяя исторический audit baseline:
   `finding -> fix commit -> regression evidence -> current status`.
4. Проверить, что README/Changelog не заявляют больше, чем подтверждено
   release evidence.

### R1 — после короткого spike

1. **Application discovery.**
   Определить минимальный read-only interface для trusted service resolution,
   Application actions/scopes, delegation behavior и privacy-safe errors.
   Исследование завершено в
   `docs/engineering/research/application-discovery.md`: выбран bounded locator,
   подготовительные и продуктовые slices AD-01–AD-05 определены.
2. **Application installation journey.**
   Проверить путь Operator ticket -> Application enrollment -> session ->
   `Content.Put/Get` как один поддерживаемый сценарий.
   Исследование завершено в
   `docs/engineering/research/application-installation-journey.md`: текущий
   публичный путь подтверждён, выбраны protected Ticket Handoff и lifecycle
   acceptance slices.
3. **Operator command smoke.**
   Сопоставить каждую документированную CLI-команду с procedure, action,
   regression test и human/JSON outcome.
   Исследование завершено в
   `docs/engineering/research/operator-command-smoke.md`: каталогизированы 68
   leaf-команд, выбран closed command contract и четыре procedure-level smoke
   slices.
4. **Feature/evidence catalogue.**
   Автоматизировать проверку, что продуктовая возможность имеет owner, interface,
   implementation и актуальное evidence.
   Исследование завершено в
   `docs/engineering/research/feature-evidence-catalogue.md`: будущим
   единственным источником выбран strict JSON, а текущий Markdown register —
   его генерируемой проекцией; определены fail-closed validation и
   commit-bound qualification rules.

R1-задача не переходит в реализацию, пока не назван внешний interface модуля и
не показано, что сложность не протекает в Application SDK или CLI.

## 7. Направления глубокого исследования

### DR-01 — Application messaging

Нужно решить:

- адресат: Principal, conversation, service или channel;
- один-к-одному, группы и membership lifecycle;
- кто выпускает и доставляет Channel Grant;
- online/offline delivery через Relay/Filter/Store/Lightpush;
- acknowledgement, deduplication, ordering и expiry;
- receive model: polling, bounded cursor или stream;
- backpressure, quotas и malicious authorized member;
- малый payload против Content Reference для больших данных;
- revoke/rotate/recovery и совместимость поколений;
- Application actions, Delegation и audit semantics.

Искомый deep module должен скрывать Waku selector, encryption, replay, Store
query и retry за небольшим Application interface. Простое открытие произвольного
`Publish(topic, bytes)` отклоняется.

### DR-02 — Application hosting

Нужно решить:

- регистрирует ли Application существующий локальный endpoint или владеет
  workload;
- lease, readiness, renewal, drain и crash semantics;
- связь Application Principal, workload и published service owner;
- поддерживаемые протоколы и ingress policy;
- authentication удалённого вызывающего;
- local-only против network-published mode;
- отзыв публикации и сохранение истины после restart.

Seam должен находиться над workload/hosting/publication orchestration, а не
раскрывать три отдельных интерфейса вызывающему Application.

### DR-03 — Production Channel Grant authority

Нужно решить:

- trust root и модель realm;
- issuance, delivery, acknowledgement и recovery;
- member add/remove, revocation и generation rotation;
- backup/restore consistency group;
- separation discovery, data и application channels;
- audit и operator workflow без раскрытия capability material;
- single-realm, federation и migration assumptions.

### DR-04 — Multi-host и реальная достижимость

Нужно решить:

- поддерживаемые private-LAN/public-direct topologies;
- bootstrap availability и DNS discovery;
- NAT/firewall и advertised endpoint contract;
- WSS certificate provisioning;
- churn, partition, Store availability и recovery;
- deployment ownership, upgrade ordering и observability;
- минимальная support matrix для первого production release.

### DR-05 — Direct service interaction

Нужно решить:

- является ли Ardents только discovery plane или также выдаёт client adapter;
- end-to-end authentication между Principals;
- authorization сервиса и связь с Access Grant/Delegation;
- TLS identity, certificate rotation и endpoint pinning;
- request/stream limits, retry и error model;
- где заканчивается Ardents interface и начинается application protocol.

### DR-06 — Release qualification

Это R3-программа, но найденные gaps могут потребовать R2:

- один clean commit проходит format, vet, architecture и traceability;
- отдельно проходят fast, integration и E2E;
- проходят required multi-node/multi-host scenarios;
- security exception reconciliation актуальна;
- native install/upgrade/rollback и Compose drill воспроизводимы;
- артефакты имеют immutable source/toolchain/base provenance;
- ни один retry не скрывает flake.

## 8. Зависимости между исследованиями

```text
Current stabilization ───────────────> DR-06 Release qualification
         |
         +--> Application discovery (R1)
         |          |
         |          +--> DR-02 Application hosting
         |          +--> DR-05 Direct service interaction
         |
         +--> DR-03 Channel Grant authority
                    |
                    +--> DR-01 Application messaging
                    |
                    +--> private multi-host operations

Network foundation + deployment ─────> DR-04 Multi-host/reachability
Content Put/Get ──────────────────────> DR-01 large-message references
Workload + hosting + publication ────> DR-02 Application hosting
```

Новые Application features не должны блокировать release qualification
существующего stabilization scope. Если первый release не включает Application
messaging/hosting, это должно быть явным scope decision.

## 9. Research packet

Каждое R1/R2-исследование оформляется одним документом:

```text
Title / decision owner / date / baseline commit
User outcome
In scope / out of scope
Current interface and reachable journey
Current implementation and evidence
Missing behavior
Actors, assets and trust assumptions
Module, external interface and proposed seam
Dependencies:
  - in-process
  - local-substitutable
  - remote but owned
  - true external
Alternative designs
Failure, recovery and migration semantics
Security/privacy/abuse analysis
Observability and operator actions
Acceptance matrix
Open questions
Recommendation:
  - implement
  - prototype
  - write ADR
  - defer/reject
Issue slices and dependency order
```

Tests должны проходить через тот же interface, что и вызывающая сторона.
Внутренние seams допустимы, но не должны попадать во внешний interface только
ради тестов.

## 10. Условия перехода из исследования в разработку

Задача получает статус `ready for implementation`, только если:

- пользовательский outcome сформулирован проверяемо;
- scope и явные non-goals зафиксированы;
- внешний interface модуля выбран;
- seam и зависимости классифицированы;
- authority, identity и ownership не остаются неявными;
- определены happy path, отказ, retry, restart и recovery;
- известны persistence/wire compatibility и migration consequences;
- определены limits, privacy и abuse controls;
- acceptance содержит unit/integration/E2E уровень по риску;
- работа разрезана вертикально, а не по слоям;
- отсутствует открытый вопрос, способный изменить внешний interface.

Если последний пункт не выполнен, задача остаётся research/prototype, даже если
часть реализации кажется простой.

## 11. Порядок проведения глобального исследования

### Wave 0 — Baseline

- закрыть formatting drift;
- получить clean gate snapshot;
- связать remediation commits с findings и evidence;
- заморозить точный research baseline commit.

### Wave 1 — Existing product truth

- подготовить dossiers для Identity, Network, Discovery, Content,
  Transfer/Replication, Workloads/Hosting и Operations;
- для каждого отметить implemented/reachable/operable/qualified;
- выделить gaps, не требующие нового product decision.

Bounded R1 packets для Application installation, Operator command smoke и
feature/evidence catalogue завершены на `main@180decc1b03f94a6115b59a4046b4795308ec235`.
AIJ-01/02, OCS-01–05 и FEC-001/002 реализованы и записаны как закрытые задачи в
локальном tracker. Их последний product commit —
`main@8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.

### Wave 2 — Ready work

- R0 baseline и локальный/static evidence snapshot завершены;
- bounded R1 spikes и выбранные AIJ/OCS/FEC implementation slices завершены;
- Application Discovery AD-01–AD-04 остаются отдельным готовым implementation
  stream, AD-05 — его R3 qualification gate.

### Wave 3 — Deep research

- DR-01 Messaging;
- DR-02 Application hosting;
- DR-03 Channel Grant authority;
- DR-04 Multi-host/reachability;
- DR-05 Direct service interaction.

Wave 3 использует
`docs/engineering/research/wave3-research-charter.md`, integrator-owned decision
register и локальный backlog `.scratch/wave3-deep-research/`. Первая безопасная
параллельная волна — DR-03, DR-02 и DR-04; DR-01 зависит от DR-03, DR-05 — от
DR-02.

### Wave 4 — Scope and release decision

- выбрать функции первого release;
- отделить post-v1 исследования;
- выполнить DR-06 qualification на выбранном scope;
- создать финальный dependency-ordered implementation/release backlog.

## 12. Результат подготовительного этапа

Подготовка считается завершённой, когда:

- существует один текущий capability/evidence register;
- historical audit отделён от current status;
- каждая известная задача имеет класс R0/R1/R2/R3/Deferred;
- для каждого R2 назначен research packet и decision owner;
- release qualification не смешана с разработкой новых features;
- первая волна R0/R1 может выполняться независимо от глубоких исследований.
