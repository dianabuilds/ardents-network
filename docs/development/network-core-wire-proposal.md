# Сетевое ядро: wire-кандидат Service Connection

2026-09-21. **Предложение, не принятый контракт и не разрешение реализации.**
Приложение к [единому проекту и карте перехода](network-core-transition.md).
Этот документ владеет предлагаемыми bytes/полями/фазами Connection;
проект владеет архитектурой, реальными callers, claims, миграцией и задачами.
Текущий принятый [protected Route protocol](../technical/protected-route-protocol.md)
сохраняет приоритет; никакие новые ALPN, IDs или правила не объявляются принятыми.

Подготовка к принятию разнесена по отдельным карточкам карты
[#50](https://github.com/dianabuilds/ardents-network/issues/50): #214 — encoding,
#215 — binding/handoff, #216 — initial/ContinueOpen, #224 — DATA/ACK,
#225 — assignment, #221 — EOF/Settled, #222 — terminal retention.
Публикация этих карточек не принимает кандидат и не начинает runtime changes.

Собрана согласованная редакция из RESEARCH `connection-wire-draft.md`,
`g06-g07-rule-revision.md` и `instance-offer-authentication.md`.
Внешний RESEARCH — каталог
`ardents-handover-model-11a26ed48d104b1a826aa64fd2e1ea9c` в системном Temp.
Имена внешних evidence ниже обозначают происхождение и ограничения проверок,
а не обязательные документы для определения полей этой grammar.
SHA-256 исходного wire-файла до переноса:
`6c4d8748f893df5d2f67393cc7e3575dc3ab9ff623cb274507fc4978cd0852b2`.
История внешних probes остаётся отдельно; их PASS не переносится на runtime.

C0 здесь — immutable Connection context, а не название delivery milestone.
Все kind IDs, ALPN, domain labels и parser bounds — значения кандидата,
не публичный registry. Секрет Kcont и внутренний handle не являются Node identity.

## 1. Транспортная и криптографическая граница

Один и тот же сквозной ordered stream работает поверх разрешённого Route,
независимо от того, какой TCP/TLS или QUIC Carrier используется между узлами.
В этом кандидате B — Service/TLS server, A — исходный logical initiator/TLS
client. Физический dial Route не меняет роли A/B и координатора назначения A.
B-initiated канал возможен лишь при отдельно проверенной достижимости A;
данный wire её не создаёт.

Предлагается exact end-to-end ALPN `ardents-connection/1`, TLS 1.3 без 0-RTT
и tickets для этого профиля. Проверяется сквозной Service Instance key и его
текущая авторизованная привязка Target/Publication, а не только сертификат relay.
ALPN не несёт Target/Name и не заменяет эти проверки. Отсутствующий/иной ALPN
отклоняется до сообщений. Текущий text_service_tls.go не задаёт этот ALPN:
это предложенное изменение, не свойство существующего runtime.

Никакой Connection handle, Name, C0, Kcont или доказательство local Principal
не входит в outer Carrier ALPN/SNI/QUIC CID. Private local authority остаётся
локальной; запрос на канале не выдаёт её удалённому узлу. Все сообщения ниже
находятся внутри сквозного Service TLS.

## 2. Представление и пределы

Frame = uint32 big-endian L || canonical CBOR body ровно L bytes.
L: 1..16448. До аутентифицированного data-ready тела дополнительно <=1024;
все неданные control kinds этого кандидата <=1024 и после ready.
DATA payload: 1..16384; 0-byte DATA запрещён, пустой поток закрывается EOF(0).
Это технические parser bounds кандидата, не throughput или размер всего job.
Они относятся к сквозной Connection. Route ARDP имеет свой body bound
(`internal/route/closed_lane.go`), поэтому один Connection frame может
переноситься несколькими нижними frames. Нельзя увеличить Route allowance
до размера Connection frame или считать TLS/Route overhead бесплатным.

CBOR: [core deterministic encoding RFC 8949 §4.2.1](https://www.rfc-editor.org/rfc/rfc8949.html#section-4.2.1); definite fixed arrays,
uint64, exact bstr lengths и bool. Maps/tags/floats/null/undefined/negative ints
не входят в wire schema. Нет необязательных/неизвестных/trailing полей.
Maximum arity 16 и nesting 4 достаточны приведённым schemas; сам decoder обязан
ещё проверить точную arity каждого типа. Отсутствие ключа и zero не синонимы.

Typed decode выполняется в новый временный объект; полный semantic validation
и сравнение canonical representation предшествуют effects. Один writer
сериализует frames на канале. Частичная ошибка записи отравляет только этот
канал: не дописывать следующий frame поверх неизвестного префикса.

Обозначения:
- b32/b64: bstr ровно 32/64 bytes; u64: 0..2^64-1; role: A=0, B=1.
- direction: 0 означает A→B, 1 означает B→A; ACK направлен обратно данным,
  но его direction обозначает подтверждаемое направление.
- Положительные generation/revision не равны 0, не оборачиваются.
- Даты: целые Unix seconds в диапазоне 1..2^63-1, проверенные time owner;
  monotonic local deadlines не передаются как чужие monotonic timestamps.

## 3. Неизменный C0 и предложение лимитов

PROPOSE body имеет 13 элементов:
[16, 1, network:b32, target:b32, instanceKey:b32, instanceGeneration:u64,
 publicationDigest:b32, profileDigest:b32, initiatorBinding:b32,
 nonceA:b32, routeBinding:b32, limitsA:[maxBytesA:u64,maxBytesB:u64,windowA:u64],
 bounds:[workNotAfter:u64,workMaximum:u64,noNewRecoveryAfter:u64]]

maxBytesA/B — maxima данных соответствующих направлений. windowA — память
для входящего потока B→A, зарезервированная A. B возвращает windowB для A→B.
C0 limits = [maxBytesA,maxBytesB,windowA,windowB]. В OFFER B может только снизить
предложенные maxima/windowA и выбрать windowB в пределах своих разрешений.
A принимает уменьшение лишь если локальный запрос допускает такие условия.
Нет автоматического уменьшения безопасности, смены profile или другого Target.

windowA<=maxBytesB; windowB<=maxBytesA. Нулевое window допустимо лишь для
направления с нулевым maxBytes. Иначе window положительное и реально reserved.
Initial receive credit равен window получателя. Последующий receiveLimit
не больше min(maxBytesDirection, delivered + reservedWindow), со checked
arithmetic. Credit — абсолютный монотонный предел, не прибавляемый счётчик.
Не хранить unbounded data ради указанного peer максимума.

0 < noNewRecoveryAfter <= workNotAfter <= workMaximum;
все значения дополнительно ограничены actual authority и локальным запросом.
При исходном admission (до OFFER и перед CONFIRM/Start) оба конца независимо
требуют currentTime < noNewRecoveryAfter и currentTime < workNotAfter через
своего time owner. Изначально просроченное право восстановления не принимается.
Это не гарантирует достаточность оставшегося времени для сети/потери ответа:
локальный запрос задаёт также minimum acceptable workNotAfter и
noNewRecoveryAfter, проверяемые A вместе с минимальными объёмами данных.
Эти минимумы проверяются до выбора capsule bounds и при принятии OFFER.
Непринятое предложение не разрешает CONFIRM или Application effect.
Минимумы не выбирает peer, они не добавлены как public wire fields. Конкретные
числа и запас на Time Confidence/Route setup ещё требуют общего бюджета.
Эта проверка исходного admission не запрещает уже принятой Live Connection
завершать работу после noNewRecoveryAfter в пределах workNotAfter; отдельные
правила новых Attachments и сохранённых путей остаются действующими.
Сверка с выбранным Introduction-контрактом выявила ошибку прежнего предложения:
сроки уже фиксированы в capsule до PROPOSE. Теперь требуется exact equality
capsule = PROPOSE = C0 для всех трёх bounds; B отказывает, если не может их
принять. Ресурсные maxima/windows по-прежнему могут уменьшаться отдельно.
Разбор композиции и выполненная проверка (внешнее evidence: `route-connection-composition.md`).
После C0 никакой refresh/новый nonce не увеличивает эти границы.
Work maximum не означает автоматическое продление
workNotAfter. Точное изменение времени активной работы не вводится этим wire.

C0 имеет 14 элементов:
[1, network, target, instanceKey, instanceGeneration, publicationDigest,
 profileDigest, initiatorBinding, nonceA, nonceB:b32, handle:b32,
 limits:[maxBytesA,maxBytesB,windowA,windowB], bounds,
 proposeDigest:b32]

nonceA — тот же fresh Connection-nonce, созданный до исходной Introduction;
capsule, PROPOSE и retained C0 содержат его неизменным. Он не виден Rendezvous.
nonceB и handle — свежие CSPRNG values для исходного открытия.
handle случайно создаётся B до OFFER и входит в exact Prepared record.
proposeDigest = SHA-256(полного исходного PROPOSE frame, включая L).
initiatorBinding — сохранённое per-Connection opaque commitment локальной
Destination Binding, не raw Principal/Name и не полномочие для B. Его локальная
семантика согласуется с name-resolution-contract.md и general authorization.

В C0 каждое поле, кроме разрешённого снижения limits и новых nonceB,
handle, должно точно соответствовать PROPOSE/проверенному Service. B не может
подменить profile, расширить maximum или выбрать иную публикацию. Согласование
чисел не означает, что issuer/admission права появились из peer сообщения.

C0bytes — exact canonical CBOR encoding самого массива C0, без OFFER wrapper.
H0 = SHA-256(ASCII `ardents-connection-context/1` || 0x00 ||
            uint32be(len(C0bytes)) || C0bytes).
Сохранить C0bytes/H0; не реконструировать их позднее из изменившегося State/Name.

## 4. Ключ, channel binding и proof transcript

Kcont = TLS-Exporter(initial end-to-end TLS,
 `EXPORTER-Ardents-Connection-Continuity-v1`, H0, 32).
CB = TLS-Exporter(current end-to-end TLS,
 `EXPORTER-Ardents-Connection-Channel-v1`, channelContext, 32).

routeBinding = SHA-256(ASCII `ardents-attachment-route/1` || 0x00 ||
 uint32be(len(introductionPlaintext)) || exact introductionPlaintext).
channelContext = SHA-256(ASCII `ardents-connection-channel-context/1` ||
 0x00 || H0 || routeBinding).

Проверка реального caller (внешнее evidence: `join-service-source-trace.md`) показала, что нынешний
attempt.digest — SHA-256 plaintext без этого domain/length. Он не может быть
подставлен как routeBinding. Capsule owner должен вычислить commitment выбранного
протокола, пока exact bytes доступны, и передать его с тем же consumed JOIN.

Это общая end-to-end привязка к конкретной recipient-confidential Introduction
капсуле, содержащей проверенные join/Target/Publication/profile/bounds факты;
не digest полного клиентского пути и не раскрытие его B. A берёт exact bytes
созданной капсулы; B — exact bytes после её аутентифицированного раскрытия и
проверки всех полей/current authority. Обе стороны связывают именно эту капсулу
с реально полученным JOIN channel. Peer-supplied routeBinding без совпадения
с этими локально проверенными фактами отвергается. Capsule grammar и actual
JOIN owner являются обязательным входом Route-контракта; тестовый trusted digest
не закрывает эту интеграцию. Выбранная исходная Service binding должна совпасть
с капсулой; новый запрос не даёт продолжать чужой или иной профиль маршрута.
Первоначальный routeBinding включён в PROPOSE и связан с C0 через proposeDigest.
У нового канала он свежий и находится в CONTINUE_REQUEST; исходный C0 неизменен.

Kcont извлекается только из исходного TLS и сохраняется исходным owner.
Новый CB никогда не заменяет Kcont. TLS exporter context для Kcont — H0, для CB — channelContext;
не писать свой HKDF и не использовать early exporter. Fresh TLS после потери
Kcont/state не восстанавливает исходные права.

Для сообщения с proof последнее поле всегда b32. Сначала проверить exact
schema/canonical bytes. ZeroProofFrame — копия ПОЛНОГО принятого frame с
последними 32 байтами (содержимое последнего proof bstr) заменёнными нулями;
length, array shape и bstr header сохраняются. До проверки формы нельзя
вслепую обнулять последние 32 bytes произвольного input.

proof = HMAC-SHA-256(Kcont, canonical CBOR array
 [`ardents-connection-proof/1`, senderRole, H0:b32, CB:b32,
  ZeroProofFrame:bstr]).

Это protocol composition из стандартных primitives, не собственная реализация
криптографии. Go crypto/tls, crypto/hmac, crypto/sha256 и constant-time MAC
comparison; proof проверяется ДО state effect. Разные kinds и роль входят
в transcript; отражение OFFER/CONFIRM либо REQUEST/ACCEPT невалидно.
Encoder создаёт frame с zero proof, вычисляет MAC и заменяет ровно этот payload.
Transcript построен из exact bytes, а не произвольного повторного marshal.

Для response на continuation request:
requestDigest = SHA-256(полного exact REQUEST frame с настоящим proof).
Он связывает также intent и роль запросившего. Это digest внутри сквозной
защиты, не общий relay correlation ID.

## 4.1. Подпись Instance в OFFER

`instanceDigest = SHA-256(ASCII "ardents-instance-offer/1" || 0x00 || H0[32] || CB[32])`.

Exact current Ed25519 Instance signer подписывает эти 32 bytes обычным
Ed25519 (`crypto.Hash(0)`), не Ed25519ph. Publisher проверяет полученные
64 bytes ожидаемым public key. A берёт ключ из независимо проверенной
Publication, а не из OFFER. Подпись включена в HMAC proofB; подпись сама
не включает proofB, циклического вычисления нет.

Порядок B: проверить Service TLS, exact JOIN/capsule, PROPOSE и local
authority → зарезервировать один Prepared → сформировать C0/Kcont/CB →
вызвать текущий signer → снова проверить Stop/currentness/deadline →
сохранить exact OFFER и отправить. Ошибка signer не допускает Start.
Exact retry возвращает прежний OFFER без нового signer/owner и без
продления срока, но всё равно проходит current checks.
A проверяет C0/PROPOSE/capsule, подпись и proofB перед CONFIRM.
Передача Application разрешается только отдельным переходом §6.

Это перенос предложения `instance-offer-authentication.md`, не удаление
нынешней InstanceProof без migration decision. Новый ALPN/grammar и
изменённая точка signer call требуют согласованного adoption N07/N08.

## 5. Полный registry сообщений Connection

Единая таблица фаз, REFUSE/ABORT и commit boundaries —
таблица фаз §5.1 и правила §6–7. Registry ниже задаёт
форму сообщений; разрешение эффекта дополнительно требует соответствующей
строки этой таблицы. Весь документ остаётся кандидатом, не принятым wire.

| Kind | Точная body array | Отправитель и разрешённый приём |
|---|---|---|
| 1 DATA | [1,direction:u64,offset:u64,payload:bstr] | Только отправитель direction на exact data-ready path; Live, credit/offset/EOF checks |
| 2 ACK | [2,direction:u64,acceptedOffset:u64,receiveLimit:u64,finalAccepted:bool] | Получатель direction; accepted path, Live либо duplicate terminal control в Settled |
| 3 EOF | [3,direction:u64,finalOffset:u64] | Отправитель direction; Live, либо exact duplicate в Settled |
| 16 PROPOSE | Массив §3 | Только A, исходный канал до Prepared либо exact повтор в Prepared; нет Application effects |
| 17 OFFER | [17,C0,instanceSignature:b64,proofB:b32] | Только B; A проверяет exact Instance signature, предложение/свою authority и proofB, затем удерживает Kcont |
| 18 CONFIRM | [18,handle:b32,H0:b32,proofA:b32] | Только A, exact Prepared/Starting/Live первоначального канала; один Start, duplicate join |
| 19 OPEN_READY | [19,handle:b32,H0:b32,proofB:b32] | Только B после однократной регистрации retained stream у consumer, обеспеченного приёма и local Live; не ждать request DATA/worker OPEN; A завершает исходный Open на этом канале |
| 32 CONTINUE_REQUEST | [32,role:u64,handle:b32,H0:b32,intent:u64,attemptNonce:b32,routeBinding:b32,proof:b32] | Один первый request на новом канале. intent 0=AttachLive, 1=ContinueOpen; intent 1 только A→B |
| 33 CONTINUE_ACCEPT | [33,role:u64,requestDigest:b32,proof:b32] | Противоположная request роль, только после проверки сохранённого Kcont и нужного state; candidate ещё не data-ready |
| 34 SETTLED_RECEIPT | [34,role:u64,requestDigest:b32,finalA:u64,finalB:u64,proof:b32] | Противоположная request роль с Settled record; ответ вместо ACCEPT, никогда не App launch/Ready |
| 40 ASSIGN | [40,revision:u64,paths:[1*2 b32]] | Только A; полностью заданный допустимый набор содержит канал доставки, а не patch |
| 41 ASSIGNED | [41,revision:u64,setDigest:b32,retainedPresent:bool] | Только B, на том же канале после приёма ASSIGN и установки receive readiness |
| 48 PROBE | [48,nonce:b32] | Любая роль, только data-ready Live path и bounded control reserve |
| 49 PROBE_REPLY | [49,nonce:b32] | Противоположная роль; exact outstanding probe/channel, не увеличивает credit |
| 62 ABORT | [62,code:u64] | Только authenticated accepted path; local cancel/expiry/violation/delivery failure, не graceful EOF |
| 63 REFUSE | [63,code:u64] | Initial: до Prepared либо отказ позднему PROPOSE после её expiry; continuation: отказ попытке, в том числе после ACCEPT до commit ASSIGN у B. После commit не отменяет назначение; никогда не доказывает неисполнение прежнего Open |

ABORT codes: 0 local cancellation, 1 authority/time expiry, 2 protocol violation,
3 local delivery failure. REFUSE codes: 0 unavailable, 1 unsupported profile,
2 resource limit. До проверки Kcont continuation получает только unavailable,
без различения missing handle/expired/неверный proof. Никаких свободных текстов,
stack, Name, key, IP или peer-selected recovery instructions в ERROR frame.
Злонамеренный peer сам способен прервать своё участие; REFUSE не ослабляет policy.

Поле role обязано совпадать с ролью peer на текущем end-to-end TLS (A=client, B=server), независимо от физического dial. Одной проверки MAC с общей Kcont недостаточно для выбора роли.

Уже проверенные schema и crypto не заменяют локальный state gate. Старый
accepted path допускает записи лишь пока его exact handle сохранён. Новое
максимальное revision само по себе не аннулирует retained path.

## 5.1. Допуск сообщений по фазам

Начальные S1/S2 описывают принимающую сторону B; S3 — получателя REQUEST.
В S4 приём ASSIGN описан для B, независимо от того, кто отправлял REQUEST.
Координатор A после проверенного REQUEST/ACCEPT выполняет свою половину §6:
он отправляет ASSIGN, а не ожидает ASSIGN от B. При REQUEST от B сторона A
сначала отправляет ACCEPT, затем назначение; при REQUEST от A — сначала
проверяет ACCEPT от B. Оба варианта сохраняют одну assignment authority.
ReceiveReady и PeerReady различаются; локальные состояния сторон могут
временно расходиться. Новое сообщение проверяется по exact channel/role,
а не только по состоянию Connection в целом.

| Состояние канала/записи | Может исходить | Допустимый приём → эффект |
|---|---|---|
| S1 сквозной TLS есть, PROPOSE нет | REFUSE(0 unavailable / 1 unsupported profile / 2 resource limit) | PROPOSE → проверка и reserve → OFFER (создаётся Prepared); после expiry прежней Prepared на её исходном канале — REFUSE(0) и закрытие, без новой записи |
| S2 Prepared (исходный канал живой) | OFFER; по byte-identical повтору PROPOSE — тот же OFFER | точный повтор PROPOSE → прежний OFFER; иной PROPOSE → закрытие канала без frame, Prepared не трогается; CONFIRM → см. state-таблицу (один Start / join) |
| S3 новый сквозной канал кандидата (REQUEST не принят) | REFUSE(0) coarse | CONTINUE_REQUEST: proof не пройден → REFUSE(0)+закрытие кандидата; пройден → S4. Никакого эффекта на существующие пути |
| S4 кандидат после ACCEPT, ASSIGN не применён | никаких самостоятельных frame; ответов нет до ASSIGN | ASSIGN (полное множество, revision>макс, paths∋pathId этого канала): применить → S5 и ASSIGNED; отказать попытке → REFUSE(0/2), набор путей не меняется, канал остаётся кандидатом до закрытия; иные frame на S4 → молчаливое закрытие кандидата |
| S5 принятый путь (receive-ready) | ABORT(0..3) = соединение-скоуп; B ставит ASSIGNED перед DATA в serial writer после commit; A передаёт DATA/ACK/EOF/PROBE только после проверки ASSIGNED | корректные кадры — по правилам потока; ASSIGN-повтор (byte-identical, канал жив) → прежний ASSIGNED; stale revision → discard без ответа; недопустимая аутентифицированная семантика → протокол-нарушение: локальный Stopping + best-effort ABORT(2) |
| S6 initial path после OPEN_READY | = S5 (implicit revision 1) | = S5 |
| S7 retired handle | ничего; незавершённые исходящие очереди сбрасываются | любые новые события на прежнем handle → discard без ответа и effects; уже принятые ранее bytes остаются в общем состоянии; повторные 2/3 на retired ACK-обязательств не создают |
| S8 Settled (соединение) | exact-повторы terminal control (2 с finalAccepted, 3); SETTLED_RECEIPT по 32 | проверенный 32 (query slot) → 34; 2/3 — как повторы; новые DATA/assignment/probes не допускаются: на retired handle discard, на ещё принятом канале недопустимая фазой аутентифицированная семантика — protocol violation; новых admissions нет |
| S9 Stopping | ABORT(0..3) best-effort по открытым принятым каналам | завершение уже допущенных операций без нового admission; новые кандидаты/ASSIGN — отклонение с закрытием |
| S10 Closed | ничего | всё discard; handle не создаёт нового соединения |

## 6. Последовательность открытия и продолжения

Исходное открытие: PROPOSE → OFFER → CONFIRM → OPEN_READY.
Instance signature внутри OFFER задана в
контракте Instance OFFER (§4.1): standard Ed25519
от exact current Instance signer на domain-separated hash(H0,CB). Она входит
в HMAC transcript proofB. Добавляет 66 bytes body без отдельного flight;
старый OFFER parser/evidence не считается квалификацией новой schema.
Новая schema/signature/proofB проверены вместе в
offer-exchange-probe (внешнее evidence: `offer-exchange-probe/README.md`): 20 случаев с настоящим
TLS и model admission. Это не actual JOIN/State/worker квалификация.
A резервирует собственный receive state и помечает MayHaveAdmitted ДО первой
попытки I/O CONFIRM либо разрешающего запуск ContinueOpen. Ошибка записи не
сбрасывает признак. B сохраняет Prepared до OFFER; после правильного CONFIRM единожды
Prepared→Starting, join существующего launch при повторе, затем Live/OPEN_READY.
Exact повтор PROPOSE на том же Prepared канале возвращает прежний OFFER без нового owner/nonce/reserve; другой PROPOSE закрывает канал без REFUSE, сохраняя Prepared до её срока. После expiry поздний PROPOSE на исходном канале получает REFUSE(0) и закрытие без новой записи. Повтор CONFIRM не восстанавливает retired initial handle. Сбой launch не возвращает Prepared. На B OPEN_READY идёт перед первым DATA
в serial writer. A не посылает DATA до проверки OPEN_READY.

Начальный pathId = SHA-256(ASCII `ardents-connection-path/1` || 0x00 ||
                         H0 || CB_initial || byte(0) || nonceA).
Он получает implicit initial assignment revision 1. A receive-ready перед
CONFIRM, B receive-ready перед OPEN_READY; идентификатор не передаётся relay.
Если initial outcome неизвестен, сохраняются исходные handle/Kcont/C0 и bounds.

Продолжение: REQUEST → ACCEPT → ASSIGN → ASSIGNED, с прежним data path при
его сохранности. Physical opener может быть B, но A остаётся assignment owner.
На одном новом канале выбирается ровно один requester; получатель не запускает
встречный REQUEST там же. Оба могут dial разные каналы в общем pending budget;
A сериализует победителя, остальные закрываются и joins. У A не более одного
pending ASSIGN; следующий фиксируется после результата либо retirement/Join
канала предыдущего. Это не отменяет возможный commit B при потерянном ответе.

Предложение локального resource contract — §3.9 основного проекта: не более
двух accepted и трёх всех Attachment slots, включая pending/retiring;
пересёкшиеся инициативы имеют не более двух pending. Slot возвращается после
Join, не по изменению label. Новая подготовка при двух accepted ждёт
retirement/Join одного из них. Поля wire дополнительных квот не предоставляют;
численные lifetime/rate allowances и policy интервалы ещё требуют выбора.

Для нового кандидата pathId = SHA-256(ASCII `ardents-connection-path/1` || 0x00 ||
                                    H0 || CB_new || byte(requestRole) || attemptNonce).
ACCEPT proof подтверждает retained authority/этот канал, не готовность DATA.
Для ContinueOpen B сначала допускает/joins тот же единственный launch, проверяет
Live и только затем отвечает ACCEPT. Для AttachLive Prepared/Starting не дают
нового Start. Settled на той же проверке возвращает SETTLED_RECEIPT и закрывает
bounded child после ответа, без ASSIGN. Missing state не создаётся.

ASSIGN paths отсортированы лексикографически по 32 bytes и различны.
Содержат текущий кандидат/accepted канал и максимум один иной exact retained
handle. Канал доставки должен быть точно известным аутентифицированным кандидатом/принятым путём. Другой handle необязателен: B сохраняет его только если он ещё локально принят. Отсутствующий либо незакреплённый handle не создаётся/не активируется по hash и не блокирует новый путь. Смена на единственный готовый путь также использует ASSIGN с более
высоким revision на этом пути. A резервирует/устанавливает receive-ready ДО
отправки. B применяет набор атомарно с Stop/authority и затем ASSIGNED.
setDigest = SHA-256(полного exact ASSIGN frame).
ASSIGNED.retainedPresent сообщает, сохранил ли B единственный другой путь
в момент применения этого назначения. При одном path в ASSIGN он обязан быть
false. A проверяет bool вместе с exact revision/setDigest/каналом: готовность
канала доставки не зависит от сохранности другого пути. False прекращает
локальный допуск другого пути; true не восстанавливает уже удалённый локально.
Это снимок результата commit, не обещание будущего здоровья пути. Exact повтор
ASSIGN возвращает прежний ASSIGNED, даже если позже retained path исчез; ни
ответ, ни его повтор не создаёт его заново. Новый путь всё равно обязан быть
локально живым и допустимым перед публикацией readiness.

Повтор того же revision допустим только при byte-identical ASSIGN и ещё
разрешённом exact канале. Возвращается прежний результат; conflict — violation.
Устаревшие ASSIGN/ASSIGNED игнорируются без ответа и не добавляют slots.
Новый revision не оборачивается.
После применения ASSIGN B ставит ASSIGNED перед DATA в serial writer и может
отправлять DATA на новом пути (A уже готов);
A начинает отправку только после ASSIGNED с теми же revision/setDigest/channel.
Утрата ASSIGNED не откатывает уже принятые DATA/ACK. Receipt гонка Live→Settled
заменяет pending admission на terminal outcome или отказ; поздний callback не
может снова опубликовать Data Ready. Подробный owner lifecycle остаётся общим.

Уточнение ACK publication при новом пути: после применения ASSIGN B, а после
получения валидного ASSIGNED A инициирует текущий общий ACK/receiveLimit/
finalAccepted snapshot своего входящего направления. Snapshot не ждёт новых
DATA: иначе утраченный credit-only update старого пути может оставить sender
без кредита навсегда. Он использует существующий ACK kind 2, не расширяет окно
и не требует отдельного запроса snapshot/ответа. Один новый admission инициирует одну coalesced
публикацию; exact повтор assignment не размножает очередь ACK. Send admission
сериализован с Stop/retirement; более новые изменения заменяют pending snapshot.
Settled отвечает SettledReceipt и не возвращается к data-path workers.
Обоснование и незакрытые scheduler/network проверки —
`switching-policy-proposal.md` §5 и `codec-probe/POLICY-README.md`.
## 7. DATA/ACK/EOF/terminal и ошибки

ACK acceptedOffset монотонно подтверждает непрерывный префикс. Нельзя ACK выше
реально отправленного/принятого sendEnd; устаревший допустимый ACK не откатывает
offset или credit. receiveLimit >= acceptedOffset и <= maxBytesDirection;
новое окно у каждого Attachment не создаётся. Peer ACK не доказывает Application
исполнение. Схема не требует ACK на ACK и не создаёт автоматическую response loop.

finalAccepted=true принимается отправителем только при своём FinalDeclared
и acceptedOffset == зафиксированному finalOffset. На получателе он возможен
после принятия всего префикса/EOF. Более поздний ACK с false не отменяет true.
Final acceptance сохраняется через замену пути. EOF не расходует data offset;
после uint64 overflow нет wrap. Отправитель DATA ровно совпадает с direction.
Каждое направление завершается EOF, в том числе EOF(0) при maxBytes=0.
EOF/ACK используют bounded control reserve, поэтому нулевой data credit их
не запрещает. Settled требует final ACK своей отправки, полной выдачи
входящих bytes/EOF и joined Application I/O; не требует ACK на собственный ACK.

SETTLED_RECEIPT удостоверяет сохранённые finals по обоим направлениям. Его
получатель сверяет собственный declared sendEnd/final и retained receive state;
не синтезирует отсутствующие bytes или локальный EOF по одной квитанции.
Retention/query rights и cleanup требуют отдельного принятого распределения бюджета N12 (GAP-R); до этого terminal path не считается готовым к реализации.
Нет ACK этой квитанции; утрата до срока оставляет честный Unconfirmed.

Ошибки framing или TLS обрывают exact канал. Недопустимая аутентифицированная
семантика от peer на accepted path — protocol violation с локальным Stop,
а не сигнал начать обход защиты. Неправильный proof на unaccepted candidate
закрывает candidate, не даёт незнакомцу убить существующий Connection.
Transport EOF не подтверждает logical EOF или отсутствие Application effects.
ABORT всегда Connection-scoped только на локально принятом exact канале;
на кандидате он закрывает попытку, на retired handle отбрасывается. Различие
локальных статусов между commit A и commit B не обеспечивает удалённый Stop:
при отсутствии пригодного пути peer ограничен собственными сроками и правами.

Отдельный контроль ресурса обязателен для числа frames, дешифрования, decode,
MAC/lookup, duplicate/probe rates и длительности partial reads. Размер одного
frame не ограничивает количество frames или суммарный pre-admission flood.

## 8. Предсказанные максимальные размеры для проверки encoder

Размеры body без prefix/TLS/Route, при максимальной длине uint64 и exact b32:
PROPOSE 340; C0 416; OFFER 518; CONFIRM/OPEN_READY 104;
CONTINUE_REQUEST 175; CONTINUE_ACCEPT 72; SETTLED_RECEIPT 90;
ASSIGN с двумя paths 81; ASSIGNED 47; PROBE/REPLY 37; ABORT/REFUSE 4;
DATA 16399 при payload 16384 и maximal non-overflow offset;
ACK 22; EOF 12. Frame добавляет 4 bytes. Исторический shape test (RESEARCH `connection-wire-draft.md` §10) проверял
OFFER 452 без Instance signature. Новый OFFER 518 body/522 framed отдельно
проверен в instance-offer-probe; OFFER parser/admission дополнительно проверен
в offer-exchange-probe с явными ограничениями окружения.
Это не измерение полной стоимости handover.

## 9. Граница доказательности и следующие проверки

Codec probe ранее проверял только DATA/ACK/EOF. Ни одна строка новой таблицы
не получает PASS автоматически. Нужны exact positive/negative vectors всех
сообщений, независимая проверка grammar/maxima, transcript domain/role/channel
negative controls, real TLS initial+continuation и replay/Stop interleavings.

Особенно проверять: C0 не соответствует PROPOSE; оба intents; actor B инициирует
канал, но не присваивает ASSIGN authority; missing handle; роль отражена;
другой requestDigest/routeBinding или неверная actual JOIN привязка; initial failure after launch; ACK-before-ASSIGNED;
late Ready после смены набора; устаревший retained handle; Settled-only query;
unsigned failure не превращается в доказательство NeverStarted.

Этот wire draft не выбирает public membership/authority или приватный Name
producer. Он использует принятый closed Route; способ построения конкретного
нового Attachment, бюджеты популяции и anonymity claim не следуют из grammar.
Внешние криптографические обязательства (Service key/Publication validation,
local permissions, Route joins) должны иметь действительных владельцев;
их нельзя реализовать тестовым trusted=true callback для принятия всего ядра.

Первичные основания повторно доступны 2026-09-21:
[TLS exporters, RFC 8446 §7.5](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5)
и [deterministic CBOR, RFC 8949 §4.2.1](https://www.rfc-editor.org/rfc/rfc8949.html#section-4.2.1).
Они задают используемые механизмы, а не доказывают безопасность данной
Ardents-композиции. Применяется обычный exporter после handshake, не early exporter.

## 10. Adoption и ограниченные задачи

Новый wire не совместим с нынешними Connection records. Одного объявления
ALPN недостаточно для принятия нового профиля: N07 фиксирует explicit
version/profile mapping и отказ старому peer. Основной путь для одноразового
dev-окружения — чистый deployment с новым Network/trust context и roots
по §5.1 проекта. Совместимость со старыми records и перенос живых операций
не требуются по умолчанию. Новые сообщения не подмешиваются в прежний профиль;
permissive parser и downgrade запрещены. При названной обязанности сохранить
старую identity/историю миграция готовится отдельной карточкой и сохраняет
её floors/authority. TCP↔QUIC continuation внутри нового профиля обязательно.

| Изменение | Зависимая задача в проекте | Что ещё должно быть определено или проверено |
|---|---|---|
| Exact codec, registry и auth transcript | N07 grammar + binding + включение профиля — отдельные результаты | Отказ старому профилю, настоящая capsule/JOIN binding, vectors всех shapes и фаз |
| Initial admission и retained consumer handoff | N08 | Реальный post-auth handoff; READY достижим до request DATA |
| Unknown initial outcome и ContinueOpen | N09 | Один retained owner/одна операция, потеря retained state без нового admission |
| Общие DATA/ACK/credit | N10 | Реальный reserve и один delivery owner; replay/offset invariants |
| Назначение и retirement path | N11 | State-authorized alternative N14/N15, atomic readiness, lost response/Join |
| EOF/Settled/retention | N12 | Finite terminal record/query budget и expiry; отсутствие ложного bilateral completion |
| Автоматическая смена | N13, отдельно от wire | Численные triggers/cost, admission и liveness; не превращать codec в selector |

Названные неизвестные блокируют свои изменения. Этот перенос документа
не выбирает их за Product Owner и не превращает все строки в одну Issue.
Критерии перехода к реализации и исполнимые карточки остаются в проекте,
а privacy-утверждения — в действующем threat model и §3.5 проекта.
