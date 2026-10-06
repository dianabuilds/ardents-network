# Release verification — каркас

Только описание будущего владельца; Go-пакета и реализации здесь пока нет.
Самостоятельный владелец внутри семейства Software acceptance.

**Вопрос домена:** разрешены ли именно эти программные байты для этого назначения?

**Входит:** аутентификация Release, привязка к точному артефакту и платформе,
ограничения актуальности и монотонная история, непрозрачное разрешение.

**Не входит:** первичное Enrollment-доверие, загрузка, активация и замена файлов,
запуск worker, подпись чужих полномочий или автоматический откат.

**Состояние и завершение:** собственные долговечные floors и точное разрешение;
сомнительная запись или устаревшее состояние не создают успешную авторизацию.
Замена программных файлов не откатывает историю доверия.

**Соседи:** Enrollment сохраняет первичное происхождение; Installation проверяет
разрешение на точные устанавливаемые байты; Execution отдельно проверяет
реальный запуск. Custody хранит полномочия подписи по назначению.

**Где искать обязанности:** `internal/release` и Release-проверки установочных
и runtime-композиций. Не переносить их общий жизненный цикл в этот домен.

**Основа:** [контракт](../../../docs/technical/release-update-custody.md),
[карта доменов](../../../docs/development/domain-map.md).

## Проект до реализации

Это подготовленный проект, не Go-пакет и не принятая реализация. Он сохраняет
текущий профиль Release и ADR-0119. Его задача — определить authority,
consistency и termination до переноса механизмов. Старый runtime не становится
зависимостью нового владельца. Конкретная реализация допускается только через
выбранную задачу C0 с настоящим новым consumer и зарегистрированными imports.

### Один владелец доверия, отдельные операции

Verifier владеет одним installation-local корнем доверия, exclusive lease,
принятыми Root и floors четырёх верхних ролей. Последовательная evaluation
решает, разрешён ли точный target с точными байтами и local binding.
Публичная проекция результата не является authority: opaque authorization
хранит private snapshot, нулевое значение недействительно, наружу возвращаются
копии. Изменение публичного Decision не меняет доказательство.

Enrollment подтверждает независимое происхождение первого комплекта, но не
решает Release policy. Composition переводит его подтверждённые имена и байты
в закрытый offline metadata input. Release не принимает manifest pin как
право обновления. При последующих проверках исходный доверенный Root и floors
определяются собственной сохранённой историей; новый bundle не сбрасывает их.
Installation сохраняет свою транзакцию и проверяет соответствие конкретных
устанавливаемых ресурсов; Execution проверяет реальный квалифицированный запуск.
Ни один из этих владельцев не получает Release storage или signing key.

### Видимые этапы

1. **Admission операции.** Проверить исходный context, закрытый local binding,
   фиксированное UTC reference time и bounded input. Сохранить один private
   snapshot всех metadata и artifact bytes до проверки подписей. Caller не
   меняет переданный input во время его копирования; после admission owner
   не перечитывает внешние mutable maps или файлы. Awaiting операции
   сериализуются на одном Verifier; отмена ожидания не начинает verification.
2. **Исходная история.** Под lease прочитать целую подтверждённую историю.
   Отличать действительно новый корень от повреждённого retained root.
   Потерянный pointer при retained generation не означает initial enrollment.
   Foreign, malformed, partial и uncertain state не разрешают новый target.
3. **Root.** Проверить текущий Root и последовательную цепь вращения.
   Каждый проверенный successor Root публикуется durably до использования
   для следующего Root или metadata. Последующий отказ не откатывает
   уже подтверждённый Root. Не добавлять delegated targets, ambient cache,
   downloader или несколько repositories.
4. **Metadata.** Аутентифицировать timestamp, snapshot и единственный top-level
   targets; проверить expiry на одном reference time, versions/digests и
   consistent-snapshot references. Неиспользованные metadata, превышение
   resource bounds, равная версия с другим digest и rollback отказывают.
5. **Target и policy.** Проверить canonical target path, actual artifact
   length/digest, platform/architecture/environment/Network, существующую
   custom identity и builder commitments. Build safety и protocol transition
   остаются разными правилами; ordinary/emergency thresholds и временные
   границы сохраняются. Ошибка классифицируется по причине, не по substring
   текста другой ошибки.
6. **Commit.** Проверить исходный context перед durable effect. Опубликовать
   целые metadata floors после всех принимающих проверок. При uncertainty
   не выдавать authorization и не пытаться вернуть более старые floors.
   Прочитать подтверждённый результат либо удержать fail-closed outcome.
7. **Handoff.** Повторно проверить исходную отмену и живой Verifier перед
   выдачей private authorization. Отмена после durable commit не отменяет
   floors, но не превращается в успешную передачу права. Close сначала
   прекращает admission, отменяет/соединяет admitted работу, затем освобождает
   lease; поздняя evaluation не работает с освобождённым корнем.

