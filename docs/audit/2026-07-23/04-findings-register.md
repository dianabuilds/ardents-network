# Реестр замечаний

| ID | Priority | Category | Subsystem | Finding | Confidence | Status |
|----|----------|----------|-----------|---------|------------|--------|
| SEC-001 | P1 | Security / correctness | Content API и persistence | Owner есть в authorization target, но теряется при Object/Manifest lookup и overwrite | высокий | подтверждено кодом и независимой трассировкой |
| CI-001 | P1 | Quality gate | CI / static | Canonical static job гарантированно падает на `go vet` и устаревшей команде test catalog | высокий | воспроизведено |
| CI-002 | P1 | Quality gate | CI / native install | Gate пишет evidence в несуществующий каталог чистого checkout | высокий | подтверждено кодом |
| IAM-001 | P1 | Reliability / authorization | Identity access | Нормально истёкший retained credential блокирует recovery-path guard и revocation | высокий | подтверждено кодом и независимой трассировкой |
| IAM-002 | P1 | Reliability / provisioning | Identity enrollment | One-time ticket digest коммитится раньше невосстановимой доставки plaintext | высокий | подтверждено кодом и независимой трассировкой |
| REL-001 | P1 | Availability | Ingress proxy | Ошибка одного TCP-соединения завершает весь proxy без постоянного supervision | высокий | подтверждено кодом и независимой трассировкой |
| REL-002 | P1 | Availability | Ingress proxy | 128 idle TCP-соединений бессрочно исчерпывают общий лимит всех портов | высокий | подтверждено кодом и независимой трассировкой |
| SEC-002 | P1 | Security / availability | Private messaging | Subscribe-only participant может заполнить durable replay ledger до проверки подписи/Publish | высокий | подтверждено кодом и независимой трассировкой |
| SEC-003 | P1 | Security / availability | Waku Store | Default service node сохраняет unauthenticated relay traffic без retention/volume bound | высокий | подтверждено кодом и dependency source |
| OPS-001 | P1 | Operations | Compose upgrade | Upgrade вызывает отсутствующий `cluster-data.ps1` и всегда падает до backup | высокий | подтверждено кодом и tree |
| OPS-002 | P1 | Operations | Rolling rollback | Уже пересозданный, но неготовый текущий узел не попадает в compensation set | высокий | подтверждено кодом и независимой трассировкой |
| OPS-003 | P1 | Operations | Rolling acceptance | Rollout принимает новую версию только по network status, игнорируя API/Diagnostics/state | высокий | подтверждено кодом и контрактом |
| REL-003 | P1 | Reliability | Daemon shutdown / Node API | Активный `StreamNodeEvents` блокирует shutdown и обязательный cleanup | высокий | подтверждено кодом и независимой трассировкой |
| REL-004 | P1 | Reliability | Workload runtime / Docker | Background Docker calls без timeout блокируют API, metrics и shutdown | высокий | подтверждено кодом и независимой трассировкой |
| SUP-001 | P1 | Supply chain | Release build | Mutable builders/base images не входят в provenance; double-build делит один cache | высокий | подтверждено кодом и workflow |
| SEC-004 | P1 | Security / availability | Private transfer | Trusted channel peer может racing signed error завершить чужой fetch раньше provider | высокий | подтверждено кодом и независимой трассировкой |
| ARCH-001 | P2 | Architecture debt | Repository structure | Формальный architecture acceptance расходится с file budgets, package docs, generated boundary и tree | высокий | подтверждено инвентарём |
| CLI-001 | P2 | Duplication / correctness | CLI Principal session | CLI использует stale pre-RPC time и расходится с SDK implementation | высокий | подтверждено кодом |
| DOC-001 | P2 | Documentation drift | Access model | Entry/normative docs продолжают описывать удалённый token/loopback control API | высокий | подтверждено сопоставлением |
| OPS-004 | P2 | Operations | Native upgrade/rollback | Readiness жёстко привязана к `127.0.0.1:9090` при настраиваемом loopback address | высокий | подтверждено кодом |
| QLT-001 | P2 | Testability | Lifecycle/infrastructure seams | Нулевое/низкое покрытие совпадает с критичными ingress, Docker и shutdown путями | высокий | подтверждено coverage и кодом |
| SEC-005 | P2 | Reliability / security | Privacy replay persistence | Filesystem-equivalent replay paths создают два stale snapshot над одной записью | средний | подтверждено статической трассировкой; не воспроизведено |
| SUP-002 | P2 | Supply chain / correctness | Manual release | `-Commit` маркирует текущий worktree произвольным другим SHA | высокий | подтверждено кодом; официальный CI-path не затронут |
| TST-001 | P2 | Test reliability | Integration discovery | Assertions проходят, но teardown иногда оставляет файл в `TempDir` | высокий | воспроизведено один раз; ранее проходило |

Сводка: **24 замечания: P0 — 0, P1 — 16, P2 — 8, P3 — 0.** Приоритет отражает влияние, а не трудоёмкость исправления. Ниже замечания отсортированы сначала по приоритету, затем по подсистеме; совпадающие корневые причины объединены.

## P1

### SEC-001 — Owner теряется между authorization target и content store

