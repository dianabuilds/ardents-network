# 03. Покрытие аудита

## Назначение

Этот файл фиксирует фактическую глубину статического технического и security-аудита на коммите `52af3b2480b62da60ae82c7f1d43f45cd5778230`. Он не является заявлением об отсутствии дефектов: статус отражает выполненную работу, а не качество подсистемы.

Исходный код и внешние системы не изменялись. Помимо статического чтения выполнялись безопасные локальные Go-проверки и один disposable Docker integration run; они создали только игнорируемое test evidence. Разрешённые audit-документы в `docs/audit/2026-07-23/` создавались после фиксации точки аудита и не входят в проверяемый baseline.

## Значение статусов

| Статус | Значение |
|---|---|
| `checked` | Существенные security/reliability потоки участка прослежены от внешней границы до эффекта; критичные инварианты и связанные тесты/документы сопоставлены. |
| `partial` | Выполнена структурная или целевая проверка, но не весь участок прочитан построчно либо не проверена runtime-часть. |
| `excluded` | Область осознанно исключена из line-by-line аудита; причина и способ учёта указаны явно. |
| `not checked` | Для проверки требовалась внешняя среда, запуск или доступ, которых в этой ветви аудита не было. |

Глубина:

- `deep` — трассировка данных, полномочий, ошибок и lifecycle между несколькими слоями;
- `targeted` — чтение путей, связанных с конкретными границами риска;
- `structural` — инвентаризация, contracts/wiring и точечная сверка;
- `none` — содержательная проверка не выполнялась.

## Матрица покрытия

