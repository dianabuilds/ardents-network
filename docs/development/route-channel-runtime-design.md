# Общая механика каналов Route

Инженерный проект частичной переработки по прямому поручению Product Owner
от 2026-10-05: общая база для TCP/TLS и QUIC, явные этапы операций и зависимость
от ОС только там, где различается необходимый механизм. Это проект реализации,
не изменение протокола и не утверждение о завершённом переносе.
Выбор работы и результаты принадлежат GitHub. Обязательны
[контракт миграции](route-migration-contract.md),
[карта доменов](domain-map.md),
[протокол](../technical/protected-route-protocol.md) и
[реестр пакетов](package-map.md).

## Основания в исходниках

Проверена база `1098fefb1feec54f59ac38cbe921b1148dd89ba3` и сохранённое
исправление исходного caller в публичном `Prefix.Replenish`. Исправление
отмены — вход переработки, а не работа, которую можно отбросить.

- `carrier/node_carrier.go` содержит переносимые интерфейсы, но ограничен Linux.
  `transport/queue_budget_linux.go` использует только `errors` и `sync`.
  Framing, очереди и большая часть завершения также не требуют Linux сами по себе.
- Переносимые факты и проверки выбранного пути находятся в `selection/leg.go`;
  открытие и удержание durable roots отдельно принадлежат
  `selection/selection_owner_linux.go`. Introduction recipient и context-local
  Rendezvous работают над retained Leg и Network observations без файлового
  владельца. Отсутствие syscall в конкретном файле само по себе ещё не делает
  переносимым его граф зависимостей.
- Состояние пополнения разделено между полями `session.parent*` и `lane.refill*`.
  Замена оставшегося allowance вызывается при обработке ответа и завершении
  операции. Это основание собрать переход в одном владельце, а не доказательство
  уже установленного двойного начисления.
- Исходный caller операции, lifetime префикса и производный контекст прерывания
  имеют разные значения. Исправление публичного пополнения показывает, почему
  проверка только производного контекста недостаточна.
- `selection/root_lease_linux.go` и `introduction/slot_history_linux.go`
  действительно используют flock и ограничения открытия файлов. Их гарантии
  нельзя получить простым снятием build tags. Измерения Hosting остаются Hosting.

## Владельцы и структура

| Модуль | Владеет | Не получает |
|---|---|---|
| `route/ardp` | Канонический codec и ограничения грамматики | Lifetime, разрешение операции, ресурсы |
| `route/transport` | Общий контракт ordered stream, параметры физического открытия, категории транспортных ошибок с исходными причинами и общие проверки входа | Конкретный адаптер, framing Route, Network/Admission, выбор пути |
| `route/transport/tls`, `route/transport/quic` | Реальные TCP/TLS и QUIC реализации общего контракта: точная аутентификация ключа, exporter, socket/stream deadlines и физическое закрытие | Framing Route, выбор пути, токены, повтор операции |
| `route/channel` | Единственный reader, сериализованный writer, lanes, очереди/credit, полные frame debits, control exchange, seal/join и сохранённый физический результат | Проверка State, выдача или spend токена, provider budget, Service authority |
| `route/role` | Переносимая проверка исходного retained Network duty/profile и HELLO, binding от настоящего negotiated exporter, holder HELLO/presentation/ADMIT/ACCEPT с проверками original caller вокруг I/O | Собственный Stock/spend, receiving Grant/retirement, Hosting reservation, slot/pair/prefix lifecycle |
| `route/prefix` | Исходные Entry/Interior и Source/Responder prefixes, immutable generation, допуск и учёт borrowers, parent refill и joined retirement | Состояние Registration, JOIN acquisition/pair, listener, собственная quota или spend |
| `route/introduction` | Раздельные владельцы: holder Registration с исходным запросом/conn/reader/withdrawal; receiving Registry и History со slots, opaque delivery и независимыми durable floors | Общий aggregate/root/lifetime между сторонами, Publication readiness, успешный ACK отсутствующего recipient |
| `route/join` | Source/Responder acquisitions, исходная попытка, matching pair, RESULT barrier и joined stream | Prefix generation и roots, Introduction slots, Service authentication/continuity |
| `route/receiver` | Listener, остановка и join принятых connections; exact-purpose dispatch к владельцу поведения; receiving composition настоящих соседних owners | Правила всех handlers, чужие quota/spend, состояние holder Prefix/Registration/JOIN |
| `route/selection` | Retained Entry/Interior/Rendezvous выбор и собственные durable floors | Живые channel/lane и общая mutable история других владельцев |

