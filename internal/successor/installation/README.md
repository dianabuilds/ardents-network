# Installation and Replacement — проект нового владельца

Проект по действующим контрактам, проверенный по исходникам на
`dev@edaa76ce9db8afa04d5983980b2ea86f74279e11`. Первый Go Module реализует
portable admission canonical request и свежую связную авторизацию программы/поколения;
native initial provision имеет собственные lease/journals, fixed resources,
stopped selection и intent archive. Read-only check сверяет выбранные bytes,
account и roots без Release effects. Predecessor join, installed start и recovery
ещё не реализованы для successor transition; bounded initial stopped recovery
имеет отдельную command composition и actual stopped manager receipt, но полная
interruption matrix ещё не проверена. Полная native transaction не принята или квалифицирована.
Этот документ не выбирает implementation slice и не доказывает
установленную работоспособность. GitHub остаётся журналом выполнения.
Installation — самостоятельный владелец внутри семейства Software acceptance.

**Вопрос домена:** как безопасно установить, заменить и восстановить поколение?

**Входит:** транзакция установки/замены, журнал, фиксированные ресурсы поколения,
проверка разрешённых артефактов, завершение предшественника и восстановление
после прерванной операции в рамках действующего контракта.

**Не входит:** создание Release-разрешения, первичное Enrollment-доверие,
обычная очистка Job, выдача сетевых прав и автоматический откат истории.

**Состояние и завершение:** собственный журнал и владение поколением; новое
runtime-допущение следует за завершением требуемой транзакции и join
предшественника. Таймаут ожидания не считается завершением процесса.

**Соседи:** Enrollment предоставляет проверенный комплект; Release — разрешение
на точные байты; Execution использует разрешённый установленный артефакт и
владеет отдельным жизненным циклом worker.

**Где искать обязанности:** `internal/endpoint/installation`,
`internal/endpoint/replacement` и установочные command-адаптеры. Композиция
и системные механизмы не получают полномочий Release.

**Основа:** [Release и установка](../../../docs/technical/release-update-custody.md),
[карта доменов](../../../docs/development/domain-map.md).

## Контракт и владельцы

