# Service Publication

Go-пакет проверяет публичные Credential и Publication v3: подписи Authority и
Instance, точный Target/Network, validity и отдельные права чтения и публикации.
Отдельный `durable` владеет исключительным v3 root, canonical floor и
восстановлением подписанного public record. Настоящая команда
`ardents-next publication prepare-root` возвращает проверенную public history;
она не восстанавливает private Instance или accepting readiness. Отдельный
`runtime` теперь содержит live Publisher и настоящий command consumer;
native acceptance, открытие приватной capsule и replay ещё требуют реализации
или source-matched проверки.

`instance` генерирует host key и владеет canonical public request, принятием
внешнего подписанного response и durable terminal states. Команды
`publication instance-initialize`, `instance-request` и `instance-accept`
возвращают только public input/result. Native private custody сейчас Linux;
остальные платформы отказывают до key/filesystem effects. Это не qualification
Service host. Отдельный private binding сверяет настоящий durable owner,
удерживает его reservation до Close и повышает floor перед consumed redaction.
Приватный ключ остаётся только у исходного volatile binding; reopen не возвращает
signer. Подпись public Publication требует настоящего REGISTER receipt и сохраняет
bytes exact retry. Эти механизмы проверяются вместе с настоящей регистрацией на
обоих Carriers; public preparation команды не запускают accepting Publisher или
Descriptor Store ACK. Generic signer и Authority key не выдаются.

Исходный binding также сохраняет точный public record/current через свой
consumed durable reservation. Sync staging link, record/staging directory,
renamed generation и current pointer/root предшествуют успешному результату;
неоднозначная запись закрывает дальнейшие receipts до reopen. Close удаляет
pointer перед record с обоими directory barriers, сохраняя floor. Instance
создаёт отдельный volatile X25519 recipient для строго возрастающей настоящей
ревизии; максимум два удерживаемых владельца. Подпись canonical Descriptor
использует codec Reachability и исходный Instance key, сохраняя байты и ключ
при exact retry. Это ещё не принимающая пара или настоящий Store ACK.

`runtime` удерживает исходную qualified snapshot operation и единственный
Instance lifetime borrow. `execution route-holder` с `holder.publication` и
Domain4 Route plan потребляет `publication-open/publish/refresh/link/withdraw`.
Два prefixes заимствуют одну Entry selection и общий Hosting budget. Реальный
REGISTER и Source Descriptor exchange предшествуют current transition; Link
проверяет точные live факты. Scheduler использует creation+300s, overlap не более
60s, capacity два и неизменный exact retry. Join prefixes/registrations
предшествует освобождению private binding. Полный native положительный сценарий,
ACK/race/timing evidence и private delivery пока не установлены.

**Вопрос домена:** какой авторизованный Service Instance сейчас принимает работу?

**Входит:** поколение и материал Instance по назначению, ревизия публикации,
текущая/подготавливаемая/предыдущая пара регистраций, приватный получатель,
защита от повторов, готовность, обновление и отзыв публикации.

**Не входит:** корневая подпись Credential, транспорт Route, история конфликтов
Descriptor Store, восстановление Connection, изменение Job и очистка worker.

**Состояние и завершение:** перестать принимать новые операции, завершить свои
регистрации и получателей, стереть приватный материал. Поздний ACK не возрождает
поколение. ACK регистрации и ACK Descriptor Store — разные основания готовности.

**Соседи:** Execution предоставляет точную живую операцию; Route — регистрацию
и непрозрачную доставку; Reachability — размещение Descriptor. Custody сохраняет
корневые полномочия; runtime-ключ Instance не превращается в корневую власть.

**Где искать обязанности:** `internal/service/instance`, `service/publication`,
регистрация и Introduction в `internal/endpoint`. Переносить только обязанности
публикации: принимающие слоты и их история принадлежат Route.

**Основа:** [Service](../../../docs/technical/endpoint-service-runtime.md),
[reachability](../../../docs/technical/private-reachability.md),
[карта доменов](../../../docs/development/domain-map.md).