Root-only publication и полная группа metadata floors — разные допустимые
durable состояния. Это не единая атомарная транзакция на всю evaluation.
Точные persisted identities остаются compatibility obligation; подготовленный
проект не разрешает переписать старый установленный корень или удалить историю.

### Два target для защищённого поколения

[ADR-0119](../../../docs/adr/0119-bind-protected-endpoint-generation-to-release.md)
требует две свежие opaque authorizations: executable и
`ardents/linux-amd64/protected-endpoint`. Composition удерживает одни frozen
metadata, local facts и reference time для обеих evaluation. Второй результат
`no-update` после первого commit всё равно требует настоящей проверки.
Installation сравнивает Targets floors/digest, release identity/version,
platform/architecture/environment/Network, descriptor и actual resource bytes.
Release не устанавливает ресурсы и не объявляет готовность по одному target.
Не объединять Enrollment, Release и Installation в общий root или aggregate.

### Механизмы и платформы

Переносимые правила: offline envelope, target identity, policy, floors,
immutable authorization и operation lifecycle. Native adapters нужны для
exclusive locking, открытия файлов, atomic publication и durable flush.
Суффикс Linux у простого наблюдения floors не является обоснованием зависимости.
Поддержка durable adapter доказывается actual reopen/crash/uncertainty тестами;
компиляция или переносимый parser не квалифицируют filesystem и installed ACL.
Не использовать пустой успешный flush как доказательство одинаковой durability
на разных ОС. Отсутствие выбранного механизма означает явный отказ.

Начать с одного cohesive package; разделять файлы по verification pipeline,
metadata authentication, target/policy, authorization, history и native store
lifecycle. Подпакет допустим только при самостоятельной обязанности, малом
Interface, real caller и полном registry. Не переносить десятки коротких
файлов или private test seams автоматически ради сходства с predecessor.

### Проверенные места исходника, требующие новых оракулов

Это source inspection, а не доказанный эксплуатационный сценарий или
производительная оценка:

- `internal/release/store_persist.go`: ReadFloors возвращает пустые floors при
  отсутствующем `current`; Open вызывает эту проверку после подготовки
  generations. Новый тест обязан отличать fresh root от retained generations
  с потерянным pointer и доказать отказ без нового initial trust.
- `internal/release/store_lease.go` и `store_persist.go`: filesystem lease
  исключает второго opener, но mutable `closed` и read/commit/Close не имеют
  общей in-process сериализации. Проверить overlapping evaluation, queued
  cancellation и Close во время I/O; lease возвращается только после join.
- `internal/release/evaluate.go` и `tuf_client.go`: context проверяется в начале
  metadata workflow, но перед успешным authorize нет отдельного исходного
  context guard. Требуются causal cancellation controls перед commit и перед
  handoff, с сохранением committed floors после поздней отмены.
- `internal/release/evaluate.go`: incompatible outcome выбирается по substring.
  Новый контракт ошибок должен сохранять typed cause независимо от сообщения.
- `internal/release/store_staging.go`: cleanup использует prefix и RemoveAll.
  В новом owner чужой или malformed residue не становится разрешением
  удаления; exact bounded writer forms и committed history проверяются до
  mutation. Recovery не выбирает произвольное поколение по имени или mtime.

### Реальный consumer и приёмка

Новый offline consumer в `cmd/ardents-next` передаёт реальные подтверждённые
Enrollment bytes для первого запроса, отдельный Release root и явные local
facts. Он вызывает настоящий verifier, закрывает его с сохранением ошибок
и выводит bounded outcome без raw metadata/history. Первый consumer проверяет
реальное принятие/отказ и durable reopen; он не запускает artifact и не
заменяет Installation или qualified Execution успешной заглушкой.

Independent signed fixtures пересекают публичный verification Interface.
Обязательны: genuine signature success, threshold/signature/expiry отказ,
Root rotation/reopen, rollback и same-version conflict, actual byte/local
substitution, dual-target coherence и изменённый generation при прежнем
executable, mutation isolation, bounds и original cancellation. Каждая
критическая ordering check получает causal red control. Durable tests
проверяют interrupted publication, missing pointer, foreign residue,
uncertain flush, concurrent open и join-before-lease-release на выбранных ОС.

Final acceptance включает новый command и все затронутые новые домены,
architecture/profile/static/race, обязательные quick/check и normal hooks,
source-matched scoped commit/push в dev и owner-map reconciliation.
Это Release acceptance, не installed/Service/privacy qualification.
Genuine opaque Route delivery всё ещё требует настоящих Installation,
Execution, Publication и Connection; отдельный Release pass не закрывает Route.