Приоритет имеют [ADR-0119](../../../docs/adr/0119-bind-protected-endpoint-generation-to-release.md),
[product scope](../../../docs/product/scope.md),
[threat model](../../../docs/security/threat-model.md),
[установочный handoff](../../../docs/technical/endpoint-service-runtime.md#selected-protected-installation-handoff)
и [confinement](../../../docs/technical/application-confinement.md).
Изоляция нового Route обязательна и для его зависимостей и тестов.

| Владелец | Что передаёт или решает | Чего передача не доказывает |
|---|---|---|
| Enrollment | Первичный pin, точный inventory, неизменяемый снимок файлов; отдельная загрузка самосогласованного кандидата для замены | Подписи Release, свежие floors, разрешение установки или запуска |
| Release | Свежие непрозрачные разрешения на точные байты программы и descriptor; собственная монотонная история | Владение каталогами, состояние manager, завершение процессов |
| Installation | Связность пары разрешений, локальные declarations, поколения, binding/selection, собственный журнал и транзакция | Network/Admission/Instance/Publication authority, готовность Service |
| Native adapters Installation | Реальные account/ownership/inode, fixed resources, unit/InvocationID, pin и join исходных scopes | Право принять неизвестную платформу или ослабить обязательную защиту |
| Execution | Отдельный допуск и lifetime конкретного квалифицированного worker | Право устанавливать, менять Release floors или восстанавливать установку |
| Command composition | Создание подлинных новых владельцев и перевод результата операции | Подмена доказательств данными CLI или сохранённым JSON |

Новый владелец не импортирует `internal/endpoint`, его `runtimeplan`, `worker`,
старые Enrollment/Release или их тестовые wrappers. Старые runtime consumers
не вызывают новый Installation. Прежние установленная история и живые roots
не усыновляются автоматически. Разрешение этого переноса не выбирает удаление
или переустановку прежней системы.

## Один владелец транзакции, отдельные доказательства

Установка сериализует мутации своей selection, поколений, fixed resources и
журнала под одной исключительной native lease. Release имеет собственную lease
и floors: это не общая транзакция. Отказ второй оценки или поздней установки
сохраняет уже записанные Release floors. Ошибка или отмена не откатывает доверие.

Граница между portable правилами и native эффектами проходит внутри
Implementation: связность разрешений, canonical records, phase ordering и
сборка неизменяемых байтов не требуют Linux build tag. Реальные UID/GID,
system manager, Unix peer credentials и cgroup pins требуют отдельных Linux
адаптеров. Другие ОС могут выполнять portable проверки; это не Windows
installation profile. Не создавать generic backend, экспортируемую матрицу
callbacks или пустые подpackages ради дерева каталогов.

Файлы группируются по законченной ответственности: admission запроса,
authentication поколения, binding/selection, journal/recovery, native ownership,
predecessor join и start barrier. Отдельный метод не требует отдельного файла.
При появлении реального отдельного Module его пакет одновременно получает
`doc.go`, Implementation, behavior tests, точные imports и production caller.

## Наблюдаемые этапы

1. **Admission.** Canonical request не более 64 KiB; неизменённая схема
   `ardents-endpoint-installation-request-v1`, UTC reference time, согласованные
   headless-v2/Source-v1 declarations, distinct declared Source families,
   абсолютные непересекающиеся roots. Reader не получает Publisher inputs.
   Синтаксис не доказывает независимость операторов. Нет private material в
   поколении; реальные credentials и permissions принадлежат их владельцам.
2. **Byte snapshot.** Первая установка требует независимый manifest pin.
   Замена отказывает при supplied initial pin и загружает самосогласованный
   кандидат без утверждения Enrollment-доверия. Все последующие проверки
   используют одни сохранённые байты, а не повторное чтение mutable bundle.
3. **Fresh authorization.** Две оценки через подлинный новый Release на одном
   наборе metadata, Local и reference time. Программа —
   `ardents/linux-amd64/endpoint`; поколение —
   `ardents/linux-amd64/protected-endpoint`. Требуются opaque authorizations
   даже при `no-update`. Сравниваются private `AcceptedDecision`, равные
   Targets version/digest, release identity/version, platform/architecture,
   environment/Network и reference time. Descriptor не более 16 KiB и его
   точные девять файлов должны связать программу и все actual resource bytes.
4. **Generation preparation.** До выбора поколения проверить реальные account
   и immutable/mutable root identities; собрать bound plans, fixed unit output,
   target observations и digests в local binding. Root-owned direct generation
   directory определяется digest descriptor. Selection отдельно связывает
   descriptor и binding. Binding не сериализует opaque authority.
5. **Durable intent and staging.** Сохранить original failure/phase и birth
   identities до записи принадлежащих установке объектов; sync полного
   поколения предшествует изменениям fixed resources. Первая установка не
   принимает уже существующие writable roots/account/resources без доказанной
   принадлежности. Replacement сохраняет complete predecessor generation.
6. **Predecessor termination.** Для замены проверить unit/account/program и
   actual MainPID/InvocationID, закрепить исходные scopes, повторно проверить
   invocation и остановить fixed Endpoint и обе activation sockets. Требуются
   join исходных pins, stopped unit observations и отсутствие remaining worker
   scopes до fixed writes. Stop exit 0, EOF или timeout не являются join.
7. **Publication.** При остановленных scopes заменить только доказанно owned
   direct fixed files; records фиксируют inode/mode/group и old/new digests
   до truncation. Проверить bytes, опубликовать selection, reload и проверить
   actual loaded fragments/properties. Filesystem и manager не атомарны.
   `provision` заканчивается `installed-stopped`, без implicit start.
8. **Installed start.** Проверить selected bytes, mutable roots, собственные
   UID/GID/executable/arguments/cgroup и actual unit/MainPID/InvocationID.
   При successor transition root-private start guard и записанный birth identity
   completion socket удерживают barrier до durable archive и exact invocation
   acknowledgment. Отсутствие cursor не разрешает runtime. После ACK ошибка
   cleanup сохраняет `installed-started-recovery-required`, а не сообщает,
   что процесс никогда не был допущен.
9. **Recovery.** Только явная операция над exact owned intent/generations и
   inode records с двумя новыми floor-compatible разрешениями. Record заново
   синхронизируется до изменения ресурса: видимость не доказывает durability.
   Foreign bytes/inode, missing ownership, conflicting binding или неполные
   floors дают refusal/repair-required. Нет automatic rollback или root reset.

Замена требует строго более нового generation release даже при неизменной
программе. Равная версия с другим digest отказывает. Environment, Network и
durable root identities не меняются. Recovery может завершить только текущий
разрешённый intent; retained predecessor сам по себе не даёт rollback authority.
Read-only `installation-check` не открывает Release history, не ремонтирует
файлы и возвращает только integrity observation. Restart сохраняет floors и
не обещает Publication/Connection continuity.

Начальная native transaction отдельно сохраняет `directory-creations` в своём
generation journal. После exclusive mkdir и sync исходного закрытого каталога
запись связывает generation, путь, device/inode и исходные и требуемые mode/GID;
sync записи и journal предшествует chown/chmod. Перевод worker-root в read-only
доступ имеет отдельную запись на том же исходном inode до изменения mode.
Отмена удерживает записи и каталог в достигнутом состоянии. Начальная операция
не усыновляет оставшуюся запись при повторном вызове. Эти данные дают native
recovery необходимую provenance, но не заменяют свежие Release proofs и не
означают, что полное recovery принято или квалифицировано.

`ardents-next installation recover-initial <root> <UTC-reference-time>` удерживает
existing Installation writer lease до физического закрытия. Открытие проверяет
original intent, preparation/generation journals, account/roots и complete
file/directory birth inventories до Release effects. Composition загружает
Candidate без initial pin и открывает только complete retained Release history;
две новые proofs должны разрешать те же program/generation bytes и identities
при новом времени проверки. Original request, plans, unit и binding не
пересобираются с новым reference time. Все actual resource prefixes проверяются
до первого ремонта; visible birth records повторно sync до изменения записанных
объектов. Копии opaque handle разделяют один admission latch и lease; завершение
использует только original opening context и не может продлить его новым caller.
Операция оставляет установку stopped, сохраняет first failures в archive и
отказывает при live scopes, unknown ownership или недостающих birth records.
Отсутствие pending cursor само по себе не разрешает работу: explicit retry после
архивирования проверяет exact selected completed intent и получает fresh proofs.
Read-only Check также отказывает при pending recovery failure. Старые roots без
directory birth evidence сохраняются; новый recovery не усыновляет их молча.
Все explicit retries относятся к одному immutable intent и сохраняют его первую
recovery failure, включая уже архивированную. Поздний отказ возвращается caller
как текущая ошибка; запись первой ошибки заново sync перед удержанием active
copy, чтобы последующее архивирование не конфликтовало с исходным archive.
Один native recovery lifetime владеет frozen observations, журналами и ремонтом
на исходных inode; его cohesive implementation держит эти проверки рядом, без
generic callbacks или отдельного пакета для каждого syscall.

Изолированный Ubuntu24/systemd255 receipt проверяет genuine initial provision,
recovery по exact selected archive после удаления cursor, same-inode authorized
prefix repair, foreign-prefix refusal и сохранение first failure при успешном
retry. Selection и Release floors не изменились; Endpoint и worker не запускались.
Это controlled filesystem faults, не process crash или power-loss qualification.
Native reader сохраняет joined filesystem errors; отсутствие optional record
распознаётся через error tree, а foreign cursor не разрешает archive fallback.
После проверки исходные процессы и cgroup тестового manager физически завершены;
его outer stop сохранил timeout result, поэтому clean manager shutdown не заявлен.

Отдельный native subprocess control проверяет реальный SIGKILL после generation
staging и fixed-resource write: исходный child joined, kernel writer lease
освобождена, исходные records/bytes/inodes сохранены, повторный initial owner
отказывает при существующем root. Это filesystem-mechanism fixture без
Enrollment/Release/account/manager authority; он не доказывает успешное recovery
или полную actual-manager interruption matrix и power-loss qualification.

Отдельный actual-manager receipt на Ubuntu24/systemd255 использует настоящий
новый `provision` с signed-byte fixture. Внешний syscall controller завершает
исходный процесс через SIGKILL после успешного sync `0006.json` и его journal
directory, до manager reload. Исходные threads joined и writer lease освобождена;
pending Check отказывает. Новый `recover-initial` получает две fresh proofs,
возвращает `installed-recovered-stopped`, архивирует exact intent и сохраняет
все десять fixed bytes/inodes/access, selection и Release floors. Read-only Check
после этого возвращает `local-integrity-verified`. Endpoint и worker не запускались.
Второй receipt в отдельном свежем manager прерывает настоящий `provision` после
успешного sync `0007.json` и journal directory: reload и typed stopped observation
уже состоялись, но intent ещё не архивирован. Исходные threads joined, lease
освобождена; pending Check отказывает. Genuine fresh-proof recovery сохраняет
исходный `0007.json`, завершает exact intent и возвращает stopped result с
успешным read-only Check. Все десять fixed bytes/inodes/access, selection и floors
не меняются внутри этой отдельной установки. Совпадение generation digest между
receipts не объединяет их native identities или истории.
Это два process-crash boundaries, не полная interruption matrix, power-loss,
independent builders/custody или installed qualification. Исходные процессы и
cgroup отдельного test-manager отсутствуют после outer stop с retained timeout
result; clean manager shutdown не заявлен.

## Недостающие seams до реализации

Новый Enrollment экспортирует `Verify` и opaque `Bundle` с первичной provenance,
а также отдельный `ReadCandidate` и opaque `Candidate` с теми же bounded reads
и inventory проверками, но без independent-pin или running-executable claim.
Новый Release имеет `Evaluate`, `CurrentFloors(ctx)` и immutable
`Authorization.AcceptedDecision` и `OpenRetained`, который не создаёт пустую
историю доверия и требует сохранённые floors всех metadata roles. Новый
`installation.AuthenticateInitial` потребляет настоящий pinned Bundle, а
`AuthenticateCandidate` — отдельный Candidate и уже полные Release floors.
Обе операции замораживают те же metadata/local/reference inputs, проверяют
private proofs и удерживают exact generation bytes; отказ второй оценки
не возвращает floors. Настоящие `ardents-next installation authenticate-initial`
и `authenticate-candidate` выполняют эти операции, закрывают Release owner и
возвращают только `authenticated-generation`. Это не `installed` и не readiness.
До successor transition ещё требуется собственная installed binding/history
continuity и actual native transaction; одна полная Release history их не заменяет.
Candidate не возвращает `Bundle` и не принимает искусственно вычисленный
«первичный pin». Read-only классификация inventory на metadata и static files
остаётся у Enrollment, а fixed metadata URLs и Release inputs собирает
application composition. Это не импорт Release в Enrollment. Command и
Installation не должны поддерживать разные списки исключений.

Новый Installation проверяет canonical request и вложенные Headless/Source
declarations через `DecodeRequest`, сохраняя текущие schemas, порядок полей
и ограничения байтов. Эти declarations приватны внутри текущего Module:
отдельный пакет без самостоятельного runtime consumer не создаётся.
Root declarations выбранного Linux-профиля используют одну POSIX grammar
на всех hosts; это не native filesystem проверка или Windows installation.
Нормализованные дубликаты public keys, invalid threshold и перекрывающиеся
mutable roots отказывают до эффектов. `authenticate-initial --request <file>`
и `authenticate-candidate --request <file>` потребляют проверенные
bundle/pin/history/reference inputs через настоящие Enrollment/Release.
Декодирование не открывает State, Source credentials или grants; bounded
command-file reading не доказывает root-owned custody. Новая runtime
composition начальной остановленной установки, read-only inspection и bounded
initial stopped recovery имеют новые consumers; installed start и successor
transition/recovery пока отсутствуют. Их компонентные проверки не доказывают принятую native установку.
Старый `internal/endpoint/runtimeplan` остаётся независимым владельцем
predecessor consumers; его импорт в новый путь запрещён.

Есть отдельное противоречие platform admission: current confinement owner
указывает Ubuntu24/systemd255, installation owner — пары 22/249 и 24/255;
Product Owner в [#359](https://github.com/dianabuilds/ardents-network/issues/359)
выбрал фактически доказанные capabilities/lifetime semantics вместо версий.
Ранее broad rewrite этой security boundary отклонён automatic review и не
интегрирован. Новое имя домена не разрешает обойти отказ. Portable extraction и
byte authorization продолжаются независимо; изменение принимающих native
profiles требует конкретного разрешённого ремонта с causal protection tests.
Unknown/missing protection никогда не является принимающим fallback.

## Source inventory: что разобрать, что оставить

Исходники ниже — named provenance в текущем checkout, не dependencies.

| Прежняя ответственность | Проверенный source | Решение для нового владельца |
|---|---|---|
| Coherent fresh pair | `internal/endpoint/installation/release_authentication.go` | Перепроверить по новому Enrollment/Release; private proofs вместо mutable public Decision |
| Existing trust continuity | `successor_authentication_linux.go` | Portable rule; actual complete floors и binding constraints, никакого initial-pin bootstrap |
| Request and declaration consistency | `request.go`, `internal/endpoint/runtimeplan` | Сохранить grammar/limits; отделить pure declarations от acquisition/credentials |
| Frozen generation assembly | `generation_assembly.go`, `unit_rendering.go` | Portable deterministic bytes; actual inode/account facts проверяются native owner |
| Ownership and journal writes | `generation_ownership_linux.go`, `resource_creation_linux.go`, `resource_replacement_linux.go` | Native identity/durability; canonical records и переходы отдельно от syscalls |
| Original process termination | `predecessor_join_linux.go` | Exact invocation + original pins, no replacement-generation completion |
| Selection/reload/start/ACK | `transition_finishing_linux.go`, `start_guard_linux.go`, `start_completion_linux.go` | Самостоятельный transition lifetime, retained errors и explicit barrier |
| Ordinary Job/worker lifecycle | `internal/endpoint/worker` | Оставить отдельному новому Execution; installation не получает fake qualification через wrapper |
| Runtime authority and Service | Endpoint composition | Оставить новым Network/Admission/Publication/Instance/Connection consumers |

## Приёмка и execution profiles

Тесты portable rules выполняются на Windows/Linux без `_linux_test.go`.
Тесты actual UID/inode, systemd, cgroup pins и Unix completion credentials
имеют Linux prerequisite с указанной причиной. Missing selected prerequisite
— invalid environment, не успешный skip. Byte-backed manager fixtures
доказывают только parser/state ordering; они не дают positive installed receipt.

Публичный consumer должен использовать genuine новый Enrollment, Release и
Installation. Проверки покрывают initial и successor, changed resource при
unchanged executable, `no-update` retry, второй target refusal после commit
floors, mutation isolation, original cancellation, lost/uncertain floors,
substitution/partial inventory, conflicting declarations и native identities.
Mechanical removal controls должны воспроизводить конкретный пропущенный
guard; независимые canonical fixtures не строятся production codec.

Для journal/recovery нужны causal отказы и crash/reopen на каждой фазе:
birth record, staged files/directories, pre/post fixed copies, selection,
reload, start observation, archive, ACK и post-ACK cleanup. Сохраняются первая
ошибка, exact owned bytes и фактические unfinished processes. Процессный crash
не доказывает power-loss или storage-hardware qualification.

Завершение требует полного регресса затронутых новых доменов, обычных
`make quick-check`/`make check`, selected native checks, source-matched receipts,
scoped commit/push и проверенной `dev`. Полная installed acceptance отдельно
требует двух отдельных admitted system managers/Endpoint principals, public
commands, actual containment/empty scopes и publish/link/read/refresh/withdraw/
restart на TCP/TLS и QUIC. Без новых Execution/Publication/Instance/Connection
эти положительные сценарии остаются недоказанными. Нельзя закрыть Installation
или всю цель Route по одной только паре разрешений или остановленной установке.
