# Installation and Replacement — проект нового владельца

Проект по действующим контрактам, проверенный по исходникам на
`dev@edaa76ce9db8afa04d5983980b2ea86f74279e11`. Первый Go Module реализует
portable admission canonical request и свежую связную авторизацию программы/поколения;
native initial provision имеет собственные lease/journals, fixed resources,
stopped selection и intent archive. Read-only check сверяет выбранные bytes,
account и roots без Release effects. Публичный successor lifetime теперь соединяет
fresh proofs, staging, original predecessor join, replacement/selection и
barrier-controlled попытку старта. Настоящий non-root consumer проверен через actual
manager, Root archival/ACK и bounded Source/permission bootstrap; terminal guard
recovery и reload-prefix recovery имеют отдельные actual-manager receipts. Три
process cuts проверяют восстановление до reload, после reload до записи phase7
и после durable phase7 до Start; они сохраняют original journal, fixed bytes и
Release floors. Восстановленный Endpoint достигает permission-pending, не полной
Service readiness. Полная successor interruption
matrix и installed Service acceptance остаются недоказанными; bounded initial stopped recovery
имеет отдельную command composition и actual stopped manager receipt, но полная
interruption matrix ещё не проверена. Полная native transaction не принята или квалифицирована.
Этот документ не выбирает implementation slice и не доказывает
установленную работоспособность. GitHub остаётся журналом выполнения.
Installation — самостоятельный владелец внутри семейства Software acceptance.


## Компоновка Implementation

`directory` удерживает original caller и закрытую карту исходных inode одной
последовательности создания mutable-каталогов. Initial preparation передаёт
допущенные пути/account, получает только проверенные detached device/inode
facts и повторно наблюдает тот же Creation. Независимая installed inspection
использует native property check без acquisition creation provenance. Root
сохраняет NSS/request/Release, journal/lease, phase/effect admission и общие
trusted-root/sync helpers для fixed resources и recovery; чужой каталог не
принимается из публичной map или похожих UID/mode.

`request` отдельно владеет canonical request-v1, Headless-v2 и Source-v1
декларациями, их связностью и чтением direct root-owned исходного файла.
`Document` выдаёт detached copies; `Origin` скрывает исходные path/digest/inode
и независимо проверяет прежние bytes/access перед эффектами. Корень сохраняет
совместимый `Request` consumer, Installation admission, Release proofs и
транзакции; происхождение запроса не выдаёт ни одного из этих прав.
`root_ownership_linux.go` сохраняет отдельно проверку trusted root-owned paths
для lease, staging и inspection: она не является request provenance.


`unit/endpoint_template.go` проверяет closed template и его command paths до
write-root admission, удерживает оригинальные immutable bytes и формирует unit
из отдельно допущенных путей. Корневой `unit_configuration.go` собирает выбор
writable roots, защиту immutable/Release roots и один expected Configuration
для rendering и actual-manager predicates. Template, first-refusal order и
write policy проверяются отдельными byte/refusal oracles; rendered bytes не
выдают Release, manager, process или startup authority.

Корень сохраняет Installation admission и владельцев транзакций. Portable
`generation_binding.go` собирает canonical immutable generation и замораживает
его closed inventory перед native staging: binding, digests и detached bytes
проверяются рядом с независимым чтением и проверкой того же closed binding.
`inspection.go` сохраняет отдельный публичный read-only Check с собственным
ограничением времени. Здесь же общий
`recoverBoundGeneration` восстанавливает exact bound bytes по свежим proofs
для initial и successor recovery; их intent admission, phase и native lifetime
остаются у отдельных владельцев восстановления. `generation_authentication.go`
собирает fresh-pair admission, complete Release floors и ограничения successor
continuity; сохранённые local facts по-прежнему не создают private proofs или
native custody. `generation_authentication_linux.go` сохраняет private
fresh-proof composition установленного successor только для его реального
native consumer; проверки исходных floors предшествуют обеим fresh evaluations.
`intent_archive_linux.go` удерживает общую exact intent custody,
copy/sync/removal; initial и successor completion допускаются отдельными checks.
Эти владельцы следуют ответственности и реальным callers, сохраняя
принимающий контракт. `installation_transaction_linux.go`
собирает original filesystem custody одного private `installationTransaction`:
lease, borrowed parents/containers, journal, generation/Prefix/Snapshot,
fixed-resource observations, intent и barrier, общий observe и joined close.
`generation_staging_linux.go` содержит generation birth/write/failure,
seal/access и проверку original generation-file provenance при recovery.
Это один владелец транзакции: проверка всех original handles и порядок их
закрытия должны оставаться под исходным lease. Разделение файлов следует
разным причинам меняться; отдельный phase package или shared lease не возникает.
`request.go` содержит корневой Request consumer; `request_test.go` проверяет
его декларации и исходную файловую custody, оставляя механизм в `request`.
`platform_refusal_other.go` собирает единый no-effect native отказ на остальных
платформах; compilation не расширяет поддерживаемую installation platform.
`initial_preparation_linux.go` — initial
request acquisition, preflight, account, preparation, fixed-resource publication и публикация stopped selection;
`fixed_resource_creation_linux.go` — recorded birth, same-inode access promotion и retained fixed-file observations; `successor_replacement_linux.go` — original replacement
records, допуск access/phase и mutation исходного fixed file, включая selection;
`successor_transition_linux.go` — successor lifecycle, intent
и ordering; `installed_inspection_linux.go` — leased inspection и retained private
record durability. Грамматика completion frame принадлежит `completion/frame.go`: один Encoder
и validator обслуживают root producer и независимый startup peer. Native
`start_barrier_linux.go` отдельно допускает Installation selection; frame не
получает authority из schema или digest. Отдельные корневые frame-файлы удалены;
точный byte oracle и отказы сохранены у completion и native barrier.
Read-only physical files остаются в `installed_files_linux.go`:
их использует и non-root startup, который не получает writer lease.