- **Категория / уверенность / статус:** security и correctness; высокий; подтверждено source trace двумя независимыми проходами.
- **Подсистема и файлы:** `api/ardents/identity/v1/contract.go:174-180`; `internal/localapi/identity/operator_interceptor.go:233-249`; `internal/localapi/content/server_data_objects.go:12-34`; `server_data_manifests.go:12-34`; `internal/content/objects.go:39-61`; `manifests.go:39-71`; `internal/content/catalog/stores.go:99-129`.
- **Фактическое наблюдение и доказательство:** `content-object` и `content-manifest` требуют Owner. Interceptor авторизует `(Node, Effective Owner, Kind, ID)`, но Get игнорирует admitted call и читает глобально по ID; Publish принудительно ставит текущего Owner и store безусловно заменяет запись с тем же ID.
- **Ожидаемый контракт / нарушение:** exact grant должен разрешать только ресурс ровно указанного Owner. Policy identity четырёхкомпонентна, а persistence identity одно-компонентна.
- **Влияние:** Bob с exact grant на `(Bob,X)` может прочитать Object/Manifest Alice с ID `X` и заменить его записью Bob. Blob path использует отдельный owner/reference binding и не затронут.
- **Условия:** два владельца используют одинаковый ID; атакующий имеет допустимый exact grant на свой Owner и этот ID.
- **Корневая причина:** canonical resource identity не проведена через handler, domain aggregate и storage key.
- **Связанные замечания:** ARCH-001, QLT-001.
- **Направление / масштаб / риск исправления:** унифицировать owner-qualified key, lookup и authorization; предусмотреть миграцию существующего каталога. Масштаб большой, риск высокий из-за persisted compatibility.
- **Проверка после исправления:** cross-owner matrix для Get/Publish Object и Manifest; migration test с collision ID; interceptor/store invariant test. Существующий B4 тест sibling ID этого случая не покрывает.

### CI-001 — Canonical static job не может пройти

- **Категория / уверенность / статус:** quality gate; высокий; обе причины воспроизведены.
- **Подсистема и файлы:** `.github/workflows/ci.yml:63-71`; `sdk/go/internal/adapter/enrollment_test.go:140`; `tests/tooling/testcatalog/main.go:18-26`.
- **Фактическое наблюдение и доказательство:** canonical LF checkout проходит generation/format, затем `go vet ./...` падает: assignment копирует `testEnrollmentSigner`, содержащий `atomic.Int32/noCopy`. Даже после этого workflow вызывает отсутствующий `./tests/cmd/testcatalog` с удалённым `-mode`; актуальный tool находится в `tests/tooling/testcatalog` и такого flag не имеет.
- **Ожидаемый контракт / нарушение:** static job должен быть воспроизводимым gate для всех downstream suites. Здесь две последовательные гарантированные ошибки.
- **Влияние:** jobs `fast`, `tagged`, `security`, `deployment`, `native-install`, `multinode` и release candidate, зависящие от `static`, блокируются.
- **Условия:** любой чистый CI run текущего SHA.
- **Корневая причина:** workflow не обновлён одновременно с testcatalog relocation/CLI change, а test helper нарушает vet copylock invariant.
- **Связанные замечания:** CI-002, QLT-001.
- **Направление / масштаб / риск исправления:** исправить helper и canonical catalog invocation, затем зафиксировать CLI contract самого tool. Масштаб малый, риск низкий.
- **Проверка после исправления:** один clean canonical static job на LF checkout; отдельно отрицательный catalog fixture, чтобы gate доказывал validation, а не только exit 0.

### CI-002 — Native-install gate не создаёт evidence directory

- **Категория / уверенность / статус:** quality gate; высокий; подтверждено source/workflow trace.
- **Подсистема и файлы:** `tests/ci/native-install-gate.ps1:1-8,52-54`; `.gitignore:3`; `.github/workflows/ci.yml:176-192,211-214`.
- **Фактическое наблюдение и доказательство:** script вычисляет `$reportPath`, но не создаёт его; после успешного systemd smoke вызывает `WriteAllText(.../passed.txt)`. `.artifacts` ignored и отсутствует в fresh checkout.
- **Ожидаемый контракт / нарушение:** gate обязан самостоятельно создать output path, объявленный параметром/default.
- **Влияние:** clean CI падает после фактически успешного acceptance и блокирует release dependency; evidence upload также не находит файл.
- **Условия:** fresh runner без заранее созданного `tests/.artifacts/native-install`.
- **Корневая причина:** script не владеет lifecycle своего report directory.
- **Связанные замечания:** CI-001, OPS-004.
- **Направление / масштаб / риск исправления:** idempotent create directory до smoke/write. Масштаб малый, риск низкий.
- **Проверка после исправления:** запуск gate в clean checkout с отсутствующим parent и проверка `passed.txt`/upload.

### IAM-001 — Истёкший retained credential блокирует revocation