Уточнение Product Owner выбирает отдельные реализации `transport/tls` и
`transport/quic` с едиными входными и выходными контрактами. Поэтому прежнее
имя transport больше не может одновременно обозначать высокоуровневые
операции, импортирующие оба адаптера. Целевая структура:

```text
route/
  ardp/                  каноническая грамматика
  transport/             общий транспортный контракт и его проверки
    tls/                 реализация TCP/TLS
    quic/                реализация QUIC
  channel/               общая механика framing и завершения
  role/                  общая проверка duty/profile/HELLO
  prefix/                исходные prefixes и lifetime заимствований
  join/                  acquisitions, pairing и joined streams
  receiver/              listener и exact-purpose receiving composition
  selection/             retained выбор и собственная история
  introduction/          отдельные holder/receiving владельцы и slot history
```

Это структура модулей одного домена, не новые домены. Каждый верхний пакет
владеет согласованным состоянием и завершением своей операции. Команда создаёт
настоящие соседние domain owners и передаёт их узкие реальные операции.
`operation` допускается только как промежуточное место сохранённого кода при
извлечении: оно отсутствует в конечной модели и не сохраняет второй runtime,
скрытую общую реализацию или фасад ради прежних callers.

Целевая отдельная граница общей механики — `route/channel`. Её допустимые
зависимости: `route/ardp`, нижний `route/transport` и стандартная библиотека;
реальные потребители — владельцы prefix, Introduction, JOIN и receiving.
Carrier передаётся через общий
ordered-stream interface без импорта конкретного QUIC в канал. Канал не
импортирует prefix, introduction, join, receiver, selection, Network,
Admission или Hosting. Перед
выделением исполнитель проверяет, что
малый интерфейс действительно скрывает весь согласованный lifecycle, а не
экспортирует private поля и десятки методов старой session. Реестр точных
импортов, doc.go, реализация, проверки поведения и настоящий вызывающий код
добавляются вместе. Этот проект сам по себе не добавляет Go-пакет или import grant.

Если для согласованности session/lane требуется временно сохранить их вместе,
промежуточная перестройка допустима внутри временного operation до извлечения.
Конечный результат должен
иметь проверяемую переносимую общую механику и ясную структуру ответственности;
обёртка над прежним Linux-only runtime не выполняет задачу. Не создавать общий
пакет для всех доменов или отдельный домен «harness».

### Разные операции и владельцы их lifetime

Уточнение Product Owner: один каталог `operation` не решает смешение разных
операций. Он не является конечным владельцем всех сценариев Route. Перенос
существующего кода в этот каталог освобождает имя нижнего transport, но не
доказывает выделение ответственности. Следующее разделение следует состоянию
и завершению операций, а не одному пакету на каждый wire message.

| Ответственность | Связанные действия | Состояние и завершение |
|---|---|---|
| Prefix | Открыть Entry/Interior, заимствовать разрешённый terminal, пополнить parent, seal/close | Исходная физическая generation, parent allowance и учёт её borrowers |
| Holder Registration | REGISTER, чтение opaque delivery, WITHDRAW | Исходный запрос, conn/reader и первоначальная registration lifetime; заимствование exact Prefix generation |
| Receiving Introduction | Проверить/зарегистрировать slot, dispatch opaque delivery, принять WITHDRAW | Registry, pending deliveries и независимая History с durable non-reclaim floors у receiving principal |
| JOIN | Source/Responder acquisition, matching, RESULT, framed joined stream | Одна исходная попытка и pair, result barrier и joined termination |
| Receiving composition | Принять Carrier/role channel и направить exact purpose его владельцу | Listener, принятые connections, их остановка и join; не правила всех получателей |
| Channel | Framing, scheduling, lanes, credit, bounded control exchange | Один reader/writer и физическая ошибка; никаких slot, pair или Network решений |