Такое объединение следует state/lifecycle ownership: отдельный phase helper
не получает самостоятельного владельца или пакета. Файл может быть длинным,
если для понимания одной операции важны её инварианты и весь cleanup. Разные
authority и physical lifetimes остаются отдельными.

`successor_recovery_linux.go` удерживает один независимо открытый recovery
lifetime: admission original intent/journal, private writing prefix или sealed
candidate, fresh-proof continuation, terminal removal и общий physical close.
Методы этого владельца собраны вместе вместо отдельных файлов по состояниям
writing/staged/completion. Каждая ветвь сохраняет свой exact journal и
pre-effect checks; объединение не делает detached binding авторизацией и не
сливает Prefix, Snapshot или original process custody. Связанные native тесты
собраны в `successor_recovery_native_linux_test.go`, portable public-handle
тесты остаются в `successor_recovery_test.go`. Более длинный файл позволяет
проверить dispatch и retained cleanup одного владельца в одном месте; отдельные
physical Modules и successor creation lifetime сохраняют свои причины меняться.


`unit/` отдельно владеет pure fixed Endpoint/activation contract: typed
configuration, executable/argv, invocation и fresh-stopped/quiescent/running
различия. Он получает detached ожидаемые пути и настоящие typed observations
через Interface; сам не наблюдает manager, не выбирает writable roots, не имеет
Release proofs, lease, process/cgroup custody или права stop/start. Корневой
`manager_binding_linux.go` собирает request write-root admission, projection,
совместимые классы ошибок и actual manager/activation observations, проверяемые
через этот fixed-unit Interface. Общие проверки stopped manager не принадлежат
initial selection: их используют initial, predecessor, candidate и recovery
lifetimes. Они собраны рядом с manager binding. Публикация initial selection,
reload и stopped completion находятся рядом с состоянием и cleanup
`initialPreparation`; отдельный файл по этой фазе удалён. Независимый byte/order
тест selection находится у `installationTransaction` в staging-тестах, поскольку он
проверяет original stage custody, а не успешную manager transaction.
Корень не дублирует fixed-unit policy. `systemd/` по-прежнему
владеет физическими manager observations и joined subprocess. Перенос policy
не расширяет native platform admission и не квалифицирует installed runtime.

`systemd.EndpointReference` удерживает fixed Endpoint unit на одном original
private system-bus соединении: это препятствует сборке inactive unit и загрузке
candidate configuration до явного reload. `installedPredecessor` получает
reference до наблюдения конфигурации и fixed mutation, удерживает её через
replacement/reload и закрывает после original physical join. Потеря соединения
или смена manager identity отказывает; reconnect не создаёт новую custody.
`systemd` ограничивает auth и scalar message bytes до библиотечного decoder,
прерывает и joins original transport I/O. Release proofs, phase admission и
expected configuration остаются решениями корня Installation.

## Внутренняя структура

Корневой пакет владеет Installation-решениями и последовательностью операций:
проверкой запроса и пары разрешений, поколением, установкой, восстановлением и
заменой. Системные наблюдения не получают эти полномочия при выделении в Module.

`Successor` удерживает исходный caller, writer lease и единственный completion
latch, общий для копий handle. `OpenSuccessor` проверяет native request и
installed custody до Candidate/OpenRetained effects. `Complete` сам получает
две fresh proofs через переданный Release verifier и проверяет continuity
до evaluation, затем выполняет одну replacement/start последовательность.
`ardents-next installation upgrade-installed --request <file>` — настоящий
consumer этого Interface; его Release lease живёт до физического Close.
Двухминутный replacement bound включает acquisition, но отмена не завершает
cleanup: если original work или quiescence не доказаны, Close удерживает writer
и ожидает реальные observations без нового Stop/ACK/admission. Полный ACK
сохраняет `installed-started-recovery-required` и ненулевой exit при поздней
ошибке, включая закрытие отдельного Release owner. Такой исход не означает
Service readiness. Настоящий новый consumer в `ardents-next` независимо допускает
non-root startup и bounded Source/permission bootstrap через новых владельцев.
Private Publication/Connection и полная installed acceptance остаются отдельными.

`successor.go` владеет публичным one-use lifetime, `successor_transition_linux.go` — его
native последовательностью и физическим закрытием. `candidate_linux.go`
удерживает ресурсы одной попытки fixed-unit start: original proc/cgroup custody,
quiescence и irreversible ACK latch. Эти ресурсы изменяются независимо от
fresh-proof/staging/selection решений в `successor_transition_linux.go`, поэтому
их Implementation отделён в корневом пакете без нового package или импорта.

`SuccessorRecovery` и `installation recover-installed <root> <UTC-reference>`
восстанавливают завершение уже запущенного exact successor. Открытие удерживает
writer до Candidate/OpenRetained effects, проверяет completed successor intent,
initial preparation, обе sealed generations, candidate directory birth,
все replacement records и три removal intentions. Независимые original
proc/cgroup pins и повторные actual manager observations сверяют сохранённые
MainPID/InvocationID; detached start record не заменяет их. `Complete` самостоятельно
получает две fresh proofs через retained verifier после повторного наблюдения
custody и current floors на исходном caller; готовый `Authorization` не принимается.
Proofs
должны совпасть с original target bytes/identities при новом reference time.
После resync provenance recovery удаляет только оставшийся original guard
с recorded inode/access/timestamps и архивирует точную первую transition failure
до удаления active copy. Она не повторяет ACK, Start или Stop. Существующий
runtime не получает новые права из recovery receipt. Shared one-use handle
сохраняет исходный двухминутный bound и закрывает только свои observations.