- **Категория / уверенность / статус:** reliability/authorization; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/identity/access/credential_repository.go:32-55`; `administration_revoke.go:86-109,164-196,215-238,264-293`; `administration.go:326-330`.
- **Фактическое наблюдение и доказательство:** credentials сохраняются без удаления. `subjectHasRecoveryDevice` разбирает каждый retained record с текущим `now`; нормальная expiry возвращается как corrupt record и прерывает scan до действующего recovery credential.
- **Ожидаемый контракт / нарушение:** исторически истёкшая запись не должна делать recovery-state недоступным; guard должен оценивать действующие credentials и fail-closed только на реально неоднозначной/повреждённой записи.
- **Влияние:** блокируются все device revocations и revoke активного recovery grant; ошибка наружу становится `ErrUnavailable`. Произвольный non-recovery grant revoke не затронут.
- **Условия:** у subject сохранён истёкший credential и есть другой действующий recovery device/grant.
- **Корневая причина:** repository loader смешивает integrity verification с current-time eligibility, а scan не классифицирует expiry как ожидаемое состояние.
- **Связанные замечания:** IAM-002.
- **Направление / масштаб / риск исправления:** разделить cryptographic integrity parse и temporal eligibility; определить retention/compaction. Масштаб средний, риск средний для recovery semantics.
- **Проверка после исправления:** тест «expired retained + valid recovery» для всех device revocations и revoke активного recovery grant; corrupt-record negative case должен остаться fail-closed.

### IAM-002 — One-time ticket становится недоставляемым после commit

- **Категория / уверенность / статус:** reliability/provisioning; высокий; подтверждено двумя source traces.
- **Подсистема и файлы:** `internal/identity/access/bootstrap_ticket.go:48-80`; `internal/daemon/run.go:160-177`; `internal/identity/access/application_enrollment.go:127-167`; `internal/cli/identity/administration.go:144-167`.
- **Фактическое наблюдение и доказательство:** bootstrap digest записывается до записи plaintext-файла; при file failure повторный start трактует `ErrConflict` как успех. Application digest/outbox коммитятся до audit flush/RPC delivery, а CLI сохраняет plaintext ещё позже.
- **Ожидаемый контракт / нарушение:** one-time authority должна либо быть доставлена долговечно, либо безопасно перевыпускаема/подтверждаема; commit и delivery должны иметь recoverable handoff.
- **Влияние:** первоначальное provisioning или application enrollment теряет authority на 10 минут; bootstrap после expiry требует ещё одного restart для выпуска нового ticket.
- **Условия:** ошибка файла, audit flush, transport/response или client file create после DB commit.
- **Корневая причина:** digest — durable state, plaintext delivery — необратимый side effect без acknowledgement/recovery state machine.
- **Связанные замечания:** IAM-001, REL-003.
- **Направление / масштаб / риск исправления:** проектировать transactional handoff/acknowledgement или recoverable reissue с строгим single-use invariant. Масштаб большой, риск высокий.
- **Проверка после исправления:** failure injection после каждого commit boundary; restart/retry matrix; доказательство отсутствия второго одновременно действующего ticket.

### REL-001 — Ошибка одного соединения завершает ingress proxy

- **Категория / уверенность / статус:** availability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/ingressproxy/proxy.go:50-58,105-117,127-157`; `cmd/ardents-ingress-proxy/main.go:27-29`; `internal/workload/docker/docker_proxy.go:84-98`; `docker_ancillary.go:41-85`; `reconciler_observe.go:30-67`.
- **Фактическое наблюдение и доказательство:** `io.Copy`/`CloseWrite` error одного connection отправляется в общий event channel; `Run` возвращает первую ошибку, main завершает process. Container не имеет restart policy. Ancillary reconcile есть на startup/reload, но обычный observed refresh видит backing container, не умерший proxy.
- **Ожидаемый контракт / нарушение:** per-connection transport failure должен завершать только connection; published ingress требует continuous supervision.
- **Влияние:** удалённый клиент, способный вызвать reset/copy error, выключает весь ingress workload до внешнего restart/reload.
- **Условия:** доступ к опубликованному TCP-порту и connection-level error, пока daemon context активен.
- **Корневая причина:** connection errors ошибочно классифицированы как process-fatal, а proxy lifecycle не наблюдается непрерывно.
- **Связанные замечания:** REL-002, QLT-001.
- **Направление / масштаб / риск исправления:** локализовать errors, добавить rate-limited diagnostics и explicit restart/supervision contract. Масштаб средний, риск средний.
- **Проверка после исправления:** RST/half-close test доказывает, что другие connections и listener остаются живы; kill proxy test доказывает автоматическое восстановление.

### REL-002 — Idle connections исчерпывают общий ingress budget

