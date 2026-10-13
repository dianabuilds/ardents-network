# Service Connection

Начальная ограниченная реализация связывает проверенную Publication и поля
приватной капсулы с исходной квалифицированной Execution operation. Она вычисляет
неизменяемый logical context и отдельный Attachment context, отказывает при
подмене Target/Network/profile и расширении исходных Work Safety пределов.
Publication runtime потребляет эту привязку после собственных проверок реальной
регистрации, Instance и независимо разрешённого Rendezvous. Привязка не является
успешным Service Connection: сама привязка не удостоверяет Instance TLS или
Continuity и не предоставляет ordered bytes, recovery или исход полного потока.

## Interface и Implementation

Закрытая v3 grammar и transcript проверки отделены от живой привязки:
`wire/record.go` проверяет canonical frames и ошибки физической записи;
живой поток использует его stream-only parser, который отказывает при записях
initial/recovery authentication вне соответствующей фазы.
`wire/continuity_transcript.go`, `wire/exporter_transcript.go`,
`instance_certificate.go` и `wire/instance_proof.go` проверяют конкретные
криптографические обязательства. `wire/identity_transcript.go` — единственный
производитель canonical logical/Attachment commitments для привязки и Instance.
`send_ledger.go` удерживает bounded suffix и исходные counters между
Attachment, `terminal_exchange.go` — причинный порядок Terminal/receipt/confirmation,
`recovery_episode.go` — конечное число proposals и исходный предел episode.
Эти механизмы требуют внешней сериализации и настоящих проверок владельцев;
их unit fixtures не являются Service readiness или физическим join.
Linux `make route-check` включает весь Connection вместе с соседними доменами
и race detector; это не заменяет native и installed проверки.

`publication/runtime.NextStream` устанавливает исходный Source/Responder scope
до выдачи потока. Входящая replacement капсула выбирает только тот же поток по
полной неизменяемой identity; независимая проверка текущей Publication,
монотонной revision и исходного continuity key предшествует commit.
`InitiatorSource.OpenStream` устанавливает исходный Reader replacement source;
`initiator_replacement.go` удерживает конечный ticket до фактического join,
а `initiator_replacement_commit.go` атомарно переносит все byte/Terminal ledgers.
Повторный lookup использует прежний Target, History и retained Rendezvous set.
Текущий Attachment получает свежие JOIN/TLS/capsule факты, но не новую logical
identity, operation, Work Safety границу или continuity secret. Физическое
закрытие и retained cleanup outcome отделены от Application Done.
Командные `publication-serve` и Reader `target_link` потребляют этот путь через
исходные квалифицированные Text workers. Эти consumers сами по себе не являются
полной installed/recovery/Terminal квалификацией.

`publisher_authentication.go` выполняет coalesced request/response на исходном
JOIN через purpose-specific `publication/instance.ServiceAttachment`.
Continuity проверяется до InstanceProof; начальный ответ связывает exact TLS,
proof и нулевые offsets и записывается до снятия setup deadline. Возвращаемый
opaque Attachment удерживает исходную operation. `initiator_binding.go` создаёт
свежий salted commitment исходного локального Reader ConnectionContext;
`initiator_authentication.go` проверяет точный Instance certificate, подписанный
ответ и его initial Continuity. `authenticated_attachment.go` удерживает
роль, неизменную identity и исходный TLS ресурс. `OpenStream` потребляет это
начальное состояние однократно под проверенным `execution/runtime.StreamContract`.
Начальная передача в `openStream` наблюдает Instance и исходные Route owners
до захвата state locks. `Joined.CommitOriginal` затем удерживает исходные
Source/Responder и Acquisition barriers; Connection проверяет под ними прежнюю
operation, immutable byte contract и exact initial Attachment. Publisher
использует локальный `ServiceAttachment.CheckStreamCommit`, который проверяет
живую Instance lease и ту же Publication reservation без native I/O; Reader
проверяет исходный binding и завершённую authentication. Только после этих
проверок создаются record actors и однократно потребляется continuity key.
Локальные commit checks не заменяют свежие observations до barrier и не
разрешают новый Job, поколение или физический JOIN.