Реализовать эти владельцы как cohesive пакеты после проверки настоящих callers:
например `prefix`, существующий `introduction`, `join` и `receiver`. Это имена
проектируемых границ, не регистрация отсутствующих Go-пакетов. REGISTER и
WITHDRAW остаются вместе; refill остаётся действием исходного parent, а не
отдельным subsystem. Общий transport и channel не получают зависимости от
этих высокоуровневых владельцев.

В исходной смешанной реализации `Prefix.Register`, `Prefix.AcquireSourceJoin`
и private доступ Registration/JoinAcquisition к Prefix показывали связанность,
которую нельзя перенести в циклические imports. Prefix владеет допуском,
заимствованием и join своей исходной generation. Registration/JOIN владеют
собственной операцией и возвращают заимствование после своего завершения.
Договор передачи должен удерживать те же синхронные currentness, seal и
release-after-join гарантии; вынесение mutex или private полей в публичную
структуру не является решением. Интерфейс заимствования вводится только вместе
с настоящими производственными consumers и причинными lifecycle tests.
Парные claims Source/Responder принадлежат Prefix: их допуск и публикация
блокируют сначала Source, затем его original Responder. JOIN берёт свой stream
mutex только внутри этого commit, проверяет исходного caller и sealed state и
публикует результат после проверки обоих original prefixes. Наблюдение Network,
физический I/O и join выполняются вне generation locks. Private pair claim
возвращается после JOIN physical cleanup; новая пара не может привязать чужой
Source к retained Responder. Этот переход реализован через
`prefix.JoinBorrow.Publish`/`Prefix.CommitPair` и
`join.JoinAcquisition.publish`. Пакет `operation` удалён; приватные mutex
и поля generation не становятся публичным API.
Pending terminal setup также удерживается владельцем Prefix. Только still-held
claim может один раз передать физическую работу borrower той же generation;
публикация проверяет исходного caller, child и synchronous Seal под owner lock.
Completion setup идемпотентен и не возвращает уже опубликованный borrow.
Registration не держит Prefix mutex и не хранит Prefix вместо своего borrow;
она сама joins reader/writer/caller/physical work перед возвратом claim.
Receiving composition вызывает владельцев по точному purpose; они не
импортируют listener обратно. Exact imports и отдельные doc.go добавляются с
реализацией. После этого временный `operation` не сохраняется как второй runtime
или фасад ради прежних callers.

### Направления зависимостей

Общий transport использует только стандартную библиотеку. Оба адаптера
импортируют этот контракт; TLS использует стандартную библиотеку, QUIC также
разрешённый `github.com/quic-go/quic-go`. Общие TLS-проверки допустимы в нижнем
transport, если действительно нужны обоим адаптерам; не создавать зависимости
QUIC-адаптера от TCP-адаптера только ради helper. Нельзя импортировать адаптеры
обратно из transport или channel. Выбор конкретной реализации по уже выбранному
профилю выполняется у реального opener в prefix или receiver; это точный выбор
без fallback или нового retry.

Prefix не импортирует Introduction/JOIN или состояние их операций. Consumer
получает точное исходное заимствование Prefix generation с synchronous seal и
currentness; после остановки и join своего физического work возвращает borrow.
Parent retirement сначала seals/interrupts всех borrowers, затем joins их и
только потом возвращает roots/reservations. Private mutex и поля не становятся
публичным интерфейсом. Не вводить общий root или global transaction.

Introduction и JOIN используют такой prefix borrow и общий channel; они не
импортируют receiver. Receiving composition вызывает нужного владельца по
exact purpose и отвечает за listener/connection join. JOIN сохраняет distinct
source/responder/acquisition/pair lifetimes. Holder Registration и receiving
Registry/History не делят aggregate, root или lifetime, даже если подтверждённая
cohesion оправдывает один пакет Introduction.