- **Категория / уверенность / статус:** availability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/ingressproxy/proxy.go:16-18,39,46-48,105-147`.
- **Фактическое наблюдение и доказательство:** один semaphore ёмкостью 128 разделён всеми портами. После 5-секундного outbound dial у established connections нет read/write/idle deadline; 129-е соединение немедленно отклоняется.
- **Ожидаемый контракт / нарушение:** публичный proxy должен ограничивать время/ресурсы неактивного клиента и не позволять одному порту исчерпать всю публикацию.
- **Влияние:** 128 молчащих клиентов блокируют все опубликованные порты контейнера на неограниченное время.
- **Условия:** доступ к любому ingress-порту и возможность держать TCP sockets открытыми.
- **Корневая причина:** глобальный concurrency limit не дополнен idle policy, per-source/per-port quotas или pressure eviction.
- **Связанные замечания:** REL-001, QLT-001.
- **Направление / масштаб / риск исправления:** определить protocol-appropriate idle/read/write deadlines и fair admission. Масштаб средний, риск средний из-за long-lived legitimate protocols.
- **Проверка после исправления:** 128 idle clients плюс рабочий клиент на другом порту; проверка освобождения слотов и отсутствия premature timeout для разрешённых long-lived flows.

### SEC-002 — Replay ledger заполняется до проверки подписи и Publish

- **Категория / уверенность / статус:** security/availability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/messaging/open.go:14-30,33-57,81-105`; `header.go:9-16,81-93`; `replay.go:73-101`; `internal/daemon/configuration.go:685`.
- **Фактическое наблюдение и доказательство:** receive order — AEAD open, durable replay admit, затем protobuf decode/signature/class/Publish authorization. Outer expiry допускает 30 дней; ledger fail-closed при 4096 entries/channel.
- **Ожидаемый контракт / нарушение:** долговременный replay budget должен расходоваться только сообщениями, которые прошли authenticity и semantic authorization, либо иметь отдельный bounded quarantine.
- **Влияние:** subscribe-only holder channel secret публикует 4096 AEAD-valid, но malformed/unsigned envelopes и блокирует легитимный channel до 30 дней; можно заранее занять известный MessageID.
- **Условия:** secret-bearing participant с Subscribe и возможностью raw Waku publish; посторонний без channel secret не подходит.
- **Корневая причина:** anti-replay state mutation расположена до полной аутентификации inner envelope.
- **Связанные замечания:** SEC-005, SEC-003.
- **Направление / масштаб / риск исправления:** пересмотреть protocol ordering и atomic admit-after-auth; сохранить защиту от concurrent duplicate delivery. Масштаб средний, риск высокий для protocol compatibility/concurrency.
- **Проверка после исправления:** capacity test с invalid signature/Publish, concurrent duplicate test, restart persistence и 30-day outer header case.

### SEC-003 — Waku Store не имеет retention bound

- **Категория / уверенность / статус:** security/availability; высокий; подтверждено first-party code и pinned dependency source.
- **Подсистема и файлы:** `internal/config/defaults.go:12-25`; `internal/network/waku/messaging.go:44-79`; `service.go:265-279`; `go.mod:20`; go-waku v0.10.3 `waku/persistence/store.go:115-145,210-237` и `waku/v2/protocol/legacy_store/waku_store_protocol.go:141-169`.
- **Фактическое наблюдение и доказательство:** default `service_node` создаёт SQLite message provider и включает Relay/Store. Ardents не передаёт `WithRetentionPolicy`; dependency defaults оставляют max messages/duration равными нулю, а cleanup тогда ничего не удаляет.
- **Ожидаемый контракт / нарушение:** unauthenticated network ingress, сохраняемый до Ardents-level auth, обязан иметь quota/retention и disk-pressure behavior.
- **Влияние:** любой reachable Waku peer непрерывно публикует non-ephemeral frames на joined topic и растит `waku-store.db` до исчерпания диска.
- **Условия:** service/full profile и сетевой доступ к Waku; constrained/restricted profile provider не создаёт.
- **Корневая причина:** adapter включает optional upstream persistence без явной retention configuration.
- **Связанные замечания:** SEC-002, QLT-001.
- **Направление / масштаб / риск исправления:** ввести обязательные size/time quotas, передать retention policy, определить disk-full degradation/metrics. Масштаб средний, риск средний для Store semantics.
- **Проверка после исправления:** adversarial growth test с маленькой quota, restart cleanup, metrics/readiness при pressure и compatibility Store queries.

### OPS-001 — Compose upgrade ссылается на отсутствующий backup script

- **Категория / уверенность / статус:** operations; высокий; подтверждено tree и source.
- **Подсистема и файлы:** `scripts/deploy/rollout.ps1:107-110`; `scripts/deploy/data.ps1`; `scripts/release/bundle.sh:22`; `ardents.ps1:78-82`.
- **Фактическое наблюдение и доказательство:** `New-UpgradeBackups` вызывает `cluster-data.ps1`, которого нет; release bundle включает `data.ps1`.
- **Ожидаемый контракт / нарушение:** официальный `ardents.ps1 upgrade` должен сделать verified backup до первого recreate.
- **Влияние:** любой Compose upgrade текущего release завершается до backup и rollout. Fail-closed сохраняет узлы, но supported upgrade path отсутствует.
- **Условия:** любое выполнение `upgrade`.
- **Корневая причина:** rename/move deployment helper не был атомарно отражён в caller и gate.
- **Связанные замечания:** OPS-002, OPS-003, CI-001.
- **Направление / масштаб / риск исправления:** исправить reference и добавить bundle-level upgrade smoke. Масштаб малый, риск низкий.
- **Проверка после исправления:** clean release bundle, backup evidence для каждого service, затем injected pre-recreate failure без изменения узлов.

### OPS-002 — Failed current node не откатывается

