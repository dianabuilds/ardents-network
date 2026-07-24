# Архитектура

## Точка и метод реконструкции

Архитектура восстановлена для `main@52af3b2480b62da60ae82c7f1d43f45cd5778230` по точкам входа, графу Go-пакетов, composition root, protobuf-контрактам, хранилищам, deployment/release-скриптам, тестам и нормативной документации. Документация использовалась как проверяемое утверждение, а не как безусловный источник истины.

## Фактическая архитектура

Проект представляет собой модульный Go-монолит с тремя исполняемыми файлами:

- `cmd/ardentsd` — узел, владеющий состоянием, локальными API, сетевыми протоколами, workload runtime и наблюдаемостью;
- `cmd/ardentsctl` — операторский CLI и SSH/local transport-клиент;
- `cmd/ardents-ingress-proxy` — отдельный TCP-прокси для ingress workload-контейнеров.

Основная композиция находится в `internal/daemon`. Она связывает доменные пакеты, ConnectRPC-серверы, Waku/libp2p-адаптер и Docker-адаптер. Публичные protobuf/Connect-контракты находятся в `api/ardents`; Go SDK — в `sdk/go`.

Фактические слои и направления зависимостей:

```text
cmd/*, ardents.ps1
  -> internal/daemon | internal/cli | internal/ingressproxy
       -> internal/localapi | internal/applicationapi
            -> identity/access + domain services
       -> content, discovery, hosting, messaging, policy,
          publication, replication, transfer, workload
       -> network/waku         (Waku/libp2p adapter)
       -> workload/docker      (Docker/Moby adapter)
       -> storage              (bbolt/JSON/filesystem primitives)
  -> api/ardents/*             (generated RPC and message contracts)
  -> sdk/go                    (public application client)
```

Go не допускает циклические импорты. Дополнительный `importguard` подтверждает, что Waku/libp2p не выходит за `internal/network/waku`, а Docker SDK — за `internal/workload/docker`.

## Ключевые границы модулей

| Граница | Владелец | Фактический контракт | Оценка |
|---|---|---|---|
| Operator/Application API | `internal/localapi`, `internal/applicationapi`, `api/ardents` | Два Unix ConnectRPC surface, отдельные audience и наборы процедур | Граница выражена хорошо; часть старой документации всё ещё описывает token/loopback API |
| Identity и authorization | `internal/identity`, `internal/identity/access`, interceptors API | Principal/device credentials, sessions, grants, canonical action/target, sealed admitted call | Централизация сильная, но owner-sensitive content target теряет Owner в handler/storage |
| Network transport | `internal/network/waku` | Waku/libp2p relay, store, filter, light-push, discovery transport | Адаптер изолирован; upstream store включён без retention |
| Private messaging | `internal/messaging` | Sign-then-encrypt, replay ledger, channel permissions | Криптографическая граница явная; replay admission расположен раньше полной проверки сообщения |
| Content/transfer/replication | соответствующие `internal/*` | Object/manifest, private fetch, availability и replica state | Домены разделены, но identity объекта неодинакова между policy и persistence |
| Workload execution | `internal/workload`, `internal/workload/docker` | Desired/observed state, Docker executor, companion ingress proxy | Ограничения контейнера сильные; lifecycle зависит от небounded Docker calls и непрерывности proxy |
| Persistence | `internal/storage` и domain repositories | bbolt/JSON/SQLite с domain-specific schema/open logic | Нет отдельного каталога миграций; миграция встроена в владельцев store |
| Observability | `internal/observability`, daemon HTTP server | Loopback health/metrics, optional bearer token | Отделено от control API; metrics может синхронно зависнуть на runtime/Docker |
| Release/deploy | `scripts`, `deploy`, `.github/workflows` | Build, provenance, native/Docker install, rollout, gates | Самостоятельный слой, но несколько release/rollout invariants не обеспечены |

## Основные сквозные потоки

### Локальная аутентификация и RPC

1. Клиент подключается к Operator или Application Unix socket.
2. Сервер получает peer credentials, выдаёт одноразовый challenge и проверяет root-signed device credential.
3. Сессия привязывается к audience, source/peer и сроку действия; на каждом использовании повторно проверяется revocation.
4. Interceptor сопоставляет процедуру с action, строит server-derived target, проверяет grant и передаёт handler запечатанный `AuthorizedCall`.
5. Handler вызывает доменный сервис и store.