| № | Каталог / подсистема | Статус | Глубина | Что проверено | Непокрытая часть / причина |
|---:|---|---|---|---|---|
| 1 | Корень: `README.md`, `CONTEXT.md`, `AGENTS.md`, `go.mod`, `go.sum`, `ardents.ps1` | `checked` | `targeted` | Точка аудита, заявленная архитектура, module graph, правила работы, launcher и связи с поставкой | Changelog и лицензия не анализировались как исполняемая логика |
| 2 | `.github/workflows` | `checked` | `deep` | Единственный CI workflow, зависимости jobs, вызываемые scripts/tools и release-gates | Реальный GitHub runner не запускался |
| 3 | `.agents/`, `skills-lock.json` | `excluded` | `none` | Учтены в файловом инвентаре; применимые repository instructions прочитаны отдельно | Agent workflow не является поставляемым runtime-продуктом |
| 4 | `api/ardents/identity/v1` — proto-контракт | `checked` | `deep` | Identity services/messages, auth-related поля и соответствие handler слоям | Совместимость с внешними независимо реализованными клиентами не испытывалась |
| 5 | `api/ardents/identity/v1` — сгенерированный Go/Connect код | `partial` | `structural` | Наличие, package topology и связь с `.proto` | Не выполнялся line-by-line аудит генератора и generated boilerplate |
| 6 | `cmd/ardentsd` | `checked` | `deep` | Композиция daemon, конфигурация, startup/bootstrap, серверы, signal/drain/shutdown; daemon запускался в integration harness | Не выполнялся отдельный native/systemd signal test |
| 7 | `cmd/ardentsctl` | `checked` | `targeted` | Entry wiring, transport/client construction, session-oriented команды и output boundaries | Не каждый UX/TUI путь выполнен интерактивно |
| 8 | `cmd/ardents-ingress-proxy` | `checked` | `deep` | Entry wiring, параметры и передача управления proxy implementation | Бинарник не запускался под сетевой нагрузкой |
| 9 | `deploy/` | `checked` | `deep` | Dockerfiles, Compose, systemd unit, native/container runtime assumptions и readiness wiring | Release images/native systemd не запускались; disposable integration image был пересобран |
| 10 | `docs/` | `partial` | `targeted` | Архитектурные, product, operations, distribution и security утверждения сопоставлялись с кодом | Не все 42 baseline-документа проверены с одинаковой глубиной |
| 11 | `internal/applicationapi` — admission/binding/call/content | `checked` | `deep` | Граница Application API, sealed call context, principal/binding и ownership paths | End-to-end workload client не запускался |
| 12 | `internal/applicationapi/protocol` | `partial` | `structural` | Контракты и связь generated handlers с рукописной реализацией | Generated boilerplate не проверялся построчно |
| 13 | `internal/buildinfo` | `checked` | `targeted` | Источник build metadata и использование в release/diagnostics | Reproducibility фактического бинарника не измерялась |
| 14 | `internal/cli/client`, `internal/cli/identity` | `checked` | `deep` | Session login, challenge/response validation, local/SSH transport и обработка ответов | Нет интерактивного/удалённого прогона |
| 15 | Остальной `internal/cli` | `partial` | `targeted` | Command topology, configuration, output, node/content/network/workload call sites | Не каждая команда и TUI state transition прослежена полностью |
| 16 | `internal/config` | `checked` | `deep` | Загрузка, defaults, строгая validation, privacy/replay paths и runtime wiring | Матрица всех ошибочных конфигураций не исполнялась |
| 17 | `internal/content` | `checked` | `deep` | Object/manifest/blob/retention модель, ID/owner semantics, catalog/payload persistence | Производительность и большие payload не испытывались |
| 18 | `internal/daemon` | `checked` | `deep` | Composition root, bootstrap handoff, read model, workload/network/server lifecycle, drain/shutdown | Нет live startup/shutdown и fault injection |
| 19 | `internal/diagnostics` | `partial` | `targeted` | Health/event/operation contracts и вызывающие lifecycle-пути | Не все producers/consumers и retention scenarios прочитаны построчно |
| 20 | `internal/discovery` | `partial` | `targeted` | Records, resolution, trust boundary и сетевые call paths | Нет живого peer discovery и исчерпывающей проверки всех record variants |
| 21 | `internal/hosting` | `partial` | `structural` | Роль пакета и связи с workload/daemon | Малый участок не проходил отдельную глубокую data-flow трассировку |
| 22 | `internal/identity` — access/session/grant/recovery/tickets | `checked` | `deep` | Credentials, challenges, sessions, grants/revocations, recovery guard, bootstrap/application tickets, audit outbox и delegation repository | Криптопримитивы third-party библиотек не переаудировались |
| 23 | `internal/identity` — principal/capability/keyring/trust | `checked` | `deep` | Principal derivation/parsing, capability validation/store/delivery, sender authorization, HPKE attestation, key/trust boundaries | Нет межреализационных crypto test vectors вне репозитория |
| 24 | `internal/ingressproxy` | `checked` | `deep` | Accept/copy/error lifecycle, connection semaphore, адреса и shutdown behavior | Нет soak/load/slow-client прогона |
| 25 | `internal/localapi/auth`, `internal/localapi/rpc` | `checked` | `deep` | Operator authorization interceptor, sealed AuthorizedCall, canonical target extraction/finalization и scope enforcement | Unix-socket peer behavior не проверялся на живой ОС |
| 26 | `internal/localapi/identity` | `checked` | `deep` | Enrollment, session, grant/revocation, recovery и bootstrap RPC paths | Нет live multi-client concurrency run |
| 27 | `internal/localapi/content` | `checked` | `deep` | Object/manifest/blob handlers, retention, owner/resource mapping и persistence effects | Большие/повреждённые blobs не прогонялись |
| 28 | `internal/localapi/node` | `checked` | `deep` | Event streaming, deadlines, cancellation и shutdown interaction | Реальный streaming client не запускался |
| 29 | Остальной `internal/localapi` | `partial` | `targeted` | Configuration, diagnostics, network, transfer и workload handlers проверялись по security-relevant call paths | Не каждый RPC и ошибка прослежены end-to-end |
| 30 | `internal/localapi/protocol` | `partial` | `structural` | Proto service topology и соответствие generated registration рукописным handlers | Generated файлы исключены из line-by-line чтения |
| 31 | `internal/messaging` | `partial` | `deep` | Signing/encryption envelopes, sender checks, privacy/replay path wiring, store-forward time semantics | Не весь пакет прочитан построчно; live Waku delivery и adversarial corpus не запускались |
| 32 | `internal/network` | `partial` | `targeted` | Peer/routing/Waku adapter, trust crossings, storage/config composition и lifecycle limits | Реальная P2P сеть, NAT, churn и hostile peers не проверялись |
| 33 | `internal/observability` | `partial` | `targeted` | Loopback exposure, server lifecycle, timeouts и ключевые metric paths | Полнота/кардинальность каждой метрики не оценивалась нагрузочно |
| 34 | `internal/policy` | `partial` | `targeted` | Policy model и enforcement call sites на проверенных API путях | Нет формальной полноты всех action/resource combinations |
| 35 | `internal/provision` | `partial` | `structural` | Назначение, wiring и вызывающие daemon/workload paths | Не выполнена отдельная глубокая проверка каждого provision transition |
| 36 | `internal/publication` | `partial` | `targeted` | Publication flow, identity/network/content boundaries | Нет live publish/receive path и полного adversarial input review |
| 37 | `internal/replication` | `partial` | `targeted` | Availability/placement contracts, persistence и content/network call paths | Нет multi-node replication, partition или recovery run |
| 38 | `internal/storage` | `checked` | `deep` | Secure file handling, bbolt-backed storage, ownership/permissions assumptions и startup use | Filesystem race/fault injection и platform matrix не выполнялись |
| 39 | `internal/transfer` | `partial` | `targeted` | Transfer contracts и вызовы из content/local API/network | Нет bandwidth, interruption/resume и malformed-stream tests |
| 40 | `internal/workload/docker`, `internal/workload/execution` | `checked` | `deep` | Docker/process lifecycle, contexts/timeouts, ancillary reconcile, networks/containers и shutdown; 9 Docker executor integration cases прошли | Hung Docker endpoint и долгий lifecycle fault не воспроизводились |
| 41 | `internal/workload/readiness`, `internal/workload/registry` | `partial` | `targeted` | Readiness и registry wiring, адреса, lifecycle consumers | Нет живого registry/readiness matrix |
| 42 | `sdk/go/client`, `sdk/go/identity`, `sdk/go/internal/adapter` | `partial` | `deep` | Public client/session/auth boundaries, response validation и mapping к transport/protocol | Не весь публичный SDK API проверен на usability/compatibility |
| 43 | `sdk/go/protocol` | `partial` | `structural` | Наличие и связь generated protocol packages с контрактами | Generated boilerplate не проверялся построчно |
| 44 | `scripts/` — build/release/verify/publish/deploy/install | `checked` | `deep` | Artifact provenance/metadata, base images, gates, backup/rollback/compensation, native install и readiness | Scripts не выполнялись против registry/host |
| 45 | `scripts/generate-identity-artifact-vectors` | `partial` | `structural` | Package presence, назначение и связи с testdata | Generator не запускался; vector correctness не пересчитывалась независимо |
| 46 | `tests/integration`, `tests/e2e`, package unit tests | `partial` | `targeted` | Unit suite, coverage и выбранный race scope прошли; integration: 43/44, один teardown failure; тесты читались как evidence | E2E не запускался; все 295 test-файлов не читались построчно |
| 47 | `tests/testkit`, `tests/fixtures`, `tests/tooling`, `tests/ci` | `partial` | `structural` | Test topology, helpers/tools и ссылки из CI; importguard прошёл, integration runner исполнен | Fixture payloads не проходили полный content audit; native/multinode/deployment gates не запускались |
| 48 | Все generated protobuf/Connect `.go` | `excluded` | `none` | 28 файлов учтены, происхождение и contract linkage проверены структурно | Generated boilerplate исключён из line-by-line аудита |
| 49 | Third-party module source и transitive dependencies | `excluded` | `none` | `go.mod`/`go.sum` инвентаризированы; `govulncheck` выполнен; целевые Waku persistence paths проверены в pinned dependency source | Полный line-by-line dependency audit, license и container image scan не выполнялись |
| 50 | Игнорируемые runtime/build outputs и caches | `excluded` | `none` | Границы области зафиксированы; подтверждено требование внешнего Go cache | Не являются baseline source; локальные runtime данные не исследовались |
| 51 | `.git/`, IDE metadata, локальная машина вне workspace | `excluded` | `none` | Исключение зафиксировано | Не являются first-party поставляемым кодом |
| 52 | Native systemd/SSH/registry/release/deployment environment | `not checked` | `none` | Не выполнялось | Требует внешнего состояния и потенциально изменяющих действий; local disposable Docker integration учтён в строках 9, 40 и 46 |
| 53 | Live Waku/libp2p сеть и multi-node scenarios | `not checked` | `none` | Не выполнялось | Требует сетевых peers, времени и управляемого adversarial окружения |
| 54 | Frontend и first-party smart contracts | `excluded` | `none` | По tracked tree такие компоненты отсутствуют | Неприменимо; наличие `go-ethereum` как зависимости не создаёт отдельный on-chain codebase |