- **Категория / уверенность / статус:** operations/reliability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `scripts/deploy/rollout.ps1:65-68,85-103`; `docs/operations/deployment-contract.md:94-98`.
- **Фактическое наблюдение и доказательство:** service сначала force-recreate и проверяется; в `$changed` он добавляется только после readiness. Если новая версия запустилась, но readiness упала, catch откатывает лишь предыдущие services.
- **Ожидаемый контракт / нарушение:** compensation set должен включать каждый service с начатой необратимой mutation.
- **Влияние:** текущий узел остаётся на failed target image, а прежние откатываются; кластер становится mixed-version/offline вопреки сообщению об автоматическом rollback.
- **Условия:** recreate успешен, readiness текущего service не достигнута.
- **Корневая причина:** rollback journal фиксируется после commit criterion, а не до mutation.
- **Связанные замечания:** OPS-001, OPS-003.
- **Направление / масштаб / риск исправления:** transaction journal с состояниями before/mutated/accepted/compensated и rollback текущего узла. Масштаб средний, риск высокий для upgrade safety.
- **Проверка после исправления:** failure injection после каждого recreate и на каждом readiness step; итоговые image digests всех узлов должны совпадать с fallback.

### OPS-003 — Rollout acceptance проверяет только network status

- **Категория / уверенность / статус:** operations; высокий; подтверждено code/contract comparison.
- **Подсистема и файлы:** `scripts/deploy/rollout.ps1:43-54`; `docs/operations/upgrade-migration.md:38-48`; `deployment-contract.md:94-98`.
- **Фактическое наблюдение и доказательство:** seed принимается при `network.state == ready`, остальные при `network.joined == true`. Документ требует protected local API, canonical network, Diagnostics и retained Principal/grant state.
- **Ожидаемый контракт / нарушение:** readiness после schema/binary change должна доказывать все критичные retained/control-plane invariants.
- **Влияние:** rollout продолжает обновлять кластер и записывает новый manifest при потерянном identity state, недоступном API или degraded Diagnostics.
- **Условия:** network healthy, одна из остальных подсистем не восстановлена.
- **Корневая причина:** deployment script использует один удобный сигнал вместо composite acceptance contract.
- **Связанные замечания:** OPS-001, OPS-002, IAM-002.
- **Направление / масштаб / риск исправления:** versioned machine-readable upgrade probe/command с identity retention assertions. Масштаб средний, риск средний.
- **Проверка после исправления:** fault matrix network/API/Diagnostics/state; каждый дефект должен остановить rollout и запустить корректную компенсацию.

### REL-003 — StreamNodeEvents блокирует process shutdown

- **Категория / уверенность / статус:** reliability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/daemon/api.go:234-256`; `internal/localapi/node/queries.go:36-45`; `internal/daemon/events.go:35-49`; `internal/daemon/run.go:79-108,338-372`; `deploy/systemd/ardentsd.service:15`.
- **Фактическое наблюдение и доказательство:** streaming procedure исключена из TimeoutHandler/deadline и ждёт request context. Server не получает signal-bound `BaseContext`; signal path вызывает `Shutdown(context.Background())` и ждёт stream до deferred `Node.Stop`.
- **Ожидаемый контракт / нарушение:** process shutdown должен отменить long-lived streams и иметь bounded drain до state/network/workload cleanup.
- **Влияние:** любой авторизованный read-only watcher удерживает SIGTERM; systemd через 30 секунд делает SIGKILL, поэтому workload stop, publication withdrawal и transport cleanup не выполняются.
- **Условия:** активный stream в момент signal; клиент не закрывает соединение.
- **Корневая причина:** request lifecycle не связан с process lifecycle, а shutdown использует unbounded context до domain stop.
- **Связанные замечания:** REL-004, IAM-002.
- **Направление / масштаб / риск исправления:** server BaseContext/cancellation tree, bounded Shutdown и заранее определённый cleanup order. Масштаб средний, риск средний для streaming semantics.
- **Проверка после исправления:** активный stream + SIGTERM; процесс выходит до budget, stream получает cancellation, publication/workloads/transport подтверждённо остановлены.

### REL-004 — Docker stall блокирует API, metrics и shutdown

- **Категория / уверенность / статус:** reliability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/workload/docker/docker_executor.go:43-74`; `docker_inspect.go:12-41`; `internal/workload/runtime.go:66-90,192-205`; `internal/localapi/workload/queries.go:12-29`; `internal/observability/collector.go:66-76`; `internal/daemon/read_model.go:358-366,442-449`; `run.go:198-200`; `runtime.go:322-336`.
- **Фактическое наблюдение и доказательство:** Docker client не имеет overall timeout; List/Get заменяют RPC context на `context.Background()` под Runtime mutex. Metrics/read-model вызывают observed sync, shutdown также идёт с Background.
- **Ожидаемый контракт / нарушение:** внешний control plane должен иметь bounded calls, propagate cancellation и не удерживать shared state lock во время unbounded I/O.
- **Влияние:** зависший Docker endpoint блокирует workload queries/mutations, Prometheus scrape/read model и остановку daemon; один call может удержать mutex бесконечно.
- **Условия:** Docker engine/socket принимает call, но не отвечает; startup inventory частично смягчён отдельным 30s timeout, не все paths затронуты.
- **Корневая причина:** cancellation policy потеряна на domain/adapter boundary и сетевой I/O выполняется под coarse mutex.
- **Связанные замечания:** REL-003, QLT-001.
- **Направление / масштаб / риск исправления:** bounded child contexts, configurable client transport timeout, snapshot/lock split и degraded cached observation. Масштаб большой, риск высокий для concurrency/state consistency.
- **Проверка после исправления:** fake/hung Docker transport для List/Inspect/Stop; API/metrics отвечают bounded degraded result, shutdown завершается, race suite остаётся чистым.