Разделение surface и централизованный admission соответствуют ADR 0001/0002. Исключение — `Object`/`Manifest`: policy target содержит `Owner`, а lookup/overwrite в content domain фактически адресуется только по `ID` (SEC-001).

### Приватное сообщение

1. Отправитель сериализует payload, подписывает canonical envelope и шифрует его channel secret.
2. Waku доставляет ciphertext.
3. Получатель проверяет AEAD и durable replay ledger, затем декодирует envelope, проверяет подпись, класс сообщения и текущие права.
4. Сообщение передаётся подписчику либо запускает transfer/discovery path.

Порядок шага 3 создаёт возможность заполнить replay ledger сообщениями, которые никогда не пройдут подпись/authorization (SEC-003).

### Workload и ingress

1. Workload desired state валидируется и сохраняется.
2. Docker adapter создаёт ограниченный backing container.
3. Для опубликованного ingress создаётся отдельный proxy container.
4. Proxy принимает соединение и перенаправляет его на workload.
5. Runtime reconciles observed state, publication и ancillary containers.

Смерть proxy после отдельной ошибки соединения не обнаруживается обычным `RefreshObserved`; а общий лимит без idle deadline позволяет исчерпать все слоты (REL-001/REL-002).

### Upgrade/release

1. CI запускает static/test gates.
2. Release build создаёт binaries/images и metadata/provenance.
3. Rollout выполняет backup, последовательно пересоздаёт узлы, проверяет readiness и при ошибке компенсирует изменённые узлы.

Фактический путь нарушен отсутствующим именем backup-скрипта, неполным compensation set и слишком узким readiness-сигналом (OPS-001—OPS-004).

## Заявленная архитектура

Основной нормативный документ `docs/engineering/codebase-architecture.md` задаёт:

- модульный монолит и один явный composition root;
- изоляцию concrete Waku и Docker adapters;
- доменное владение правилами и narrow interfaces только на реальных seams;
- два раздельных Principal-authenticated local API;
- отделение generated contracts от handwritten production code;
- исчерпывающее target tree;
- лимит не более 12 handwritten production-файлов на пакет и service/RPC boundary;
- package documentation для каждого `internal`-пакета;
- отсутствие legacy-путей и tracked tooling вне разрешённого дерева.

ADR 0001 и 0002 закрепляют разделение application/operator interface и Principal-centered identity/access. Продуктовые и operational документы уточняют canonical identities, capabilities, network privacy, workload policy и deployment semantics.

## Расхождения заявленного и фактического

1. **Structural acceptance устарел.** Вопреки лимиту из `docs/engineering/codebase-architecture.md:722,1009,1049-1050`, `internal/identity/access` содержит 24 handwritten production-файла, `internal/daemon` — 16; ещё шесть пакетов содержат по 13.
2. **Package documentation неполна.** Требование `docs/engineering/codebase-architecture.md:1061` не выполнено для handwritten `internal/identity`, `internal/localapi`, `internal/localapi/network`, `internal/provision`.
3. **Generated boundary частично смешана.** `internal/messaging/private.proto` и `private.pb.go` находятся внутри handwritten domain package, хотя заявлен отдельный generated-контур.
4. **Service count устарел.** Документ указывает восемь bounded generated services (`docs/engineering/codebase-architecture.md:1014`), тогда как фактически определены девять protobuf service, включая отдельно смонтированный `IdentityService`.
5. **Target tree не отражает tracked audit tooling.** Документ запрещает tracked `.agents` (`docs/engineering/codebase-architecture.md:346`), но в commit находится 11 файлов `.agents`.
6. **Principal migration завершена в коде, но не в документации.** README и часть protocol/product/operations документов всё ещё описывают loopback token API, тогда как реализация и новые документы используют Unix Principal sessions.
7. **Release reproducibility заявлена сильнее, чем обеспечена.** Двойная сборка использует общий cache и mutable base tags; metadata не фиксирует base image digests.

Эти расхождения объединены в ARCH-001, DOC-001 и SUP-001; это не семь независимых дефектов.

## Матрица архитектурных решений