Сохранённые зависимости прежних операций на route, selection, ardp, Network и
Admission/receiving распределяются только по фактическим новым callers и
владельцам. Общий канал не получает их ради удобства extraction. Команда
вызывает настоящие верхние операции и receiving composition. Бывшие carrier
и временный operation удаляются после переноса всех реальных consumers и
тестов; не оставлять рабочую копию или facade. Exact imports, doc.go, behavior
tests и non-test consumers регистрируются с реализованной границей. Разрешение
тестовых импортов также соответствует реальным contract tests. Проектная схема
не заменяет package-map и не регистрирует отсутствующие пакеты.
## Общий интерфейс и различия Carrier

Отправная точка — уже используемый `net.Conn` для role channels и узкий
`Carrier` для outer lanes. Требуемые операции выводятся из действующих callers:
ordered read/write, независимые deadlines там, где ими пользуется scheduler,
прерывание и закрытие; аутентифицированный exporter остаётся механизмом Carrier.
Не объединять outer Node authority и inner role authority ради одного типа.

Оба адаптера исполняют одинаковые правила channel admission, byte accounting,
cancel и join. Различия TLS record I/O, QUIC stream/connection retirement,
handshake и транспортных ошибок остаются внутри Carrier. Закрытие физического
ресурса само по себе не означает, что завершены все заимствовавшие его операции.

Общие категории результата различают отказ до эффекта, отмену, expiry,
отказ peer, ошибку протокола, недостаток capacity и физический сбой.
Сохранять исходные причины через wrapping/unwrapping, включая все ветви joined
errors. Признак peer retirement в одной ветви не превращает другие ошибки
в успешное закрытие. Начавшийся write и доказанно не начавшийся write остаются
разными фактами; нельзя выводить отсутствие эффекта только из класса ошибки.

## Этапы и точки необратимости

Этапы должны быть видны в коде через именованные операции и владельцев переходов.
Это не новые wire fields, публичные trace IDs или универсальная транзакция.

1. **Связать запрос.** Сохранить исходный caller операции, lifetime владельца,
   точный peer/purpose и исходные абсолютные границы. Производный контекст
   прерывания не заменяет ни одну из этих идентичностей.
2. **Проверить текущие основания.** Получить настоящее наблюдение Network,
   проверить exact duty и ещё раз исходные callers после завершения I/O.
3. **Подготовить разрешённый эффект.** Holder резервирует требуемую capacity
   и durably отмечает presentation до отправки token bytes. Receiver после
   получения ADMIT проверяет право, резервирует capacity и выполняет durable
   spend до ACCEPT. Конкретный порядок внутри этих разных путей остаётся
   у действующего контракта Admission/Hosting; общей «commit после обмена» нет.
4. **Выполнить обмен.** Один reader и один выбранный физический writer;
   учёт полного frame и факт начала I/O принадлежат каналу. До начала можно
   отменить только выбранную работу; после начала неопределённость нельзя
   превратить в replay, refund или успех. Долгое storage I/O не держит mutex
   канала и не блокирует допустимый control/termination соседей.
5. **Завершить переход и передать результат.** Один владелец фиксирует изменение
   allowance по сохранённому witness с учётом параллельного трафика. Completion
   наблюдает именно этот переход, не начисляет повторно. Перед выдачей результата
   повторяются обязательные проверки authority, времени и исходных callers.
6. **Закрыть допуск.** Синхронно запретить новые эффекты и прервать свою работу.
7. **Дождаться завершения.** Join исходных readers/writers/children и начавшихся
   callbacks; timeout ожидания не считается завершением.
8. **Вернуть ресурсы.** После join ровно один раз вернуть переданные reservations
   и borrowed roots. Повторный Close возвращает сохранённый terminal result.

Активный parent exchange имеет одного bounded владельца: состояние, исходный
caller, charged/witness, ответ, требуемая дополнительная capacity и terminal
result. Завершённые попытки не накапливаются. При этом доля Hosting и объекты
Grant остаются у своих владельцев; канал не начинает проверять токены.

## Переносимость

Переносимое ядро содержит framing, scheduling, credit/accounting, operation
lifetime, ошибки и неизменяемые факты. Linux-ограничения остаются у механизмов,
которым нужны flock, безопасное открытие/права файлов, fsync/rename или счётчики
ОС, и у композиции, которая действительно требует эти механизмы.