### SUP-001 — Release provenance не фиксирует mutable builders

- **Категория / уверенность / статус:** supply-chain assurance; высокий; подтверждено release scripts/workflow.
- **Подсистема и файлы:** `scripts/release/build.ps1:39-49,54-73,83-96`; `scripts/release/metadata.ps1:87-93`; `deploy/docker/images/*.Dockerfile`; `.github/workflows/ci.yml:223-233`.
- **Фактическое наблюдение и доказательство:** builders/base images заданы mutable tags (`golang:1.26-bookworm`, `debian:bookworm-slim`, PowerShell tag), BuildKit provenance выключена. Две сборки идут подряд на одном runner/cache; metadata не записывает base digests.
- **Ожидаемый контракт / нарушение:** reproducible/attributable release должен фиксировать все executable build inputs и независимо сравнивать результаты.
- **Влияние:** перемещённый или скомпрометированный upstream tag/cache может дать одинаковые два malicious artifacts, которые затем получат project metadata/attestation без доказательства base identity.
- **Условия:** изменение upstream tag/registry/cache либо компрометация builder supply chain.
- **Корневая причина:** hash repeatability внутри одной среды приравнена к hermetic reproducibility/provenance.
- **Связанные замечания:** SUP-002, CI-001.
- **Направление / масштаб / риск исправления:** pin image digests/toolchain, сохранить base/material digests, строить в независимых clean environments или использовать verifiable hermetic builder. Масштаб средний, риск высокий для release pipeline compatibility.
- **Проверка после исправления:** metadata содержит immutable materials; rebuild на независимом runner совпадает; deliberate digest/tag substitution отклоняется.

### SEC-004 — Trusted peer может оборвать multi-provider fetch

- **Категория / уверенность / статус:** security/availability; высокий; подтверждено независимой трассировкой.
- **Подсистема и файлы:** `internal/transfer/private_exchange.go:172-185`; `fetch_response_auth.go:37-75`; `fetch_response_protocol.go:29-45,82-102`; `fetch_await.go:19-48`; `manifest_response.go:65-88`; `manifest_fetch.go:41-62`.
- **Фактическое наблюдение и доказательство:** responses маршрутизируются waiter по видимому `request_id`. Signed error не связывает resource identity и после trust/signature проверки становится terminal; await не ждёт других providers.
- **Ожидаемый контракт / нарушение:** ответ одного кандидата должен завершать broadcast fetch только при успешной проверке запрошенного resource либо при определённом quorum/selected-provider contract.
- **Влияние:** trusted/discoverable channel member видит request, выигрывает race signed error и детерминированно отказывает blob/manifest fetch от честного provider.
- **Условия:** peer usable по Discovery/Trust, имеет private channel publish authority и видит request; arbitrary untrusted peer не подходит.
- **Корневая причина:** request correlation не фиксирует ожидаемый responder/resource для error path, а первая terminal failure глобальна.
- **Связанные замечания:** SEC-001, SEC-002.
- **Направление / масштаб / риск исправления:** bind error to resource/candidate, агрегировать candidate outcomes, завершать failure только после исчерпания допустимых providers. Масштаб средний, риск высокий для latency/retry semantics.
- **Проверка после исправления:** multi-responder race: malicious trusted error первым, honest success позже; untrusted response игнорируется; all-candidates-fail остаётся bounded.

## P2

### ARCH-001 — Structural acceptance больше не соответствует tree

- **Категория / уверенность / статус:** architecture debt/standards; высокий; подтверждено полным tracked inventory.
- **Подсистема и файлы:** `docs/engineering/codebase-architecture.md:84-91,346,722,1009-1061`; `internal/messaging/private.proto`; `.agents/**`.
- **Фактическое наблюдение и доказательство:** восемь handwritten packages превышают budget 12 (`identity/access` 24, `daemon` 16, шесть по 13); четыре handwritten internal packages не имеют package docs; private generated contract смешан с domain; фактически девять proto services; tracked `.agents` запрещён target tree.
- **Ожидаемый контракт / нарушение:** architecture acceptance объявляет эти ограничения исчерпывающими и выполненными, но текущий commit их не проходит.
- **Влияние:** standards не могут служить review gate; разработчик не понимает, является ли tree допустимым или накопил нарушения.
- **Условия:** любое изменение, оцениваемое по текущему architecture document.
- **Корневая причина:** structural rules не автоматизированы либо не обновлялись вместе с ростом модулей/tooling.
- **Связанные замечания:** DOC-001, QLT-001.
- **Направление / масштаб / риск исправления:** сначала решить, какие constraints всё ещё нужны, затем автоматизировать; исключения оформить явно. Масштаб средний, риск низкий/средний.
- **Проверка после исправления:** machine-readable tree/package budget gate и review обновлённой decision matrix.

