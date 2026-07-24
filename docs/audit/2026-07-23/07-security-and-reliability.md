# Безопасность и надёжность

## Общая оценка

Проект содержит сильную Principal/capability модель, строгую canonicalization, криптографически защищённые private protocols и ограниченный Docker workload profile. Основные риски находятся не в примитивах, а в последовательности проверок и переходах между владельцами состояния:

- authorization identity → persistence identity;
- AEAD acceptance → durable replay mutation → signature/permission;
- untrusted Waku relay → upstream persistent store;
- RPC/network connection → process lifecycle;
- external Docker call → shared runtime lock/shutdown;
- release/deployment intent → фактические материалы и compensation.

Подтверждено 13 security/reliability findings, из них 11 P1 и 2 P2. Operations/CI/supply-chain findings, влияющие на безопасную поставку, рассматриваются отдельно в полном реестре.

## Границы доверия

| Граница | Доверенный субъект / доказательство | Недоверенный ввод | Основные контроли | Остаточный риск |
|---|---|---|---|---|
| Local process → Operator API | root-signed device credential, Unix peer binding, session | RPC payload, session token, peer process | challenge, audience/source binding, revocation recheck, canonical action/target | owner target теряется в content persistence; stream живёт дольше process context |
| Local application → Application API | application Principal/device/session и resource binding | app RPC payload | отдельный socket/audience, binder, sealed call | ticket handoff и часть binding-кода слабо покрыты |
| Channel member → private messaging | channel secret, Subscribe/Publish bits, node signature | encrypted envelope, header, payload | XChaCha20-Poly1305, signature, class lifetime, replay ledger | replay state меняется до подписи/Publish |
| Remote peer → Waku | libp2p peer/network reachability | relay/store messages | per-message size, protocol parsing | Store persistence не ограничена Ardents auth или retention |
| Trusted discovered peer → transfer | signed node identity, current trust, channel authority | fetch responses/errors | signature/discovery/trust checks | первый trusted error глобально завершает fetch |
| Daemon → filesystem | local OS permissions и configured paths | config paths, retained/corrupt state, background writers | private file modes, bbolt transactions, strict config | alias replay paths; one-time ticket delivery gap; teardown race |
| Daemon → Docker Engine | local privileged socket | engine responses, hangs, container state | constrained create options, immutable workload images | unbounded calls под shared mutex; proxy lifecycle не supervised |
| Internet → workload ingress | TCP listener | arbitrary connections/stream behavior | port validation, 128-slot semaphore, constrained proxy container | single connection kills proxy; idle slot exhaustion |
| Source → CI/release/deploy | Git SHA, workflow, scripts, image tags | builders, registry tags, environment failures | hash verification, signed workflow identities, backups/rollback intent | static gate broken, mutable builders, incomplete compensation/readiness |

## Положительные контроли

- Principal/Device IDs имеют domain-separated canonical derivation.
- Challenge/session связаны с audience, interface/protocol major и local peer identity.
- Sessions ограничены по времени и повторно проверяют device revocation.
- Local API централизует exact action/target admission и передаёт sealed `AuthorizedCall`.
- Private payloads используют sign-then-encrypt, XChaCha20-Poly1305, HKDF и X25519/HPKE.
- Config parsing отклоняет unknown/duplicate fields и проверяет loopback observability.
- Private files создаются с ограничительными правами; config snapshots redacted.
- Workload containers запускаются non-root, read-only, без capabilities, с `no-new-privileges` и resource limits.
- Importguard удерживает Waku/libp2p и Docker SDK в concrete adapters.
- `govulncheck` не нашёл уязвимостей в вызываемом или импортируемом коде.

Эти сильные стороны не компенсируют findings ниже, но уменьшают их attack surface. Например, replay poisoning требует channel secret, а transfer veto — trusted channel peer.

## Security findings

### SEC-001 — Cross-owner Object/Manifest access

Authorization проверяет Owner, handler/store — нет. Это нарушение tenant/resource isolation с чтением и overwrite, а не только модельный долг. Приоритет P1; полная трассировка и remediation constraints — в `04-findings-register.md`.

### SEC-002 — Durable replay poisoning

Subscribe-only participant, уже знающий channel secret, может занять 4096 replay entries AEAD-valid, но не подписанными/не разрешёнными сообщениями с outer expiry до 30 дней. Произвольный внешний клиент без secret не подходит. Приоритет P1.

### SEC-003 — Unbounded unauthenticated Waku persistence

Default service node принимает non-ephemeral relay traffic в SQLite Store до Ardents-level Principal/private-envelope authentication. В pinned go-waku v0.10.3 retention выключена, если policy не передана; Ardents её не передаёт. Это remote disk-exhaustion path. Приоритет P1.

### SEC-004 — Trusted peer fetch veto