Разделять rule и adapter внутри каждого существующего владельца. Не переносить
всю persistence в новый общий storage домен. Не добавлять успешные заглушки
для неподдержанной ОС, не сбрасывать durable floors и не подменять genuine
roots памятью. Недоступный обязательный механизм отказывает до эффектов.
Portable component tests на Windows не квалифицируют Windows installation.

## Приёмка переработки

### Обоснование платформенных границ

Само наличие Linux deployment profile не разрешает ограничивать общую механику
этой ОС. Ограничение определяется реально используемой гарантией и полным
графом зависимостей. Переносимость правила и поддержка installed-продукта —
разные утверждения.

| Владелец | Реальная зависимость | Требуемое размещение проверок |
|---|---|---|
| `channel`, `ardp` | Ordered I/O, context, время, sync; без native durable roots | Общие behavior tests без Linux tags, одинаковые oracles на Windows и Linux |
| `role` | Стандартные TLS exporter/ordered I/O, context и exact retained duty/profile; без native root или receiving Grant | Portable actual-TLS oracle: original caller cancellation после presentation не эмитирует ADMIT; plain stream без exporter отказывает до presentation. Настоящие Stock/spend подтверждаются отдельно в command scenarios |
| Prefix runtime, setup/borrow/readiness/refill | Portable Network/Leg facts, role/channel и настоящие TLS/QUIC adapters; не открывает durable root | Одинаковые физические pipe, setup cancellation/publication, idle/join и parent-presentation refusal tests выполняются Windows/Linux. Тот же Prefix использует genuine Linux command composition; это не квалификация Windows Stock/root/installation |
| `transport/tls`, `transport/quic` | Стандартный TLS/TCP и поддерживаемый QUIC; различия deadline/close принадлежат адаптерам | Настоящие handshake/I/O tests на обеих ОС; native ошибки отдельно |
| `transport/socket_retirement_*` | POSIX errno и Winsock имеют разные нативные коды | Точный native classifier; общий тест категории не подменяет проверку Winsock |
| `selection/root_lease_linux.go` | Неблокирующий exclusive `flock` на удерживаемом descriptor | Реальные конкурирующие владельцы root; память не заменяет межпроцессный lease |
| `selection/root_permissions_linux.go`, `directory_sync_linux.go` | Unix permission bits и durable directory sync | Private-root и crash/reopen гарантии проверяются на поддерживаемом файловом механизме |
| `introduction/slot_history_linux.go` | `flock`, `O_NOFOLLOW`/`O_NONBLOCK`, durable независимые floors | Actual roots, unsafe-file refusal, uncertain write и reopen; нельзя заменить памятью |
| `introduction/slot_snapshot.go` | Платформенных механизмов нет: канонические binding, hashes и original-expiry/time-floor bytes | Переносимые independent-byte и повреждение/rebinding проверки; тот же codec использует настоящий durable History. Кодировка не выдаёт ACK или live registration authority |
| `introduction/slot_history.go`, `registry.go` | Платформенных механизмов нет: floors/claim/terminal state и live capacity/ACK/owning withdrawal | Portable отказ без retained storage и сохранение оригинальных ошибок/Close; genuine успешные claims/reopen/uncertain-write проверяются отдельно на настоящем Linux file adapter. Приватная storage seam не является публичной возможностью подставить ACK |
| Holder REGISTER/WITHDRAW | Переносимые Prefix, role, TLS/channel и Introduction bytes; собственный durable root не открывает | Prefix владеет retained leg/recipient checks, control claim и OPEN/TLS/role admission; Registration не читает его config/context/physical parent, а владеет operation bytes/ACK и своим reader/writer join. Клиентская реализация и causal pre-effect refusal выполняются Windows/Linux. Успешные Stock/spend/ACK по обоим Carrier остаются genuine command scenarios с native roots |
| JOIN acquisition/client/pair/relay/stream | Переносимые Prefix, role/channel, context и ordered I/O | Клиентские, pairing, framing и joined retirement правила и их механические проверки выполняются на Windows/Linux. Genuine signed State/spend/Carrier сценарии проверяются отдельно в native command composition |
| JOIN Context и receiving composition | Context использует concrete selection.Owner с leased durable roots; Receiver соединяет реальные native Admission/Hosting/selection owners | Linux integration и root/lease/reopen проверки остаются у этих конкретных владельцев. Ограничение композиции не переносится на отдельные переносимые JOIN правила |
| Сквозная `cmd/ardents-next` композиция | Настоящие Network/Admission/Hosting и Route roots используют выбранные Linux механизмы | Linux integration tests с genuine authority/spend/persistence; portable mechanism pass не считается их выполнением |