### CLI-001 — CLI session validation использует stale clock

- **Категория / уверенность / статус:** duplication/correctness; высокий; подтверждено source comparison, не runtime.
- **Подсистема и файлы:** `internal/cli/client/session.go:103-109,137,159-209`; `internal/identity/access/service.go:102-110,422`; `sdk/go/internal/adapter/session.go:186,234`.
- **Фактическое наблюдение и доказательство:** CLI фиксирует `now` до Begin RPC и отклоняет server `IssuedAt > now`; server формирует timestamp позже и округляет до секунды. SDK пересэмплирует время.
- **Ожидаемый контракт / нарушение:** timestamp validation должна учитывать network duration/skew и быть единой для CLI/SDK.
- **Влияние:** intermittent false rejection при пересечении секундной границы; Complete expiry существенно вероятна только при максимальном session lifetime, default имеет запас.
- **Условия:** server-issued second позже client sample; для expiry — длинный exchange/maximum lifetime.
- **Корневая причина:** security-sensitive handshake продублирован и получил разные clock semantics.
- **Связанные замечания:** DOC-001.
- **Направление / масштаб / риск исправления:** общий protocol client/validator или CLI поверх SDK; явно задать skew. Масштаб средний, риск средний.
- **Проверка после исправления:** advancing fake clock между Begin/Complete, boundary-second и skew tests одинаково для CLI/SDK.

### DOC-001 — Legacy token/loopback access model остаётся в entry docs

- **Категория / уверенность / статус:** documentation drift; высокий; подтверждено code/docs comparison.
- **Подсистема и файлы:** `README.md:22,36,102`; `docs/protocols/communication-contracts.md:45-56`; `docs/product/distribution-model.md:42,46`; `docs/operations/operator-runbook.md:22`; против `docs/product/principal-identity-and-access.md:703,728,873-886` и `docs/operations/operator-access-contract.md:22,37,43`.
- **Фактическое наблюдение и доказательство:** старые документы требуют API token/loopback control API/OpenSSH path; текущий runtime использует Principal credentials/sessions на раздельных Unix sockets и прямо запрещает permanent shared tokens/plaintext loopback.
- **Ожидаемый контракт / нарушение:** README, protocol и runbook не должны предписывать удалённую модель безопасности.
- **Влияние:** оператор проектирует неверную automation/secrets/network exposure и не может выполнить инструкции на текущем runtime.
- **Условия:** onboarding, deployment или интеграция по старым entry docs.
- **Корневая причина:** Principal migration обновила implementation и новые normative docs, но не полный documentation graph.
- **Связанные замечания:** ARCH-001, CLI-001.
- **Направление / масштаб / риск исправления:** назначить один canonical access contract, переписать entry docs и добавить link/check matrix. Масштаб средний, риск низкий.
- **Проверка после исправления:** docs search не находит активных token/loopback directives вне явно исторического раздела; команды runbook проходят docs smoke.

### OPS-004 — Native readiness игнорирует настраиваемый address

- **Категория / уверенность / статус:** operations; высокий; подтверждено source/config trace.
- **Подсистема и файлы:** `scripts/install/linux.sh:95-105,317-344`; `internal/config/types.go:73-75`; `internal/config/validate.go:200-216`; `internal/daemon/configuration.go:394-396`.
- **Фактическое наблюдение и доказательство:** upgrade/rollback всегда curl `127.0.0.1:9090/readyz`, хотя daemon bind берёт настраиваемый loopback address.
- **Ожидаемый контракт / нарушение:** installer должен проверять фактическую конфигурацию установленного service.
- **Влияние:** здоровый узел на другом разрешённом порту ложно отклоняется и компенсируется; посторонний ready JSON на 9090 может дать ложное принятие.
- **Условия:** non-default observability address либо другой local process на 9090; initial install этим helper не gated.
- **Корневая причина:** installer дублирует runtime default вместо чтения effective config.
- **Связанные замечания:** OPS-003, CI-002.
- **Направление / масштаб / риск исправления:** читать/получать effective endpoint и аутентично связывать probe с установленным daemon. Масштаб средний, риск средний.
- **Проверка после исправления:** upgrade/rollback на default и alternate port; decoy server на 9090 не должен пройти.

### QLT-001 — Critical lifecycle seams не имеют достаточного покрытия

- **Категория / уверенность / статус:** testability/quality; высокий; подтверждено coverage profile и test inventory.
- **Подсистема и файлы:** `cmd/*` — 0%; `internal/ingressproxy` — 0%; `internal/workload` core — 5.6%; `internal/hosting` — 9.7%; `internal/network/routing` — 5.1%; `internal/replication/availability` — 0%; соответствующие lifecycle paths REL-001—REL-004.
- **Фактическое наблюдение и доказательство:** общий runtime handwritten statement coverage 56.7%, но process/adapter/lifecycle seams, где найдены P1, отсутствуют или покрыты единицами процентов. Race run прошёл только по выбранным пакетам; ingress не содержал tests.
- **Ожидаемый контракт / нарушение:** high-impact shutdown, proxy, external-engine timeout и rollout state machine должны иметь deterministic failure-injection tests, а не только happy-path domain tests.
- **Влияние:** локальные unit suites остаются зелёными при process-wide hangs, remote DoS и broken compensation.
- **Условия:** изменения или failures на infrastructure/lifecycle boundary.
- **Корневая причина:** тестовая пирамида сильна в domain behavior, но executable/deployment adapters трудно подменяются и не имеют fault contract.
- **Связанные замечания:** REL-001—REL-004, OPS-001—OPS-003, TST-001.
- **Направление / масштаб / риск исправления:** injectable transports/clocks/process supervisor и сценарии по найденным invariants; не ставить общий процент как самоцель. Масштаб большой, риск низкий для production, средний для test architecture.
- **Проверка после исправления:** каждый P1 seam имеет deterministic negative test; coverage threshold — по critical-file diff, плюс race/lifecycle gates.