Discoverable/trusted channel peer может первым отправить signed error с видимым request ID. Error path не связывает requested resource и становится terminal для всех кандидатов. Untrusted sender отбрасывается; угроза — malicious insider/compromised trusted node. Приоритет P1.

### SEC-005 — Replay store alias

Две разные config-строки могут указывать на один файл. Независимые in-memory snapshots поочерёдно заменяют один bucket/key, и после restart стёртый replay ID забывается. Требуется operator misconfiguration либо path alias; приоритет P2.

## Identity и provisioning reliability

### IAM-001 — Recovery guard недоступен из-за normal expiry

Retained credential, который нормально истёк, трактуется loader как corrupt и прерывает scan recovery devices. В результате невозможно отозвать device или активный recovery grant даже при другом действующем recovery device. Это fail-closed, но нарушает доступность критичной defensive operation.

### IAM-002 — Commit-before-delivery one-time tickets

Bootstrap/application ticket digest долговечно сохраняется раньше, чем единственная plaintext capability попадает в файл/ответ клиента. Ошибка после commit создаёт недоставленный активный ticket и временно блокирует перевыпуск. Решение должно сохранить single-use/anti-leak свойства; простой лог plaintext недопустим.

## Runtime reliability

### REL-001 и REL-002 — Ingress availability

Connection-level error поднимается до process-fatal, а proxy container не имеет постоянной restart policy/supervisor. Отдельно 128 idle connections без deadlines занимают общий semaphore всех портов. Оба пути достижимы удалённо через опубликованный ingress и имеют разные remediation: error isolation/supervision и fair idle resource policy.

### REL-003 — Unbounded stream drain

`StreamNodeEvents` не получает process cancellation, а `http.Server.Shutdown(context.Background())` ждёт его до deferred domain cleanup. Авторизованный watcher может непреднамеренно или намеренно довести systemd stop до SIGKILL.

### REL-004 — Docker control-plane stall

Часть query/metrics/shutdown paths использует `context.Background()` и выполняет Docker I/O под shared runtime/node mutex. Hung engine способен остановить control plane и graceful shutdown. Startup inventory имеет отдельный timeout; finding не распространяется на каждый Docker call.

## Supply-chain и deployment posture

К security/reliability posture напрямую относятся:

- CI-001/CI-002 — обязательные gates не могут сформировать зелёное trusted evidence;
- OPS-001 — supported upgrade отсутствует;
- OPS-002 — compensation оставляет failed current node;
- OPS-003 — acceptance не доказывает identity/API/Diagnostics state;
- OPS-004 — native readiness может проверять не тот endpoint;
- SUP-001 — builder/base identities не входят в provenance;
- SUP-002 — manual release SHA не связан с source bytes.

Ни один из этих defects сам по себе не доказывает компрометацию текущих artifacts. Они означают, что pipeline не обеспечивает заявленную гарантию при ошибке или adversarial upstream input.

## Dependency posture

`govulncheck ./...`:

- vulnerabilities in called code: 0;
- vulnerabilities in imported packages: 0;
- module-only: GO-2026-5932 для `golang.org/x/crypto/openpgp` в `x/crypto@v0.54.0`, не вызывается проектом.

`docs/security/security-exceptions.md` уже описывает этот exception. Отсутствие called vulnerabilities не является полным SCA/container verdict: image packages, dynamically reached code, configuration CVEs и будущие disclosures вне этого результата.

## Availability failure map

| Внешнее событие | Локальный механизм | Масштаб отказа | Finding |
|---|---|---|---|
| TCP reset/copy error | connection error → global event → proxy exit | весь ingress одного workload | REL-001 |
| 128 idle sockets | общий semaphore без idle timeout | все ingress-порты workload | REL-002 |
| открытый events stream при SIGTERM | unbounded HTTP drain | весь daemon stop/cleanup | REL-003 |
| hung Docker response | Background I/O под mutex | workload API, metrics, shutdown | REL-004 |
| 4096 invalid inner envelopes | replay admit до auth | private channel до expiry | SEC-002 |
| поток Waku relay frames | persistent Store без retention | диск service node | SEC-003 |
| signed early error | first terminal fetch result | конкретные/повторяемые fetches | SEC-004 |
| failure после ticket commit | active digest без plaintext | bootstrap/enrollment до expiry/restart | IAM-002 |

## Рекомендуемый порядок security stabilization

1. Закрыть remote unauthenticated/low-privilege availability paths: Waku retention, ingress process/idle behavior, replay mutation order.
2. Исправить tenant/resource identity Object/Manifest до допуска нескольких operator principals.
3. Восстановить defensive identity operations и recoverable ticket handoff.
4. Ограничить shutdown/Docker calls и доказать cleanup.
5. Исправить transfer multi-provider failure semantics.
6. После восстановления CI/deploy gates закрепить adversarial regression tests и immutable release materials.

Полные условия, строки, масштаб и способ проверки каждого finding находятся в `04-findings-register.md`; machine-readable подтверждённые security findings — в `findings.json`.