Read/Write/CloseInput, принятие записей, завершение физической записи и
подготовка recovery выполняют полные observations до state locks. Затем
`Stream.commit` удерживает исходные Source/Responder barriers и Stream mutex,
проверяет exact Reader Source либо логическую Instance lease, прежнюю operation,
сроки Credential/Work Safety и immutable byte contract. Только ограниченные
локальные изменения ledgers происходят под этим барьером; физическая запись,
наблюдения и ожидание свободного места выполняются снаружи. После I/O ещё раз
сверяются те же record actors и generation, прежде чем менять counters или
Terminal state. Уведомление об изменении фиксируется под mutex, поэтому эффект
между выходом из барьера и ожиданием не теряется. Отмена освобождает ожидание
без нового Data/ACK. Instance проверяет исходную Publication reservation и при
окончательном replacement commit; локальная проверка не возобновляет authority
и не заменяет свежие native observations перед передачей.

Локальный отказ Source и Reader Attachment сохраняет исходную отмену;
Source также сохраняет истечение или уже записанную
ошибку. Обычное закрытие возвращает отказ закрытого ресурса; оно не добавляет
нарушение целостности. Проверка identity и исходного ConnectionContext остаётся
отдельной: действительная подмена сохраняет integrity failure, а потеря прежней
Execution operation возвращает её исходную причину.

Reader Source закрывает и дожидается своего исходного Prefix, сохраняя полный
результат логического Stream и Prefix отдельно от физической очистки. Прежний
заменённый Attachment сохраняет исход своей попытки у своего владельца; этот
исход не переопределяет успешное восстановление логического потока. В Execution
передаётся только исходный joined `Prefix.CleanupResult` вместе с фактическими
Attachment cleanup failures. Отмена работы не создаёт отказ физической очистки;
реальная поздняя ошибка записи, закрытия или возврата ресурса остаётся отказом.

`stream.go` и `stream_records.go` владеют ordered byte counters, ACK, half-close,
Terminal exchange, сохранённым исходом и joined I/O; законченный контрольный
хвост остаётся в исходных Work Safety/no-new-recovery пределах. Двухсторонние
командные consumers используют эти владельцы через исходные Text workers;
наличие API и consumers не квалифицирует полный установленный путь.
Publisher Attachment владеет отдельной исходной `instance.ServiceLease`:
отдельный физический Attachment может завершиться, сохранив logical Instance
borrow и его прежние cleanup ошибки. Логический поток должен удержать ту же
аренду через реальные replacement и Terminal-control операции.

`publisher_replacement.go` удерживает ограниченную попытку под тем же Stream,
исходной Instance lease и continuity secret. Он проверяет fresh JOIN/TLS,
роль, exporter и неизменный logical context, сверяет peer offsets без изменения
байтовых ledgers и пишет один ответ через purpose-specific Instance operation.
Integrity violation завершает логический поток, а не открывает новую попытку.
`wire.ErrInvalidRecord` отличает уже наблюдаемое противоречие закрытой grammar
от физической ошибки чтения: EOF, неполный frame и другие ошибки I/O сохраняют
исходную причину. Полученный неверный prefix или length отказывает немедленно,
без ожидания оставшегося тела. Эта классификация не устанавливает причину
сетевого обрыва и сама по себе не разрешает replacement.
После резервирования proposal отказ закрывает и дожидается нового JOIN даже
при неудаче выделения TLS borrow; исходная lease не закрывается этой очисткой.
Подготовка теперь резервирует ticket до приобретения fresh Responder JOIN,
сравнивает конфигурацию с actual private opening и всем исходным logical tuple
до сетевого эффекта, затем сама открывает тот же JOIN. Его рабочий caller —
исходный logical lifetime, а конечный proposal context прерывает только setup.
Неудачный JOIN также расходует предложение и физически завершается до возврата
ticket. Close ждёт окончательного опубликования подготовки перед join callbacks,
включая callback, установленный после начала отмены.
`publisher_recovery.go` удерживает исходный Source и opaque ResponderScope до запуска stream,
начинает конечный отсчёт при физической потере и дожидается обоих старых record
actors. `publisher_replacement_commit.go` передаёт новый Attachment после этого
join, проверки прежнего caller и живых владельцев, атомарно меняет три ledgers
и запускает writer для retained suffix. Отсоединение setup callbacks не снимает
проверку исходного caller до commit. Instance отдельно проверяет прежний private
recipient/cutoff после TLS transfer без native I/O под Source locks.
Source/Publication dispatch и Reader recovery используют эти механизмы через
`NextStream` и `InitiatorSource`. Проверка конкретного recovery сценария не
доказывает полную матрицу recovery и Terminal-control отказов.