## Выполненные проверки

Ниже перечислены уже выполненные статические и локальные validation-действия; повторный запуск для составления этого файла не выполнялся.

| Проверка | Результат / назначение |
|---|---|
| `git rev-parse HEAD`, определение ветки и `git status --short` | Зафиксированы baseline commit, ветка и исходное состояние worktree |
| `git ls-files` и группировка по каталогам/расширениям | Получен полный tracked inventory: 883 файла |
| Классификация Go-файлов | 754 Go-файла: 295 `*_test.go`, 318 test/testkit/fixture/tooling, 28 generated protobuf/Connect, 408 handwritten production |
| `go list ./...` | Получена topology 95 first-party packages; команда не запускала тесты |
| `go test ./... -count=1` | PASS; все обычные Go package tests прошли, 82.3 s |
| `go test ./... -covermode=atomic -coverprofile=<external-temp>` | PASS; all-source 44.5%, runtime handwritten 56.7%; profile удалён из external temp |
| `go test -race` для messaging, transfer, ingressproxy, network/waku, identity/access и workload | PASS; `internal/ingressproxy` не содержит test-файлов |
| `./ardents.ps1 test integration -RebuildContainer -ReportDir tests/.artifacts/reports/audit-20260723-integration` | FAIL: 44 total, 43 pass, 1 intermittent TempDir cleanup failure после пройденных assertions; test binaries удалены runner'ом |
| `go vet ./...` | FAIL: copylock в `sdk/go/internal/adapter/enrollment_test.go:140` |
| `go run ./tests/tooling/importguard` | PASS |
| `scripts/generate-api.ps1 -Check`, `tests/check-format.ps1` | На Windows checkout дали CRLF false positive; на canonical LF clone exact SHA обе PASS |
| Canonical CI testcatalog command | FAIL: `tests/cmd/testcatalog` отсутствует; актуальный tool не поддерживает `-mode` |
| `govulncheck ./...` | PASS: 0 vulnerabilities в вызываемом/импортируемом коде; один module-only GO-2026-5932 в `x/crypto/openpgp`, не вызывается |
| `staticcheck`, `deadcode`, `gosec` | Результата нет: установленные локальные версии несовместимы с Go 1.26.5/source syntax; это ограничение среды, не pass |
| Docker cache inventory | Два Ardents volume суммарно 7.88 GiB, ниже policy threshold 8 GiB; очистка не выполнялась |
| `go env GOCACHE GOMODCACHE` | Подтверждено, что Go caches находятся вне репозитория |
| Чтение `go.mod`/`go.sum` | Инвентаризирован dependency surface и версии прямых зависимостей |
| `rg`, `Get-Content`, `Get-ChildItem` и cross-reference чтение | Прослежены call sites, error paths, contexts, authorization scopes, storage paths и lifecycle wiring |
| Сопоставление proto → generated registration → interceptor → handler → repository/effect | Проверены ключевые Operator/Application/Identity API границы |
| Сопоставление исходников с unit/integration tests | Найдены существующие проверки и gaps; результаты единичных unit/race/coverage/integration прогонов использованы как execution evidence |
| Сопоставление кода с архитектурными/operations/release документами | Проверены заявленные trust boundaries, bootstrap, deploy, release и readiness assumptions |
| Статический разбор CI/release/deploy/install scripts | Проверены dependencies, gates, artifact metadata, backup/rollback и native/container installation paths |
| Отбраковка гипотез | Спекулятивные случаи отклонялись, если не было достижимого пути, нарушенного инварианта или достаточной threat-model опоры |