### SEC-005 — Alias paths дают два replay snapshots над одним файлом

- **Категория / уверенность / статус:** reliability/security; средний; source trace подтверждён, runtime scenario не исполнялся.
- **Подсистема и файлы:** `internal/config/validate.go:116-120`; `internal/daemon/configuration.go:415-417,635-643,666-685`; `internal/messaging/replay.go:18-19,54-101`.
- **Фактическое наблюдение и доказательство:** config сравнивает replay paths как строки и не canonicalizes их. Два ledger с одинаковыми bucket/key загружают отдельные snapshots и каждый сохраняет whole clone.
- **Ожидаемый контракт / нарушение:** пути должны указывать на разные physical stores либо store должен координировать namespaces/transactions.
- **Влияние:** `/x/replay.db` и `/x/./replay.db`, case/symlink alias перезаписывают state друг друга; после restart стёртый MessageID снова принимается.
- **Условия:** operator misconfiguration с filesystem-equivalent paths; действительно разные files безопасны.
- **Корневая причина:** semantic path identity проверяется до filesystem normalization, а ledger persistence использует snapshot replacement.
- **Связанные замечания:** SEC-002.
- **Направление / масштаб / риск исправления:** canonical/same-file validation либо один shared store с namespaces. Масштаб средний, риск средний для path portability.
- **Проверка после исправления:** dot/case/symlink aliases на поддерживаемых ОС и restart replay test двух ledger.

### SUP-002 — Manual `-Commit` не связан с собираемым worktree

- **Категория / уверенность / статус:** supply-chain correctness; высокий; подтверждено source trace.
- **Подсистема и файлы:** `scripts/release/build.ps1:15-25,39-49,57-96`; `scripts/release/verify.ps1:84-90,123-129`; `ardents.ps1:101-107`.
- **Фактическое наблюдение и доказательство:** caller может передать любой hex SHA; он задаёт timestamp/embedded metadata, но Docker собирает bind-mounted текущий worktree. Verifier сравнивает artifact только с тем же caller value.
- **Ожидаемый контракт / нарушение:** declared commit должен быть точно источником bytes либо build должен доказать HEAD equality/clean checkout.
- **Влияние:** clean HEAD B с `-Commit A` создаёт bytes B, маркированные и проверенные как A. Официальный CI не передаёт override и этим сценарием не затронут.
- **Условия:** manual/offline release с явным отличающимся `-Commit`.
- **Корневая причина:** provenance input используется как label, но не как source selector/invariant.
- **Связанные замечания:** SUP-001.
- **Направление / масштаб / риск исправления:** запретить mismatch либо собирать archive/tree указанного object. Масштаб малый/средний, риск низкий.
- **Проверка после исправления:** negative test HEAD B + `-Commit A`; dirty/untracked/source archive cases.

### TST-001 — Integration teardown оставляет TempDir занятым

- **Категория / уверенность / статус:** test reliability и возможный lifecycle defect; высокий для наблюдения, средний для точной причины.
- **Подсистема и файлы:** `tests/integration/discovery/degraded_test.go:20-42`; `internal/daemon/api.go:189-204`; evidence `tests/.artifacts/reports/audit-20260723-integration/{junit.xml,summary.json}`.
- **Фактическое наблюдение и доказательство:** Docker integration run: 44 tests, 43 passed, один failed. Все assertions `TestDiscoveryDegradesWhenBootstrapPeerIsUnavailable` прошли, затем `testing.TempDir RemoveAll cleanup` получил `directory not empty`. Четыре более ранних retained reports этого дня показывают pass, поэтому дефект intermittent.
- **Ожидаемый контракт / нарушение:** `Stop` должен дождаться прекращения всех background writers до возврата; integration teardown должен быть deterministic.
- **Влияние:** flaky release evidence и риск, что runtime продолжает filesystem activity после заявленного stop.
- **Условия:** timing race во время teardown/replay/store activity; точный writer статически не локализован.
- **Корневая причина:** вероятно, незавершённый background lifecycle; это гипотеза, а наблюдаемый cleanup failure подтверждён.
- **Связанные замечания:** REL-003, REL-004, QLT-001.
- **Направление / масштаб / риск исправления:** сначала instrument goroutine/store shutdown ownership, затем устранить writer-after-stop. Масштаб неизвестен/средний, риск средний.
- **Проверка после исправления:** targeted stress/repeat в отдельной задаче с TempDir watcher и goroutine diagnostics; затем полный integration run один раз.