Логическая проверка Publisher использует прежние полномочия, выбранный role-3
leg и scope, независимо от готовности завершившегося физического поколения.
После резервирования finite proposal Scope приобретает точное новое поколение
с тем же Source/leg и настоящим новым Hosting reservation; старое сначала join.
JOIN и окончательный transfer сверяют именно этот acquisition. Первый
аутентифицированный JOIN также должен принадлежать тому же opaque scope.
Смена физического поколения не меняет Instance, Publication, Job, logical tuple,
continuity key, counters или Work Safety. Фактический отказ cleanup остаётся
терминальным для scope; повреждённый TLS writer не используется повторно.

`publication/runtime.NextAuthentication` потребляет завершённую initial delivery,
исходное private opening и тот же Source-bound Responder JOIN перед Service TLS.
`execution/runtime.DeliverPublisherStream` передаёт уже аутентифицированный поток
единственному Text protocol reader той же исходной qualified invocation.
Это техническая byte boundary; она не удостоверяет Service вместо Connection.
Обычная композиция обязана установить настоящий Route-backed replacement source
до передачи Application bytes. Полный handoff не заменяется вызовом одного API.

`BindRecipient` требует исходную живую Publisher operation и sealed Publication
proof. `RecipientBinding.Check` проверяет ту же operation и исходный capsule
deadline перед первой передачей результата; `Contexts` возвращает только копии
transcript commitments. `recipient_binding.go` хранит эти неизменяемые факты
без сетевых эффектов, приватного ключа или отдельного cleanup ресурса.
Поведение проверяется независимыми signed-input/transcript fixtures и отказами
при подмене, расширении пределов и отсутствии исходной operation. Эти fixtures
не квалифицируют worker, принимающий Publisher или Service stream.

**Вопрос домена:** как один аутентифицированный логический поток сохраняет идентичность?

**Входит:** неизменные Target и происхождение доказательства, точная
аутентификация Instance, упорядоченные байты, поколения Attachment,
исходный предел восстановления и сохранённый терминальный результат.

**Не входит:** повтор Application-операции, новый Target или Grant, изменение
Job, очистка worker, публикация, политика пути и бюджет Hosting.

**Состояние и завершение:** прекращает работу и дожидается своих Attachment,
сохраняет логический исход. Восстановление заменяет Attachment, но не Target,
Job, Grant или исходный срок восстановления. Route завершает физический поток,
Execution — worker; эти обязанности не переходят к Connection.

**Соседи:** Reachability предоставляет проверенное доказательство; Route —
новую физическую попытку в исходных пределах; Execution — точную живую операцию,
проверяемую перед эффектами и окончательной передачей результата.

**Где искать обязанности:** `internal/service/connection`, Service binding
и восстановление в `internal/endpoint`, `internal/endpoint/service`.
Механизмы и полномочия разбираются отдельно; старые callbacks не переносятся
как междоменный контракт.

**Основа:** [Service](../../../docs/technical/endpoint-service-runtime.md),
[граница Execution](../../../docs/technical/application-execution.md),
[карта доменов](../../../docs/development/domain-map.md).