## Глубоко прослеженные сквозные сценарии

1. Enrollment credential → challenge → session → grant/revocation → recovery/bootstrap.
2. Operator socket → auth interceptor → canonical action/resource → handler → domain repository.
3. Application socket → admission/binding → principal-owned content operations.
4. Content object/manifest/blob identifiers → owner semantics → persistence and retention.
5. Signed/encrypted network message → sender/trust validation → replay state → downstream effect.
6. Daemon startup → configuration/storage/bootstrap → API/network/workload servers → drain/shutdown.
7. Workload declaration → Docker/process execution → readiness/registry → ingress proxy → shutdown.
8. Source commit → CI/build metadata → release artifact/image → verification → deploy/install/rollback.

## Ограничения и неисполненные проверки

Не выполнялись:

- E2E, deployment, native-install, multinode и release-candidate suites;
- fuzzing, benchmarks, soak/load и целевой fault-injection;
- live ingress load/RST/idle-client сценарии и hung Docker transport;
- staticcheck/deadcode/gosec с совместимыми с Go 1.26.5 версиями;
- golangci-lint, license и container image scan;
- publish, deploy, rollback и native install против реального host/registry;
- live SSH, systemd, hostile Waku/libp2p или multi-node environment;
- независимая криптографическая верификация third-party primitives.