| Decision | Evidence | Status | Consistency | Documentation | Recommendation |
|----------|----------|--------|-------------|---------------|----------------|
| Модульный монолит с composition root в daemon | `cmd/ardentsd`, `internal/daemon/run.go`, `internal/daemon/configuration.go` | явно принято | Реализация последовательна | Подробно описано | Сохранить; не выносить домены в сервисы без отдельного operational основания |
| Раздельные Operator и Application API | `docs/adr/0001-*`, `internal/localapi`, `internal/applicationapi` | явно принято | Последовательно в коде | Новые документы согласованы, старые entry docs нет | Удалить legacy token/loopback narrative |
| Principal-centered identity вместо shared API token | `docs/adr/0002-*`, `internal/identity/access`, API interceptors | явно принято | Реализация в целом едина | Конфликт старых и новых документов | Сделать Principal contract единственным нормативным источником |
| Централизованный sealed authorization | `internal/localapi/identity/operator_interceptor.go`, `internal/applicationapi/binding` | явно принято | Большинство handlers соблюдает; content owner target теряется | Описано как invariant | Исправить identity модели Object/Manifest и добавить invariant tests |
| Concrete Waku/libp2p только в network adapter | `tests/tooling/importguard`, `internal/network/waku` | явно принято | Подтверждено importguard | Описано | Сохранить; добавить bounded-store requirement в adapter contract |
| Concrete Docker/Moby только в workload adapter | `tests/tooling/importguard`, `internal/workload/docker` | явно принято | Подтверждено importguard | Описано | Сохранить; перенести timeout/cancellation policy в adapter boundary |
| Generated contracts отделены от handwritten code | `api/ardents`, `scripts/generate-api.ps1` | частично внедрено | `internal/messaging/private.pb.go` — исключение | Требование явно записано | Либо оформить исключение ADR, либо переместить private contract |
| Не более 12 handwritten production-файлов на пакет | `docs/engineering/codebase-architecture.md:722,1009,1049-1050` | частично внедрено | Семь пакетов превышают budget | Acceptance формально заявлен выполненным | Пересмотреть budget/исключения и затем синхронизировать acceptance |
| Package docs для всех internal packages | `docs/engineering/codebase-architecture.md:1061` | частично внедрено | Четыре handwritten package без package comment | Заявлено как acceptance | Добавить package contracts либо ослабить формальное требование |
| Domain-owned persistence с embedded migrations | domain repositories, `internal/storage` | неявно следует из кода | Последовательно, отдельного migration layer нет | Рассеяно по protocol/security docs | Зафиксировать ownership/versioning policy в ADR |
| Waku full/service node хранит relay traffic | `internal/network/waku/messaging.go`, `service.go` | неявно следует из кода | Store включён без retention policy | Retention contract не найден | Явно решить retention/quotas и угрозу untrusted storage |
| Companion container для ingress | `internal/workload/docker/docker_proxy.go`, `cmd/ardents-ingress-proxy` | явно принято | Creation есть; continuous supervision неполна | Описано в workload docs | Добавить lifecycle/restart/idle policy как обязательный контракт |
| Loopback observability отделена от control API | `internal/observability`, daemon server setup | явно принято | Последовательно в runtime | Хорошо описано | Сохранить; исключить unbounded runtime calls из scrape path |
| Rolling upgrade с автоматической компенсацией | `scripts/deploy/rollout.ps1`, deployment docs | частично внедрено | Backup, readiness и rollback расходятся с контрактом | Контракт сильнее реализации | Исправлять как один transactional rollout design |
| Reproducible, attributable release | `.github/workflows/ci.yml`, `scripts/release/*` | конфликтует с другим решением | Hash compare есть, immutable builders/base provenance нет | Гарантия завышена | Pin builders/base digests и проверять source SHA/worktree |
| Tracked дерево ограничено product/build/docs областями | `docs/engineering/codebase-architecture.md:346`, `.agents` | устарело | Текущий tree нарушает старое правило | Документ не обновлён | Явно разрешить repository-local agent tooling или убрать его |

## Итог

Границы доменов и инфраструктурных адаптеров в целом хорошо выражены и машинно проверяются. Наиболее опасные отклонения лежат не в циклических зависимостях, а на стыках разных идентичностей и жизненных циклов: policy target против persistence key, RPC stream против process shutdown, runtime против внешнего Docker engine, Waku transport против local retention, rollout contract против скриптов. Именно эти seams должны определять порядок стабилизации.