Тест остаётся у владельца проверяемого перехода. Физический fixture явно
устанавливает только I/O или completion, не успешную authority. Проверка
внутренних счётчиков канала принадлежит каналу; верхний consumer проверяет
доступную capacity, порядок output и retained result через настоящий интерфейс.
У каждой сквозной проверки должны быть указаны операция, реальные соседние
владельцы, наблюдаемый результат и причинный отказ. Один зелёный механизм
не доказывает приёмку композиции.

- Настоящие существующие prefix, REGISTER/WITHDRAW, JOIN и refill callers
  используют одну общую механику на обоих Carrier. Старая дублирующая реализация
  этой механики удалена; old/new isolation сохраняется, включая тесты.
- Один и тот же набор проверок интерфейса выполняется для настоящих TCP/TLS и
  QUIC адаптеров: ordered bytes, deadlines, pre-effect cancel, partial/started
  write, peer/local close, joined cleanup и сохранение физических ошибок.
  Специфичные транспортные различия имеют отдельные причинные проверки.
- Переносимые channel/codec/Carrier проверки действительно выполняются на
  Windows и Linux. Cross-compilation отдельно не доказывает поведение. Для
  оставшихся Linux-only файлов показана реальная платформенная зависимость.
- Genuine Linux композиция использует подписанный State, настоящий stock/spend,
  отдельные durable roots и Hosting. Сохранены причинные controls исходных
  callers при задержанном распространении отмены, потери State/expiry во время
  I/O, no refund после spend, reopen/uncertain writes, shared-budget races,
  sibling progress и release только после join.
- Независимые canonical bytes, refill 370/371/372, замена ровно на 32 MiB,
  отдельный debit ACCEPT и cumulative/concurrent usage сохранены. JOIN и
  остальные запрещённые типы не получают refill. Policy/quota/retry/time bounds
  не меняются. Прежние регрессии нельзя удалять только из-за смены структуры.
- Сбои полного прогона JOIN close и expiry authority получают объяснение
  причины и проверку исправления у фактического владельца. Успешный изолированный
  повтор не заменяет диагностику и полный профиль; старые receipts сохраняются.
- На финальном source identity проходят обязательные quick/full gates,
  Linux full new-domain race/architecture profile и static analysis; затем
  обычные hooks, scoped commit/push в dev и отдельная проверка интеграции.

Issuer, Descriptor Control и ещё не перенесённые соседние домены не добавляются
в эту переработку. Она не завершает весь Route и не даёт installed, privacy
или независимой security qualification.

### Selection rules and durable ownership

`selection/entry_set.go` owns portable public Entry bindings, eligibility,
uniform ordered-pair selection and original absolute bounds. These rules
do not authenticate a Network observation. `closed_sets.go` owns the actual
leased root, serialized activation, commit-before-return and retained-slot
checks against fresh observations. Its Linux dependency comes from the real
root lease and durable filesystem adapter. Moving the rules changes neither
randomness, six-hour bounds, version-1 storage positions nor refusal to replace
a pair after member loss. Native lease/reopen/floor tests remain separate;
Entry rules also execute on Windows. `minTime` lives with retained Leg rules
in `leg.go`; its former single-method file had no independent responsibility.

Public JOIN retirement uses the exact `JoinOpening` identity. There is no
implicit-current-generation `ClosePrefix` method; pending and published
retirement controls preserve the same original handle and joined failure.
The portable pool receives actual authority revalidation from native Receiver
composition directly, without a native-only private wrapper in the pool.