Отдельный [actual-manager receipt](https://github.com/dianabuilds/ardents-network/issues/506#issuecomment-6070342817)
проверяет crash после полного original ACK и durable трёх removal records,
перед unlink guard. Fresh-proof recovery удаляет только original guard,
сохраняет ту же running invocation и Release floors. Более ранние archive/ACK
prefixes дают отказ без mutation. Для terminal trial внешний test-only gate
удерживает настоящие fixture Source/clock/Candidate до recovery; production
artifacts и bounds не изменены. Исходный Root SIGKILL остаётся native fixture
FAIL, а outer manager timeout — отдельной ошибкой. Это не passing qualification
profile, полная interruption matrix или installed Service acceptance.

Отдельный pending путь допускает exact complete staged prefix до первой fixed
mutation: original selection/fixed bytes должны остаться predecessor, оба поколения
должны быть complete sealed, а journal содержать два phase records, candidate directory birth
и, необязательно, exact `0003.json` replacement intention. После этой intention
допускается original `replacements` group: пустая либо exact prefix девяти
fixed-resource records в порядке pathname, до первой mutation. Каждый record
связывает actual unchanged preimage inode/access и оба точных digest; разрыв
prefix, selection record, лишний group или более поздняя phase отказывает.
Installation проверяет schemas и prefix; journal самостоятельно удерживает
исходный directory/file inventory, а не фабрикует creation custody. После fresh
proofs все records и directory links, включая пустую group, синхронизируются
через исходные descriptors до predecessor effects. Intention и records сами
не разрешают mutation. Он удерживает независимый Snapshot и вновь открытый
original journal, а не fabrication creation Owner. После двух fresh proofs для
original candidate bytes и resync intent/records/generation он отдельно закрепляет
actual predecessor, выполняет original stop/join и существующую replacement,
selection/reload и guarded start sequence. До этой admission Close закрывает
только observations; после неё удерживает physical work до join. Полный ACK
сохраняет post-acceptance result при late failure. Presence не заменяет ни одну
из этих проверок. Pending start, first failures вне bound generation-writing
grammar и более ранние неподдержанные cleanup prefixes пока дают
`repair-required`; positive actual-manager pending recovery ещё не подтверждена.
Настоящий принимающий runtime уже имеет bounded startup consumer. Native filesystem tests проверяют механизм и
causal refusals, не успешную installed recovery.


До selection publication pending recovery также допускает interrupted fixed-copy
prefix на recorded original inodes: complete candidate files в pathname order,
не более одной пустой/частичной old-or-candidate копии, затем complete predecessor
files. Любое отличие от predecessor требует всех девяти original replacement
records. Complete preimages берутся из independently sealed predecessor; actual
current bytes/ctime сохраняются отдельно и не подменяются old bytes. Foreign
bytes, suffix, другой inode/access, второй torn файл или поздний candidate после
old boundary отказывают. При уже изменённых bytes opening отдельно подтверждает
actual stopped manager/activation units и empty scopes до Release composition;
live predecessor отвергается без Stop. Повторные quiescence checks, две fresh
proofs и original record/directory resync остаются обязательными перед repair.
Exact `0004.json` (`fixed-resources-replaced`) также допускается до selection:
все девять original records и complete candidate bytes обязательны. Даже если
static bytes двух поколений совпали, эта phase требует actual quiescence до
Release composition. С этой durable completion actual loaded configuration
может точно совпадать с complete predecessor либо candidate: после смерти
исходного caller новый manager reference может загрузить уже заменённый unit.
Все девять complete candidate images и original records проверяются до этого
match; pending selection не становится authority. До completion этот путь
по-прежнему требует predecessor configuration; `0007` требует только candidate.
После fresh proofs и повторных observations исходный
completion record синхронизируется повторно без перезаписи или смены inode.
Missing intention, incomplete inventory/bytes, first error и selection phase
отказывают. Эти filesystem механизмы не доказывают positive managed startup/recovery.

Pending `0005.json` (`publishing-selection`) отдельно допускает old selection до
его record либо old/empty/old-prefix/candidate-prefix/complete candidate на
recorded original inode. Selection record следует после всех девяти fixed
records и связывает canonical complete Previous/Candidate из original intent,
а не digest текущей torn копии. Все fixed files к этой фазе обязаны совпасть
с complete candidate. Opening читает pending selection как bounded physical
image, не использует его как trusted generation pointer; до Release effects
проверяет exact phase/record/inode/access/bytes и actual quiescence. Fresh proofs
и original provenance resync precede repair; original publishing intention
resync-ится без перезаписи. Foreign bytes, missing selection record при
изменённых bytes, wrong digest/inode, early selection record и missing fixed
records отказывают. start/ACK prefixes пока не допускаются
этим recovery prefix; filesystem tests не являются managed startup receipt.

`installed_inspection_linux.go` владеет physical resync/copy/remove исходных private
records под Installation writer. Initial и terminal successor recovery
используют один механизм, сохраняя отдельные intent schemas, phase admission,
proofs и process policy. Recovery не фабрикует creation Owner или новый journal.

`fixedfile/` — Module исходного изменяемого файла. Он самостоятельно открывает
родителя и leaf, удерживает их исходные дескрипторы, создаёт exclusive пустой
inode либо проверяет допустимый old/candidate prefix на прежнем inode. После
journal I/O он повторно проверяет parent, inode, access, bytes/ctime, выполняет
одну запись и sync и физически закрывает свои handles с исходной отменой и
первой ошибкой. Close сохраняет residue; новый контекст не возобновляет owner.
Installation оставляет у себя допустимые пути, bytes/modes, Release proofs,
схемы и durable journal admission, исходную lease, predecessor join и selection.
Создание fixed resources/selection и recorded replacement используют этот же
Module. Отдельные дескрипторы не делят live root с транзакцией и не дают
полномочий установки, восстановления или старта. Portable prefix grammar
находится рядом с механизмом, который её потребляет; native-проверки вызывают
тот же Create/Replace/Commit/Close Interface, что и Installation.

`cgroup/` — отдельный Module исходного kernel lifetime. Он владеет ограниченным
inventory Installation scopes и исходными дескрипторами `cgroup.events`, проверяет
настоящую cgroup2, наблюдает join и закрывает свои дескрипторы. Predecessor owner
сохраняет один `cgroup.Lifetime`; initial recovery отдельно вызывает
`cgroup.ObserveEmpty`. После попытки start `RetainStarted` сохраняет partial
custody исходного Endpoint даже при отказе последующего worker inventory.
Ненулевой lifetime с ошибкой требует join перед Close; он не разрешает запуск.
Отмена исходного запроса не отменяет физическую очистку. Join Endpoint не
доказывает пустоту остальных scopes: отдельный живой worker сохраняет отказ
`ObserveEmpty`. Ни pathname, ни пустой обычный каталог не доказывают join.
Installation сохраняет свою lease, actual process/InvocationID, решение о stop
и первую ошибку. Module не импортирует родителя и не разделяет с ним live root.

Portable scope grammar и native kernel adapter находятся внутри этого Module;
его native tests входят в общий профиль Installation. Такое выделение следует
собственному ресурсу и правилу завершения, а не отдельному шагу транзакции.

`systemd/` — отдельный Module типизированных наблюдений фиксированного Endpoint
и двух activation sockets. Он владеет ограниченным чтением настоящего system bus,
проверкой JSON/variant grammar, canonical manager version, fixed unit/instance
inventory и исходной отменой/физическим join subprocess. Закрытые `Reload` и
`Start` и `Stop` выполняют только daemon-reload, start фиксированного Endpoint
и stop фиксированных sockets/Endpoint.
Installation допускает каждый эффект; результат команды не заменяет kernel
join предшественника и не разрешает возврат lease или process pins. Caller не выбирает
произвольный объект, метод, bus, executable или environment. Правила ожидаемой
конфигурации и поколения, известные protection profiles, MainPID/InvocationID и
отличие initial/quiescent/running остаются у Installation в `manager_binding_linux.go`.
Типизированные наблюдения сами по себе не дают process или startup authority.

`journal/` — Module исходных preparation и transition journals. Он владеет canonical preparation record
grammar, порядком фаз, исключительно созданным private directory, исходными
file identities, finite inventory, синхронизацией и сохранённой первой ошибкой.
Installation создаёт его после собственной admission/authorization и закрывает
до возврата lease; explicit recovery отдельно читает его record grammar, не
усыновляя новый journal по факту наличия каталога. Для complete staged successor
`OpenTransition` независимо удерживает exact ранее наблюдённые flat records и
directory inode; unobserved records/groups и изменения original access/ctime
отказывают. Installation отдельно проверяет schema/phase/provenance и fresh
proofs до effects. `Owner` скрывает root, записи
и failure latch за Create/Append/Observe/RecordFailure/Close. Account creation,
поколение, Installation lease и Release history принадлежат другим владельцам.

`Transition` отдельно владеет новым digest-каталогом и закрытыми группами
`creations`, `directory-creations`, `replacements`. Installation сохраняет схемы
этих записей, phase ordering, proof admission и решения об изменении ресурсов.
Module скрывает maps и исходные parent/root/file descriptors; `Ensure`, `Write`
и `Resync` проверяют original identity, exact bytes/access и finite inventory.
Каждый replacement record заново синхронизируется до fixed-resource mutation.
После рождения ошибка возвращает partial custody, которая закрывается только
при завершении исходной операции. Отмена после записи сохраняет visible bytes
и первую ошибку; новый caller не продолжает journal. Отдельный `RecordFailure`
пишет только исходный отказ и не возобновляет обычные операции. `Bytes` отдаёт
копию bookkeeping, которая сама по себе не разрешает эффект. Барьер запуска
закрывается до journal, а journal — до возврата Installation lease.

`process/` — Module исходного process lifetime. Он удерживает настоящий
каталог `/proc/<PID>`, первоначальное start time, executable device/inode и
проверяет точные argv, kernel credentials/protection, cgroup и InvocationID.
`Retain/Observe/Close` скрывают дескрипторы и чтения; ожидаемые факты
копируются при открытии и не дают Release, manager или startup authority.
Installation binding отдельно удерживает исходный caller, read-only lease и
проверенное поколение, сверяя их до и после process observation. Cleanup может
наблюдать тот же процесс без отмены только для физического завершения; новый
PID или разрешение при этом не создаются. Descendant join остаётся у
`cgroup/`, а stop, защита поколения и terminal outcome — у Installation.

`generation/` — Module неизменяемого каталога поколения. Он исключительно создаёт
новый digest-каталог под исходным `generations`, удерживает собственные
parent/root/file descriptors, закрытый inventory пятнадцати файлов, исходные
inodes/access/bytes и первую ошибку. `Create` возвращает частичного владельца
вместе с ошибкой после рождения каталога; Installation сохраняет его до своего
original join. Наличие каталога не даёт права на adoption или повторную запись.

Installation синхронизирует собственный birth journal до первого `Write`
и проверяет исходную lease/intent перед каждым файлом. `Seal` меняет доступ
только полного исходного каталога на том же inode; selection и manager остаются
снаружи. `Identity` и `Bytes` дают копии физических фактов и байтов, а не
root, file handles или mutable maps. Initial/successor staging и fixed resources
используют этот Interface. Доступ к контейнеру поколения по-прежнему меняет
Installation: Module проверяет тот же container inode и только закрытые trusted
access states, а caller отдельно проверяет metadata, допустимые в своей фазе.
Барьер запуска физически завершается до закрытия generation descriptors.

`generation.Snapshot` отдельно открывает sealed generation для чтения. Он
удерживает собственные parent/root/file descriptors, проверяет полный закрытый
inventory, исходные inode/access/ctime и bytes и возвращает только копии байтов
и сравнение физических фактов. Snapshot не открывает writer lock, не создаёт
файлы и не получает права на selection или startup. Исходный caller и первая
ошибка сохраняются до Close. Read-only installed Check использует его внутри
своей отдельной leased операции; root account, mutable roots, fixed resources
и canonical binding всё ещё проверяет Installation. Это позволяет будущему
service-account startup получить собственное read custody вместо root lease.

`startupInspection` отдельно удерживает исходный caller, read-only Installation
root, sealed Snapshot и собственный non-root процесс. `installedFiles` содержит
только физические чтения; root Check оборачивает их своей writer lease, а startup
создаёт новые независимые observations. Process `RetainSelf` допускает только
фактический PID/UID/GID caller; root-only `Retain` предшественника сохраняется.
Оба пути сверяют kernel supplementary groups наряду с UID/GID и capabilities.
Startup сверяет точные live Endpoint/activation properties между повторными
наблюдениями собственных bytes/process. Настоящий command/runtime consumer
удерживает этот lifetime через завершение root-операции и final runtime handoff.
Одно открытие по-прежнему не разрешает participant effects. Actual-manager
startup receipt проверяет non-root invocation и Root archival/ACK; credential-drop
refusal отдельно проверяет отрицательную границу.

`completion/` — самостоятельный Module одного bounded Unix exchange. Он открывает
собственный read-only root, удерживает metadata root-private guard/socket record
и исходный socket inode/access, проверяет actual root peer PID/UID/GID, отправляет
и получает один exact 160-ASCII frame под первоначальным deadline. Он не читает
private intent и не получает writer lease. Cancellation физически закрывает
соединение; исходный callback join и первая ошибка сохраняются до Close.
Startup вызывает Connect/Wait между собственными bytes/process/manager checks,
затем заново проверяет их. После ответа root вправе убрать свой guard/socket;
это не обновляет выбранные bytes или процесс. Module не разрешает runtime и не
пишет root ACK: root-владелец должен сначала действительно наблюдать start и
синхронизировать archive. Native wire fixtures показывают механизм exact reply,
отказов, original-inode substitution и join, а не настоящий archived transition
или установленную готовность.

`candidate_linux.go` собирает одну candidate-start lifetime: original
process/scope retention, failure quiescence и join, start-observation record,
archival и ACK completion. Их private receivers и checks сохранены; successor
transaction по-прежнему допускает staging/selection/reload и подготовку barrier.
Два native теста незавершённой candidate custody находятся рядом в
`candidate_native_linux_test.go`. Barrier владеет своим socket/peer/physical
close, а journal — physical durability; перенос не раскрывает их ресурсы через
новый Interface и не создаёт package ради группировки файлов.

Root `completeCandidateStart` — отдельная private последовательность завершения
successor: исходная Preparation с fresh pair и joined predecessor допускает
только candidate pin той же inspection/selection. Barrier сначала сверяет
actual process/manager и connecting peer/frame, затем удерживает именно этот
pin. До архивирования root записывает и синхронизирует `started-invocation.json`
с exact selected digests, PID и InvocationID; повторные native checks окружают
архивирование original intent. ACK допускается только этим владельцем, с этим
accepted pin и exact durable record/archive. Его cancellation callbacks join
до возврата. Полностью записанный frame сохраняется как post-acceptance fact
даже при поздней ошибке; другой ACK не разрешён. Record сам по себе ничего не
разрешает и не воспроизводится recovery как authority. Initial stopped archive
отдельно требует initial intent и exact `installed-stopped`, а не присутствие
любого седьмого record. Private `startCandidate` теперь владеет original
manager-start attempt, actual process и cgroup custody, включая partial result
после эффекта. Ошибка до полного ACK требует stop и original physical join
до возврата lease; MainPID=0 не заменяет typed отсутствие queued Job и повторные
actual empty/stopped наблюдения. Cleanup context не обновляет original admission
caller процесса. Полный ACK сохраняется после observer Close, и поздняя ошибка
не разрешает stop как never-admitted либо повтор запуска. First failure
сохраняется в original journal. Исчезновение исходного процесса не закрывает
его scope: failed-attempt join удерживает исходные дескрипторы независимо от
live proc observation. Отказ подтвердить текущую invocation запрещает Stop,
но не отменяет этот join; уже завершённый исходный scope допускает только
повторную проверку quiescent manager, отсутствия queued Job и пустого inventory.
Partial scope с MainPID=0 и ошибкой inventory также остаётся у исходной попытки,
а первая ошибка не исчезает после stop/join. Настоящий command/runtime consumer
прошёл actual-manager startup с original archived ACK; полная start/failure
matrix остаётся отдельной проверкой. Private post-ACK cleanup требует того же accepted pin,
full frame, exact synced started record и original intent archive. Он закрывает
исходные connection/listener до unlink, проверяет original socket/record/guard,
синхронизирует parent после каждого удаления и убирает guard последним.
Отмена или подмена сохраняет оставшийся original prefix и первую ошибку;
другой caller не повторяет cleanup. Physical cleanup fixture не создаёт ACK
или runtime authority. Public post-acceptance outcome ещё необходим, и эта
последовательность не является installed acceptance.

Cleanup исходной failed start-попытки отдельно допускает сохранённое manager
состояние `failed/failed`: exact configuration, MainPID=0, отсутствие Job и
полная typed история завершившегося executable с ненулевым failure result.
Успешный Stop не обязан очищать manager failure state. Root повторно проверяет
stopped activation units, отсутствие worker instances и original kernel join;
первая ошибка и guard сохраняются. Этот observation не допускает fresh
provisioning/recovery, нового Start или runtime, и не вызывает reset-failed.

Typed `Job` signature `(uo)` и отсутствие job как `[0, "/"]` сверены
с [systemd v255 property_get_job](https://raw.githubusercontent.com/systemd/systemd/v255/src/core/dbus-unit.c),
доступ 2026-10-07. Это sourced representation; настоящий manager receipt
для новой последовательности ещё нужен. Missing/malformed/pending Job
не разрешает cleanup completion; действующий platform admission не изменён.

Native start barrier хранит guard, socket birth, listener, peer и свой terminal
outcome как одну lifetime responsibility. Его создание, наблюдение invocation,
приём точного completion frame и физическое закрытие находятся рядом в
`start_barrier_linux.go`. Допуск барьера после joined predecessor и stopped
reload остаётся в successor composition. Portable completion grammar отдельно
проверяет canonical bytes. Барьер сохраняет Installation lease и свои admission
проверки; перенос в подpackage ради каталога потребовал бы раскрыть эти ресурсы
или разделить один lifetime между двумя владельцами.

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
Successor transition теперь собран в `successor_transition_linux.go`: original
request/lease и fresh pair, staging, selection/reload и archive-before-ACK
проверяются рядом с завершением этого же владельца. `predecessor_linux.go`
содержит original invocation/scopes, stop и обязательный physical join. Эти
политики остаются у одной Installation-транзакции; дочерние Modules независимо
владеют системными механизмами. Длинный cohesive файл предпочтительнее методов
одного lifetime, разнесённых по phase-файлам. Это ещё не полный структурный
перенос всех операций Installation и не доказательство успешного запуска.

До создания start barrier successor сохраняет read custody собственного
sealed кандидата под той же исходной lease. Directory device/inode сверяются
с первоначальным generation Owner, а bytes — с его закрытым inventory;
decoded binding/request и fixed resources проверяются заново. Эти наблюдения
потребуются процессному pin кандидата: predecessor inventory не подменяет
его executable. Detached bytes и копия lease handles не создают нового
владельца; native tests проверяют эти границы на настоящих файлах без manager
или Release substitutes. Private candidate-start/lifetime теперь реализован в том же владельце;
bounded installed Source/permission consumer теперь подтверждает actual start/ACK
и original physical join в fresh Ubuntu24/systemd255 manager. Полный Service
путь и installed qualification остаются отдельными обязательствами. Ранее отдельный source-matched
actual failed-Start trial подтверждает штатный joined cleanup после отказа
нового executable из-за отсутствующего consumer: ten original replacements,
selection/intent/floors/phase7 неизменны, first failure и guard/socket сохранены,
writer освобождён и original scopes завершены без manual disposal/reset-failed.
Это не принимающий runtime и не полный interruption matrix.

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
initial stopped recovery имеют новые consumers; successor transition имеет
публичную command composition; bounded independently admitted installed startup
выполняет genuine Source refresh и qualified permission bootstrap. Successor
recovery имеет собственные bounded consumers; полная interruption matrix и
installed acceptance остаются отдельными обязательствами.
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

Исходники ниже — named provenance прежнего владельца в текущем checkout, не dependencies. Без полного пути имена в этой таблице относятся к `internal/endpoint/installation`; текущая компоновка нового владельца описана выше.

| Прежняя ответственность | Проверенный source | Решение для нового владельца |
|---|---|---|
| Coherent fresh pair | `internal/endpoint/installation/release_authentication.go` | Перепроверить по новому Enrollment/Release; private proofs вместо mutable public Decision |
| Existing trust continuity | `successor_authentication_linux.go` | Portable rule; actual complete floors и binding constraints, никакого initial-pin bootstrap |
| Request and declaration consistency | `request.go`, `internal/endpoint/runtimeplan` | Сохранить grammar/limits; отделить pure declarations от acquisition/credentials |
| Frozen generation assembly | `generation_assembly.go`, `unit_rendering.go` | Portable deterministic bytes; actual inode/account facts проверяются native owner |
| Ownership and journal writes | `generation_ownership_linux.go`, `resource_creation_linux.go`, `resource_replacement_linux.go` | Native identity/durability; canonical records и переходы отдельно от syscalls |
| Original process termination | `predecessor_linux.go` | Exact invocation + original pins, no replacement-generation completion |
| Selection/reload/start/ACK | `transition_finishing_linux.go`, `start_guard_linux.go`, `start_completion_linux.go` | Самостоятельный transition lifetime, retained errors и explicit barrier |
| Ordinary Job/worker lifecycle | `internal/endpoint/worker` | Оставить отдельному новому Execution; installation не получает fake qualification через wrapper |
| Runtime authority and Service | Endpoint composition | Оставить новым Network/Admission/Publication/Instance/Connection consumers |

## Приёмка и execution profiles

### Контракт с новым installed runtime

Fixed ExecStart сохраняет `endpoint start-installed <installation-root>` и
точный generation executable. Его новый consumer находится в `ardents-next` command
composition и удерживает Installation startup lifetime. До participant effects
Installation самостоятельно наблюдает свой actual non-root process и read-only
selected generation, account/roots/fixed bytes, original executable/argv/groups,
cgroup и actual MainPID/InvocationID, включая обе activation sockets. Этот
read lifetime не открывает root writer lease и не получает Release authority
из persisted binding. Во время successor start он проверяет root peer и
завершает original bounded exchange; после ACK повторно сверяет свои bytes,
process и manager. EOF или timeout не заменяют ACK.

Новая runtime composition потребляет только bound неизменяемые Headless/Source
declarations после final Installation handoff. Она отдельно создаёт настоящие
Network/Admission/Route и отдельный Execution; private Publication/Connection
остаются будущими владельцами;
Installation не выдаёт Job/Grant, Service readiness или Connection authority.
Runtime закрывает собственные ресурсы и retains terminal result на исходном
caller. Положительный root archival/ACK receipt требует именно этого настоящего
consumer. Простое чтение Snapshot, test peer, filesystem marker или новый
`start-installed`, который после ACK печатает результат и выходит, его не заменяет.
Actual-manager startup и отдельные archive/ACK interruption trials уже
воспроизведены через этот consumer. Полная interruption matrix и installed
Service acceptance остаются самостоятельными границами.

### Проверки Installation

До обращения к manager для successor Start исходный barrier записывает и
синхронизирует `start-attempt.json` в original transition journal. Closed
`ardents-endpoint-installation-start-attempt-v1` связывает intent digest и digest
исходного socket birth record; после journal I/O guard/socket проверяются снова.
Отмена, uncertain write или подмена запрещают этот Start и сохраняют provenance.
Запись означает только намерение эффекта, не successful Start, InvocationID или
ACK. Её отсутствие в старом журнале не доказывает отсутствие попытки. Terminal
recovery сохраняет совместимость старого журнала; если запись присутствует,
она обязана совпасть с exact intent и original socket из removal provenance.
Она не разрешает replay или accepting runtime. Шестнадцать ordinary journal
slots сохраняются; новые attempts и namespace capacity не добавляются.
Отдельный source-matched actual-command trial прерывает исходный caller после
file/directory sync этой intention до manager Start. После original process join
и освобождения writer новый `recover-installed` возвращает
`installation-repair-required`. Независимый audit подтверждает неизменность всего
retained inventory, десяти original fixed replacements, guard/socket, floors и
actual stopped candidate manager. Original guest/scope физически joins. Это
доказанный отказ на pre-Start границе, не recovery permission, accepting startup
или завершение старого actor после уже начавшегося manager request.

Fixed systemctl/busctl subprocess lifetime удерживает создающий OS thread до
`Run`/physical join и устанавливает Linux parent-death SIGKILL. Без этого
actual-command scheduling control подтверждает живой original Start helper после
join root caller и typed отсутствия manager Job: отпущенный helper выполняет
поздний Start. На исправленном source отдельный fresh manager trial подтверждает
kernel SIGKILL и физический join original helper после смерти caller до manager
request; Endpoint не исполняется, stopped manager и весь retained inventory
сохраняются, fresh recovery отказывает. Нативный cancellation probe отдельно
проверяет join исходного child при удержанном proc descriptor. Завершение helper
не отменяет запрос, уже принятый manager, и не разрешает pending Start recovery.

Две отдельные actual-manager trials используют обычные canonical artifacts.
Первая прерывает original Root после успешного завершения Start helper, но до
Root observation/started-invocation record: original helper и Endpoint физически
joins, fresh recovery отказывает без изменения retained inventory и floors.
Вторая завершает только original Endpoint через independently pinned pidfd в
той же точке, оставляя Root живым. Root сам сохраняет exact first failure,
выполняет собственный Stop/join и освобождает original writer lock; fresh recovery
отказывает, сохраняя guard/socket, все replacement records и floors. Original
proc/kernel observers и отдельная проверка receipt подтверждают эти границы.
Исходные native FAIL и outer exit1 сохранены; проверка receipt — executor
self-review, не independent validation. Это component refusal/cleanup evidence,
не qualification-profile pass, complete interruption matrix или installed Service
acceptance; power loss не проверен.

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

Перед каждым post-ACK удалением completion socket, socket record и guard
Installation синхронизирует отдельную finite removal record в исходном transition
journal. Запись связывает exact intent digest, pathname, исходные device/inode,
access, size и timestamps, а для regular files — exact bytes digest. После journal
I/O исходный объект проверяется повторно до unlink; после unlink синхронизируется
исходный parent. Socket custody также сохраняет исходные ctime/mtime, включая
отказ при возврате прежних прав после same-inode mutation. Запись означает
намерение физического удаления, не доставленный ACK или новое admission. Она
сохраняется при отменённом removal prefix, но сама не разрешает explicit recovery
и не доказывает положительный installed start или crash/reopen сценарий.

Завершение требует полного регресса затронутых новых доменов, обычных
`make quick-check`/`make check`, selected native checks, source-matched receipts,
scoped commit/push и проверенной `dev`. Полная installed acceptance отдельно
требует двух отдельных admitted system managers/Endpoint principals, public
commands, actual containment/empty scopes и publish/link/read/refresh/withdraw/
restart на TCP/TLS и QUIC. Bounded Execution consumer уже реализован; без
private Publication/Instance/Connection эти полные сценарии остаются недоказанными. Нельзя закрыть Installation
или всю цель Route по одной только паре разрешений или остановленной установке.

### Recovery остановленного reload prefix

Exact `0006.json` и `0007.json` требуют full candidate selection и все десять
original replacement records. Installation сопоставляет actual loaded manager
configuration с complete previous/candidate при 0006 и только candidate при
0007, повторяя actual observations, stopped activation/worker checks и empty
kernel scopes до fresh Release composition. Loaded request/generation facts
отделены от original predecessor process/scopes и join. После fresh proofs и
original provenance resync continuation не повторяет earlier fixed-file writes
или Stop/ACK: pending 0006 выполняет admitted reload и independent candidate
checks, затем appends 0007; существующий 0007 только resync-ится без reload или
перезаписи original record. Guarded start остаётся отдельным lifetime. Typed
fixtures проверяют pure configuration/phase rules, filesystem probes — original
provenance/resync. Три отдельные actual-manager process-crash trials проверяют
fresh-proof recovery до reload, после reload до phase7 и после durable phase7
до Start; original records, fixed resources и floors сохраняются. Recovered
Endpoint достигает permission-pending, не полной Service readiness. Startup
и terminal guard recovery имеют отдельные проверки.

## Original generation file provenance

Перед первой записью каждого immutable artifact physical `generation.CreateFile`
создаёт и синхронизирует пустой original leaf. Installation сначала записывает
`generation-files.json` с exact closed inventory и original selection binding,
затем root-only `generation-file-<name-digest>.json`: actual device/inode,
expected digest/length и intended access/group. Только durable birth record и
повторное наблюдение исходного stage допускают `Write`. Physical Owner удерживает
исходный file descriptor при birth/write/cancellation failures до Close; access
или ctime mutation после journal I/O не разрешает новую запись. Directory seal
требует всех пятнадцати completed original files, а не только их присутствия.

Журнал сохраняет прежние phase/schema identities. Его physical retained reader
раздельно ограничивает шестнадцать ordinary transition slots и пятнадцать
file-birth slots; свободный ordinary slot не даёт ещё один file birth. Это
конечная provenance namespace, не дополнительные attempts или deadlines.
Sealed initial/staged/terminal recovery проверяет marker и полный exact набор
birth records против actual original inodes и binding/target bytes. Initial
recovery дополнительно сверяет lengths с fresh-proof generation до mutation.
Старый complete sealed prefix без marker и без file births сохраняет прежние
ограничения; неполный новый набор или births без marker отказывают. Эти записи
не создают fresh proof, creation Owner, selection или право startup.

Recovery записи generation удерживает отдельный physical `generation.Prefix`.
`successor_recovery_linux.go` собирает Installation admission исходного
intent, previous sealed generation, closed journal и pathname-ordered birth
prefix. Все предшествующие записанные файлы должны быть complete; только
последний birth допускает private/torn bytes. Promotion directory допускается
лишь при complete inventory. Публичные expected binding facts отделены от
inspected candidate и private Release proofs. Две свежие авторизации и полные
preimages предшествуют original-record resync, ремонту original inodes и seal;
независимый Snapshot затем повторно проверяет actual candidate перед phase2 и
обычным predecessor/start lifetime. Physical Prefix не получает journal, lease
или право установки. Он удерживает собственные descriptors и caller failure до
Close, включая неполное открытие.

Filesystem/race admission и causal removal проверки порядка подтверждают эту
закрытую файловую границу. Отдельное actual-command испытание process crash после
durable birth первого пустого leaf подтверждает fresh-proof recovery этого же
inode до полного sealed inventory и actual-manager stopped reload. Независимое
наблюдение сверяет все пятнадцать births, десять original fixed replacements,
сохранённые original records и Release floors; оба прерванных caller и отдельный
test-manager физически joined. Подписывающие builders и неисполняемый worker
остаются fixtures. Эта одна граница не доказывает torn writes, power-loss,
полную interruption matrix, accepting startup/ACK или installed qualification.
Filesystem fixtures сами по себе не заменяют эти проверки.

Отдельное actual-command прерывание после успешной записи 128 КиБ original
program до failed phase и first-error record проверяет fresh-proof continuation
того же inode до complete inventory и durable stopped reload. Original writing
и file-birth records, committed floors и exact bytes сохраняются; исходные
caller и manager scope физически joins. Неуспешная первая попытка наблюдения
остаётся отдельным receipt; fresh trial с ранним наблюдением подтверждён audit.
Deliberately interrupted repair не является completed public success или
qualification-profile pass. Эти наблюдения не подтверждают power loss.

### Generation-write first failure

Actual-command kernel write-failure control оставляет exact program prefix на
original inode и `0002.json` с `generation-write-failed`. Release floors уже
committed до этой записи; fixed resources, predecessor selection и sealed
predecessor остаются прежними. До ремонта recovery классифицировал присутствующий
`0002.json` как completed generation и возвращал `installation-repair-required`,
сохраняя failure, prefix и committed floors. Это подтверждённый пробел выбранного
first-failure recovery, а не accepting или repaired outcome.

Ремонт сохраняет existing grammar и finite slots: Installation отдельно
распознаёт bound failed phase, допускает только closed generation birth prefix
и получает две свежие Release authorizations. До ремонта листов она должна
durably сохранить exact failed record в original first-failure provenance,
повторно проверить оба original observations, затем retire только тот failed
phase inode и синхронизировать journal directory. Crash между copy и retirement
должен допускать только byte-identical original failure; новый caller повторно
получает fresh proofs. Successful phase2 создаётся отдельно после independently
observed complete seal. Первая ошибка сохраняется при последующих отказах и
архивируется с completed intent. Journal владеет closed physical retirement;
Installation — failed-phase grammar, fresh proofs и complete-seal admission.
Нативные проверки отдельно наблюдают physical retirement и bound failure
prefixes. Actual-command kernel write-failure и fresh-proof recovery подтверждают
сохранение exact first-error bytes, завершение original partial inode, всех
пятнадцати generation births и десяти fixed replacements, неизменные committed
floors и actual stopped reload. Original caller прерывается после durable
stopped record до Start; независимый audit проверяет этот prefix и освобождённый
writer, затем original test-manager scope физически joins. Это stopped-only
сценарий с signing/builder fixtures и worker marker, который не выполнялся.
Отдельная actual-command последовательность прерывает original caller после
durable failed phase до cleanup copy, после synced first-error copy и после
synced retirement failed slot. Каждый следующий caller заново открывает
retained prefix и получает fresh proofs; независимые audits сохраняют exact
first-error bytes, original partial inode, journal, predecessor selection и
committed floors. Последняя continuation достигает durable stopped reload с
complete original generation и fixed replacements; original tasks, writer и
test-manager scope физически joins. Это подтверждает эти process-crash границы,
включая отдельно воспроизведённое прерывание после successful unlink failed slot
до directory sync. В этой точке exact first-error copy уже file/directory-synced;
после физического join исходного caller следующий fresh-proof recovery завершает
original generation и fixed replacements до stopped reload. Это не доказывает
power loss или accepting continuation. Этот путь не допускает pending Start,
guard/socket или ACK replay
и не решает provenance старых Start actors.

Повторный cleanup generation-writing failure копирует весь original failed
record, включая phase label и первую ошибку. Изменение label даже при одинаковом
error text нарушает byte-identical provenance и запрещает retirement. Existing
conflicting copies сохраняются и отказывают; такой отказ не разрешает переписать
историческую ошибку или автоматически выбрать одну из неоднозначных записей.

`fixed_resource_images_linux.go` собирает closed fixed-resource paths, образы из
retained generation bytes и canonical artifact manifest. Initial, successor
и recovery используют одну схему manifest; successor и recovery — один
построитель девяти образов. Selection не входит в этот набор: её байты,
original provenance и отдельный порядок публикации остаются у транзакции.
Это private сборка detached bytes в том же пакете, без filesystem custody,
Release proofs, mutation permission или lifetime. Initial сохраняет свой
порядок creation/access/manifest, successor — original replacement records,
recovery — independent admission и fresh proofs. Разделение устраняет две
копии byte policy, не создаёт новый package или shared mutable owner.