Выполненные unit/race/integration проверки были единичными, а не статистическими soak-прогонами. Поэтому аудит не подтверждает:

- фактическую проходимость всего CI; напротив, static gate имеет подтверждённые failures;
- platform-specific поведение Windows/Linux sockets, permissions и process signals;
- timing-dependent race/deadlock без статически видимого пути;
- ресурсные пределы под реальной нагрузкой;
- поведение внешних registry, Docker daemon, Waku peers и systemd;
- отсутствие уязвимостей вне охвата `govulncheck` и одного целевого dependency-source review.

## Осознанно проверенные и отклонённые гипотезы

Во избежание завышения результатов отдельно проверялись, но не считались подтверждёнными findings без достаточного evidence:

- обход авторизации через fallback peer identity в handler;
- трактовка same-UID local socket как самостоятельного перехода полномочий;
- возможность произвольно задать время capability-проверки без достижимого production call site;
- обязательность rate limit для публичного revocation import без подтверждённой exploitability;
- общий filesystem symlink TOCTOU без конкретного достижимого пути и подходящей threat model.

## Как читать итоговое покрытие

- Наиболее сильное статическое покрытие: identity/access, локальная авторизация, content ownership, replay/privacy state, daemon lifecycle, ingress/workload Docker paths и supply/release scripts.
- Среднее целевое покрытие: discovery, network, diagnostics, observability, publication, replication, transfer, policy и публичный SDK.
- Наиболее существенный остаточный риск проверки: runtime concurrency под нагрузкой, E2E/multi-node/Waku behavior, native systemd/SSH/registry, полный release pipeline и уязвимости, не покрываемые `govulncheck`.
- Generated код, third-party source и локальные runtime outputs не следует считать глубоко проверенными только потому, что они присутствуют в инвентаре.
